package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/dating-bot/recommendation-service/internal/cache"
	"github.com/dating-bot/recommendation-service/internal/client"
	"github.com/dating-bot/recommendation-service/internal/domain/entity"
	"github.com/dating-bot/recommendation-service/internal/domain/repository"
	"github.com/dating-bot/recommendation-service/internal/service/ranking"
	"github.com/dating-bot/recommendation-service/internal/service/worker"
)

// RecommendationService is the application-layer orchestrator.
// It wires together:
//   - RatingRepository  — Postgres persistence for composite scores
//   - RecommendationCache — Redis queue per viewer
//   - UserProfileClient — profile metadata from user-profile-service
//   - MatchingClient    — interaction stats from matching-service
//   - PrimaryCalculator — Level-1 rating formula
//   - BehavioralCalculator — Level-2 rating formula
//   - Pool              — background goroutine pool for async recalculation
type RecommendationService struct {
	ratingRepo  repository.RatingRepository
	cache       *cache.RecommendationCache
	upClient    *client.UserProfileClient
	matchClient *client.MatchingClient
	publisher   repository.EventPublisher
	primaryCalc *ranking.PrimaryCalculator
	behavCalc   *ranking.BehavioralCalculator
	pool        *worker.Pool
	batchSize   int // candidates fetched per feed refill
}

// NewRecommendationService wires all dependencies and registers the pool handler.
func NewRecommendationService(
	ratingRepo repository.RatingRepository,
	cache *cache.RecommendationCache,
	upClient *client.UserProfileClient,
	matchClient *client.MatchingClient,
	publisher repository.EventPublisher,
	pool *worker.Pool,
	batchSize int,
) *RecommendationService {
	if batchSize <= 0 {
		batchSize = 50
	}

	svc := &RecommendationService{
		ratingRepo:  ratingRepo,
		cache:       cache,
		upClient:    upClient,
		matchClient: matchClient,
		publisher:   publisher,
		primaryCalc: ranking.NewPrimaryCalculator(),
		behavCalc:   ranking.DefaultBehavioralCalculator(),
		pool:        pool,
		batchSize:   batchSize,
	}

	// Register ourselves as the pool's job handler.
	pool.SetHandler(svc.handleJob)
	return svc
}

// ── Public API ────────────────────────────────────────────────────────────────

// GetNextProfile returns the single next recommended profile for viewerUserID.
//
// Algorithm:
//  1. Fetch viewer's own gender from Postgres (determine target gender).
//  2. Pop the next candidate user_id from the Redis queue (O(1)).
//  3. If the queue is empty, refill it:
//     a. Fetch viewer's already-interacted user_ids concurrently.
//     b. Query Postgres for top-N candidates of opposite gender.
//     c. Push the ranked list into Redis and pop the first entry.
//  4. Enrich the popped user_id with profile metadata.
//  5. Return the enriched profile.
func (s *RecommendationService) GetNextProfile(
	ctx context.Context,
	viewerUserID int64,
) (*entity.RecommendedProfile, bool, error) {

	// Step 1: determine the viewer's target gender.
	targetGender, err := s.targetGender(ctx, viewerUserID)
	if err != nil {
		return nil, false, fmt.Errorf("get target gender for viewer %d: %w", viewerUserID, err)
	}
	if targetGender == "" {
		// Viewer has no profile yet — cannot build a feed.
		return nil, false, nil
	}

	// Step 2: try to pop from the existing Redis queue.
	candidateID, ok, err := s.cache.Pop(ctx, viewerUserID)
	if err != nil {
		logrus.WithError(err).Warn("cache pop failed, falling back to DB")
	}

	if !ok {
		// Step 3: refill the queue and pop the first entry.
		candidateID, ok, err = s.refillAndPop(ctx, viewerUserID, targetGender)
		if err != nil {
			return nil, false, fmt.Errorf("refill recommendation queue: %w", err)
		}
		if !ok {
			return nil, false, nil // no candidates available
		}
	}

	// Step 4: fetch profile metadata for the candidate.
	profile, err := s.enrichCandidate(ctx, candidateID)
	if err != nil {
		return nil, false, fmt.Errorf("enrich candidate %d: %w", candidateID, err)
	}
	if profile == nil {
		// Profile was deleted between ranking and now — skip silently.
		return nil, false, nil
	}

	return profile, true, nil
}

// GetRecommendations returns a ranked batch of up to `limit` profiles for viewerUserID.
//
// Unlike GetNextProfile it does NOT pop from the cache — it recomputes a fresh
// ranked list every time so that callers get a predictable, consistent snapshot.
func (s *RecommendationService) GetRecommendations(
	ctx context.Context,
	viewerUserID int64,
	limit int,
) ([]*entity.RecommendedProfile, error) {
	if limit <= 0 {
		limit = 10
	}

	// Fetch viewer gender + already-interacted IDs in parallel.
	var viewerRating *entity.Rating
	var excludeIDs []int64

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		r, err := s.ratingRepo.GetByUserID(gCtx, viewerUserID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("get viewer rating: %w", err)
		}
		viewerRating = r
		return nil
	})

	g.Go(func() error {
		ids, err := s.interactedUserIDs(gCtx, viewerUserID)
		if err != nil {
			return fmt.Errorf("get interacted ids: %w", err)
		}
		excludeIDs = ids
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	if viewerRating == nil {
		return nil, nil // no profile, no recommendations
	}

	targetGender := oppositeGender(viewerRating.Gender)
	excludeIDs = append(excludeIDs, viewerUserID) // never recommend self

	candidates, err := s.ratingRepo.GetCandidates(ctx, targetGender, excludeIDs, limit)
	if err != nil {
		return nil, fmt.Errorf("get candidates: %w", err)
	}

	// Enrich candidates with live profile data — run in parallel.
	return s.enrichCandidatesBatch(ctx, candidates)
}

// GetRating returns the stored composite rating for userID.
// Returns sql.ErrNoRows when no rating exists yet.
func (s *RecommendationService) GetRating(
	ctx context.Context,
	userID int64,
) (*entity.Rating, error) {
	return s.ratingRepo.GetByUserID(ctx, userID)
}

// UpdateRating partially updates a user's primary or behavioral rating,
// recomputes the combined score, persists the change, logs it, and
// invalidates that user's viewer cache entries.
func (s *RecommendationService) UpdateRating(
	ctx context.Context,
	userID int64,
	primary, behavioral *float64,
) (*entity.Rating, error) {
	rating, err := s.getOrCreateRating(ctx, userID)
	if err != nil {
		return nil, err
	}

	oldCombined := rating.CombinedRating

	if primary != nil {
		rating.PrimaryRating = *primary
	}
	if behavioral != nil {
		rating.BehavioralRating = *behavioral
	}

	rating.RecalculateCombined()

	if err := s.ratingRepo.Upsert(ctx, rating); err != nil {
		return nil, fmt.Errorf("upsert rating: %w", err)
	}

	reason := "manual update via UpdateRating RPC"
	_ = s.ratingRepo.LogChange(ctx, userID, oldCombined, rating.CombinedRating, reason)

	// Invalidate all viewers' queues that might have this user cached.
	// We do this lazily (fire-and-forget) to keep the RPC latency low.
	go func() {
		if err := s.cache.InvalidateAll(context.Background()); err != nil {
			logrus.WithError(err).Warn("cache invalidate after UpdateRating failed")
		}
	}()

	return rating, nil
}

// TriggerRecalculation enqueues a full recalculation job for userID.
// The job is processed asynchronously by the worker pool.
func (s *RecommendationService) TriggerRecalculation(
	ctx context.Context,
	userID int64,
) error {
	return s.pool.EnqueueWait(ctx, worker.Job{
		Type:   worker.JobFullRecalc,
		UserID: userID,
	})
}

// HandleProfileUpdated is called by the RabbitMQ subscriber when a
// "profile.updated" event arrives. It enqueues a primary recalculation.
func (s *RecommendationService) HandleProfileUpdated(evt *entity.DomainEvent) error {
	var data entity.ProfileUpdatedData
	if err := json.Unmarshal(evt.Data, &data); err != nil {
		return fmt.Errorf("unmarshal profile.updated: %w", err)
	}

	s.pool.Enqueue(worker.Job{
		Type:   worker.JobPrimaryRecalc,
		UserID: data.UserID,
	})
	return nil
}

// HandleInteractionLiked is called when an "interaction.liked" event arrives.
// The TARGET user's behavioral rating needs recalculation.
func (s *RecommendationService) HandleInteractionLiked(evt *entity.DomainEvent) error {
	var data entity.InteractionLikedData
	if err := json.Unmarshal(evt.Data, &data); err != nil {
		return fmt.Errorf("unmarshal interaction.liked: %w", err)
	}

	// Recalculate behavioral rating for the user who received the like.
	s.pool.Enqueue(worker.Job{
		Type:   worker.JobBehavioralRecalc,
		UserID: data.ToUserID,
	})
	return nil
}

// ── Worker job handler ────────────────────────────────────────────────────────

// handleJob is the Pool's Handler — called once per dequeued Job.
// It must be safe for concurrent execution by multiple goroutines.
func (s *RecommendationService) handleJob(ctx context.Context, job worker.Job) {
	log := logrus.WithField("job_type", job.Type).WithField("user_id", job.UserID)

	var err error
	switch job.Type {
	case worker.JobPrimaryRecalc:
		err = s.recalcPrimary(ctx, job.UserID)

	case worker.JobBehavioralRecalc:
		err = s.recalcBehavioral(ctx, job.UserID)

	case worker.JobFullRecalc:
		err = s.recalcFull(ctx, job.UserID)

	case worker.JobGlobalRecalc:
		err = s.globalRecalc(ctx)

	default:
		log.Warn("unknown job type — skipping")
		return
	}

	if err != nil {
		log.WithError(err).Error("job failed")
	}
}

// ── Recalculation helpers ─────────────────────────────────────────────────────

// recalcPrimary fetches the latest profile from user-profile-service,
// recomputes the primary rating, and persists the update.
func (s *RecommendationService) recalcPrimary(ctx context.Context, userID int64) error {
	stats, err := s.upClient.GetProfileStats(ctx, userID)
	if err != nil {
		return fmt.Errorf("get profile stats: %w", err)
	}
	if stats == nil {
		return nil // profile deleted
	}

	rating, err := s.getOrCreateRating(ctx, userID)
	if err != nil {
		return err
	}

	oldCombined := rating.CombinedRating
	rating.UpdateProfile(stats.Gender, int(stats.Age), stats.City)
	rating.PrimaryRating = s.primaryCalc.Calculate(stats)
	rating.RecalculateCombined()

	if err := s.ratingRepo.Upsert(ctx, rating); err != nil {
		return fmt.Errorf("upsert rating: %w", err)
	}

	_ = s.ratingRepo.LogChange(ctx, userID, oldCombined, rating.CombinedRating, "primary recalc")
	s.publishRecalculated(ctx, rating)
	return nil
}

// recalcBehavioral fetches interaction statistics from matching-service and
// recomputes the behavioral rating.
func (s *RecommendationService) recalcBehavioral(ctx context.Context, userID int64) error {
	stats, err := s.matchClient.GetInteractionStats(ctx, userID)
	if err != nil {
		return fmt.Errorf("get interaction stats: %w", err)
	}

	rating, err := s.getOrCreateRating(ctx, userID)
	if err != nil {
		return err
	}

	oldCombined := rating.CombinedRating
	rating.BehavioralRating = s.behavCalc.Calculate(ranking.BehavioralInput{
		LikesReceived:  stats.LikesReceived,
		PassesReceived: stats.PassesReceived,
		MatchCount:     stats.MatchCount,
	})
	rating.RecalculateCombined()

	if err := s.ratingRepo.Upsert(ctx, rating); err != nil {
		return fmt.Errorf("upsert rating: %w", err)
	}

	_ = s.ratingRepo.LogChange(ctx, userID, oldCombined, rating.CombinedRating, "behavioral recalc")
	s.publishRecalculated(ctx, rating)
	return nil
}

// recalcFull recomputes both primary and behavioral ratings concurrently for
// a single user, then persists the updated composite score.
func (s *RecommendationService) recalcFull(ctx context.Context, userID int64) error {
	var profileStats *client.ProfileStats
	var interactionStats *client.InteractionStats

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		profileStats, err = s.upClient.GetProfileStats(gCtx, userID)
		return err
	})

	g.Go(func() error {
		var err error
		interactionStats, err = s.matchClient.GetInteractionStats(gCtx, userID)
		return err
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("parallel fetch for full recalc user %d: %w", userID, err)
	}

	if profileStats == nil {
		return nil // user/profile deleted
	}

	rating, err := s.getOrCreateRating(ctx, userID)
	if err != nil {
		return err
	}

	oldCombined := rating.CombinedRating
	rating.UpdateProfile(profileStats.Gender, int(profileStats.Age), profileStats.City)
	rating.PrimaryRating = s.primaryCalc.Calculate(profileStats)

	if interactionStats != nil {
		rating.BehavioralRating = s.behavCalc.Calculate(ranking.BehavioralInput{
			LikesReceived:  interactionStats.LikesReceived,
			PassesReceived: interactionStats.PassesReceived,
			MatchCount:     interactionStats.MatchCount,
		})
	}

	rating.RecalculateCombined()

	if err := s.ratingRepo.Upsert(ctx, rating); err != nil {
		return fmt.Errorf("upsert rating user %d: %w", userID, err)
	}

	_ = s.ratingRepo.LogChange(ctx, userID, oldCombined, rating.CombinedRating, "full recalc")
	s.publishRecalculated(ctx, rating)
	return nil
}

// globalRecalc fetches every rating row from Postgres and fans out a
// JobFullRecalc per user into the same pool using a bounded goroutine fan-out.
//
// This approach is intentionally self-throttling: the pool's own queue provides
// back-pressure so we don't hammer Postgres and external services simultaneously.
func (s *RecommendationService) globalRecalc(ctx context.Context) error {
	ratings, err := s.ratingRepo.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("global recalc fetch all: %w", err)
	}

	logrus.WithField("user_count", len(ratings)).Info("global recalculation started")

	// Use at most GOMAXPROCS workers for the fan-out goroutines — they're just
	// enqueueing, not doing real work.
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	var wg errgroup.Group

	for _, r := range ratings {
		uid := r.UserID
		sem <- struct{}{}

		wg.Go(func() error {
			defer func() { <-sem }()

			// EnqueueWait blocks if the pool queue is full — this naturally
			// throttles the fan-out speed to match worker throughput.
			return s.pool.EnqueueWait(ctx, worker.Job{
				Type:   worker.JobFullRecalc,
				UserID: uid,
			})
		})
	}

	if err := wg.Wait(); err != nil {
		return fmt.Errorf("global recalc enqueue: %w", err)
	}

	// Flush all caches after a global recalc so viewers get fresh feeds.
	if err := s.cache.InvalidateAll(ctx); err != nil {
		logrus.WithError(err).Warn("cache invalidate after global recalc failed")
	}

	logrus.WithField("user_count", len(ratings)).Info("global recalculation enqueued")
	return nil
}

// ── Feed helpers ──────────────────────────────────────────────────────────────

// refillAndPop builds a fresh recommendation queue for viewerUserID, pushes it
// to Redis, and pops the first entry.  Returns (0, false, nil) when no
// candidates are available.
func (s *RecommendationService) refillAndPop(
	ctx context.Context,
	viewerUserID int64,
	targetGender string,
) (int64, bool, error) {
	// Fetch already-interacted IDs so we don't re-show profiles.
	excludeIDs, err := s.interactedUserIDs(ctx, viewerUserID)
	if err != nil {
		logrus.WithError(err).Warn("could not fetch interaction history — exclusion list is empty")
		excludeIDs = nil
	}
	excludeIDs = append(excludeIDs, viewerUserID)

	candidates, err := s.ratingRepo.GetCandidates(ctx, targetGender, excludeIDs, s.batchSize)
	if err != nil {
		return 0, false, fmt.Errorf("get candidates: %w", err)
	}
	if len(candidates) == 0 {
		return 0, false, nil
	}

	// Build ordered list of user_ids and push to Redis.
	ids := make([]int64, len(candidates))
	for i, c := range candidates {
		ids[i] = c.UserID
	}

	if err := s.cache.Push(ctx, viewerUserID, ids); err != nil {
		logrus.WithError(err).Warn("cache push failed — serving from DB directly")
		// Graceful degradation: return the first candidate without caching.
		return ids[0], true, nil
	}

	// Pop the first entry from the freshly populated queue.
	candidateID, ok, err := s.cache.Pop(ctx, viewerUserID)
	if err != nil || !ok {
		// Fallback: serve directly from the slice.
		return ids[0], true, err
	}
	return candidateID, true, nil
}

// interactedUserIDs returns the set of user_ids that viewerUserID has already
// liked or passed, fetched concurrently with the match list.
func (s *RecommendationService) interactedUserIDs(
	ctx context.Context,
	viewerUserID int64,
) ([]int64, error) {
	resp, err := s.matchClient.GetInteractionHistory(ctx, viewerUserID)
	if err != nil {
		return nil, fmt.Errorf("get interaction history: %w", err)
	}

	seen := make(map[int64]struct{}, len(resp.Interactions))
	for _, i := range resp.Interactions {
		if i.FromUserId == viewerUserID {
			seen[i.ToUserId] = struct{}{}
		}
	}

	ids := make([]int64, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids, nil
}

// enrichCandidate fetches the full profile for a candidate and merges it with
// the stored rating score.  Returns nil when the profile no longer exists.
func (s *RecommendationService) enrichCandidate(
	ctx context.Context,
	candidateID int64,
) (*entity.RecommendedProfile, error) {
	var stats *client.ProfileStats
	var rating *entity.Rating

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		stats, err = s.upClient.GetProfileStats(gCtx, candidateID)
		return err
	})

	g.Go(func() error {
		var err error
		rating, err = s.ratingRepo.GetByUserID(gCtx, candidateID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}
	if stats == nil {
		return nil, nil
	}

	var score float64
	if rating != nil {
		score = rating.CombinedRating
	}

	return &entity.RecommendedProfile{
		UserID:          candidateID,
		Age:             int32(stats.Age),
		Gender:          stats.Gender,
		City:            stats.City,
		Interests:       stats.Interests,
		PhotosCount:     int32(stats.PhotosCount),
		FullnessPercent: float32(stats.FullnessPercent),
		Score:           score,
	}, nil
}

// enrichCandidatesBatch enriches a slice of ratings concurrently using a
// bounded semaphore.  Results maintain the original ranking order.
func (s *RecommendationService) enrichCandidatesBatch(
	ctx context.Context,
	candidates []*entity.Rating,
) ([]*entity.RecommendedProfile, error) {
	type indexedResult struct {
		idx     int
		profile *entity.RecommendedProfile
		err     error
	}

	sem := make(chan struct{}, runtime.NumCPU()*2)
	results := make(chan indexedResult, len(candidates))
	var wg errgroup.Group

	for i, c := range candidates {
		i, cid := i, c.UserID
		sem <- struct{}{}

		wg.Go(func() error {
			defer func() { <-sem }()

			stats, err := s.upClient.GetProfileStats(ctx, cid)
			if err != nil {
				results <- indexedResult{idx: i, err: err}
				return nil // collect errors in results channel
			}
			if stats == nil {
				results <- indexedResult{idx: i} // profile deleted — skip slot
				return nil
			}

			results <- indexedResult{
				idx: i,
				profile: &entity.RecommendedProfile{
					UserID:          cid,
					Age:             int32(stats.Age),
					Gender:          stats.Gender,
					City:            stats.City,
					Interests:       stats.Interests,
					PhotosCount:     int32(stats.PhotosCount),
					FullnessPercent: float32(stats.FullnessPercent),
					Score:           candidates[i].CombinedRating,
				},
			}
			return nil
		})
	}

	go func() {
		_ = wg.Wait()
		close(results)
	}()

	ordered := make([]*entity.RecommendedProfile, len(candidates))
	for r := range results {
		if r.err != nil {
			return nil, r.err
		}
		ordered[r.idx] = r.profile
	}

	// Remove nil slots (deleted profiles).
	out := ordered[:0]
	for _, p := range ordered {
		if p != nil {
			out = append(out, p)
		}
	}
	return out, nil
}

// ── Misc helpers ──────────────────────────────────────────────────────────────

// getOrCreateRating returns the existing rating for userID or a zero-value
// Rating if no record exists yet.
func (s *RecommendationService) getOrCreateRating(
	ctx context.Context,
	userID int64,
) (*entity.Rating, error) {
	r, err := s.ratingRepo.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.NewRating(userID, "", 0, ""), nil
		}
		return nil, fmt.Errorf("get rating user %d: %w", userID, err)
	}
	return r, nil
}

// targetGender resolves the gender that viewerUserID wants to see.
// Currently the rule is simply "opposite gender". Returns "" when the
// viewer has no rating row (i.e. has not completed their profile yet).
func (s *RecommendationService) targetGender(
	ctx context.Context,
	viewerUserID int64,
) (string, error) {
	r, err := s.ratingRepo.GetByUserID(ctx, viewerUserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return oppositeGender(r.Gender), nil
}

// oppositeGender returns the complementary gender string.
func oppositeGender(g string) string {
	switch g {
	case "male":
		return "female"
	case "female":
		return "male"
	default:
		return "female"
	}
}

// publishRecalculated fires a rating.recalculated event asynchronously.
// Errors are logged but do not affect the caller.
func (s *RecommendationService) publishRecalculated(ctx context.Context, r *entity.Rating) {
	evt, err := entity.NewRatingRecalculatedEvent(
		r.UserID, r.PrimaryRating, r.BehavioralRating, r.CombinedRating,
	)
	if err != nil {
		logrus.WithError(err).Warn("build rating.recalculated event failed")
		return
	}
	if err := s.publisher.Publish(ctx, "rating.recalculated", evt); err != nil {
		logrus.WithError(err).Warn("publish rating.recalculated failed")
	}
}

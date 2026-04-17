package ranking

import "math"

// BehavioralInput holds the raw interaction counters fetched from
// matching-service. All fields are directional — they count events
// where other users acted on the subject user.
type BehavioralInput struct {
	LikesReceived  int64
	PassesReceived int64
	MatchCount     int64
}

// BehavioralCalculator computes Level-2 (behavioural) ratings.
//
// Formula (from README §7):
//
//	behavioral = (likesWeight * normLikes)
//	           + (likePassRatio * 0.3)
//	           + (matchesWeight * normMatches)
//
// All partial scores are normalised to [0, 1] before weighting so that
// the final result always falls within [0, 1].
type BehavioralCalculator struct {
	// LikesWeight is the contribution of normalised like-count to the score.
	// Default: 0.5
	LikesWeight float64

	// MatchesWeight is the contribution of normalised match-count to the score.
	// Default: 0.2
	MatchesWeight float64

	// LikeRatioWeight is the contribution of the like/(like+pass) ratio.
	// Default: 0.3  (matches the literal coefficient in the README formula)
	LikeRatioWeight float64

	// LikesSaturation is the number of likes at which normLikes reaches ~0.95.
	// Using a sigmoid-like curve means one viral user doesn't dominate rankings.
	// Default: 100
	LikesSaturation float64

	// MatchesSaturation is the match count at which normMatches reaches ~0.95.
	// Default: 20
	MatchesSaturation float64
}

// DefaultBehavioralCalculator returns a calculator with the README-specified
// weights already filled in.
func DefaultBehavioralCalculator() *BehavioralCalculator {
	return &BehavioralCalculator{
		LikesWeight:       0.5,
		MatchesWeight:     0.2,
		LikeRatioWeight:   0.3,
		LikesSaturation:   100,
		MatchesSaturation: 20,
	}
}

// Calculate derives the behavioural rating for a user from the supplied
// interaction counters. The result is clamped to [0, 1].
//
// Normalisation strategy
// ──────────────────────
// Raw counts are normalised with a smooth saturation curve:
//
//	norm(x, k) = 1 − e^(−x/k)
//
// This maps:
//   - x = 0       → 0.00
//   - x = k       → 0.63   (one "saturation unit")
//   - x = 3k      → 0.95   (effectively saturated)
//   - x → ∞       → 1.00
//
// The exponential decay avoids hard caps while still preventing a single
// power user from dominating the entire ranking space.
func (c *BehavioralCalculator) Calculate(in BehavioralInput) float64 {
	// ── partial score 1: normalised likes received ───────────────────────────
	normLikes := saturate(float64(in.LikesReceived), c.LikesSaturation)

	// ── partial score 2: like-to-interaction ratio ───────────────────────────
	// Represents how often people choose to like vs. pass on this user.
	likeRatio := likePassRatio(in.LikesReceived, in.PassesReceived)

	// ── partial score 3: normalised match count ──────────────────────────────
	normMatches := saturate(float64(in.MatchCount), c.MatchesSaturation)

	// ── weighted combination ─────────────────────────────────────────────────
	score := (c.LikesWeight * normLikes) +
		(c.LikeRatioWeight * likeRatio) +
		(c.MatchesWeight * normMatches)

	return clampBehavioral(score)
}

// ── helpers ──────────────────────────────────────────────────────────────────

// saturate applies the smooth normalisation curve 1 − e^(−x/k).
// When k ≤ 0 the function degrades gracefully by returning 0.
func saturate(x, k float64) float64 {
	if k <= 0 || x <= 0 {
		return 0
	}
	return 1 - math.Exp(-x/k)
}

// likePassRatio returns the fraction likes/(likes+passes).
// Returns 0 when both counters are zero (new user, no data yet).
// Returns 1 when a user has received only likes and no passes.
func likePassRatio(likes, passes int64) float64 {
	total := likes + passes
	if total == 0 {
		// No interactions at all — neutral score rather than penalising new users.
		return 0.5
	}
	return float64(likes) / float64(total)
}

// clampBehavioral restricts v to [0, 1].
func clampBehavioral(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

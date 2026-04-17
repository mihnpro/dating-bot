package ranking

import (
	"math"

	"github.com/dating-bot/recommendation-service/internal/client"
)

// PrimaryCalculator computes Level-1 (primary) rating for a user based on
// their own profile quality. It does not depend on any other user's data.
//
// Formula (from README):
//
//	primary_rating = (preferences_match_score * 0.6) + (fullness_score * 0.4)
//
// Both component scores are normalised to [0, 1] before weighting.
type PrimaryCalculator struct{}

func NewPrimaryCalculator() *PrimaryCalculator {
	return &PrimaryCalculator{}
}

// Calculate derives the primary rating from a ProfileStats snapshot.
// Returns a value in [0, 1].
func (c *PrimaryCalculator) Calculate(stats *client.ProfileStats) float64 {
	if stats == nil {
		return 0
	}

	preferences := preferencesScore(stats)
	fullness := fullnessScore(stats)

	raw := preferences*0.6 + fullness*0.4
	return clamp01(raw)
}

// preferencesScore measures how complete and compelling the user's core
// profile attributes are.  Each attribute contributes a weighted sub-score:
//
//   - Gender set:       0.20  (binary — must be present)
//   - City set:         0.20  (binary)
//   - Age plausible:    0.20  (16–80 range check)
//   - Interests bonus:  0.40  (scaled by count, saturates at 5 interests)
//
// The result is in [0, 1].
func preferencesScore(s *client.ProfileStats) float64 {
	score := 0.0

	if s.Gender != "" {
		score += 0.20
	}

	if s.City != "" {
		score += 0.20
	}

	if s.Age >= 16 && s.Age <= 80 {
		score += 0.20
	}

	// Interests: each unique interest adds 0.08, capped at 5 (= 0.40).
	interestCount := math.Min(float64(len(s.Interests)), 5)
	score += interestCount * 0.08

	return clamp01(score)
}

// fullnessScore combines the server-computed FullnessPercent (which already
// accounts for filled profile fields) with a photo bonus.
//
//   - Base:        profile.FullnessPercent  ∈ [0, 1]
//   - Photo bonus: +0.10 per photo, capped at +0.30 (≥3 photos)
//
// The bonus can push the raw score above 1.0, which is then clamped.
func fullnessScore(s *client.ProfileStats) float64 {
	base := float64(s.FullnessPercent) // already in [0, 1] from user-profile-service

	photoBonus := math.Min(float64(s.PhotosCount)*0.10, 0.30)

	return clamp01(base + photoBonus)
}

// clamp01 restricts v to the closed interval [0, 1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

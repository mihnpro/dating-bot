package entity

import "time"

// RecommendedProfile is a candidate profile enriched with its ranking score.
// It is returned by GetNextProfile and GetRecommendations RPCs.
type RecommendedProfile struct {
	UserID          int64
	Age             int32
	Gender          string
	City            string
	Interests       []string
	PhotosCount     int32
	FullnessPercent float32
	Score           float64 // combined_rating used for sorting
}

// Rating holds the three-level composite score for a single user.
// It is the central domain object of the Recommendation Service.
type Rating struct {
	UserID           int64
	Gender           string
	Age              int
	City             string
	PrimaryRating    float64
	BehavioralRating float64
	CombinedRating   float64
	CalculatedAt     time.Time
}

// RatingLog records every change to a user's combined rating for auditing.
type RatingLog struct {
	ID          int64
	UserID      int64
	OldCombined float64
	NewCombined float64
	Reason      string
	ChangedAt   time.Time
}

// NewRating creates a zero-value Rating for a user whose scores have not
// been calculated yet.
func NewRating(userID int64, gender string, age int, city string) *Rating {
	return &Rating{
		UserID:           userID,
		Gender:           gender,
		Age:              age,
		City:             city,
		PrimaryRating:    0,
		BehavioralRating: 0,
		CombinedRating:   0,
		CalculatedAt:     time.Now(),
	}
}

// RecalculateCombined recomputes the combined rating from the current
// primary and behavioral scores using the formula from the README:
//
//	combined = 0.3 * primary + 0.7 * behavioral
//
// It clamps the result to [0, 1] and updates CalculatedAt.
func (r *Rating) RecalculateCombined() {
	combined := 0.3*r.PrimaryRating + 0.7*r.BehavioralRating
	if combined < 0 {
		combined = 0
	}
	if combined > 1 {
		combined = 1
	}
	r.CombinedRating = combined
	r.CalculatedAt = time.Now()
}

// SetPrimary updates the primary rating and immediately recomputes the
// combined score.
func (r *Rating) SetPrimary(v float64) {
	r.PrimaryRating = clamp01(v)
	r.RecalculateCombined()
}

// SetBehavioral updates the behavioral rating and immediately recomputes
// the combined score.
func (r *Rating) SetBehavioral(v float64) {
	r.BehavioralRating = clamp01(v)
	r.RecalculateCombined()
}

// UpdateProfile refreshes the denormalised profile fields stored alongside
// the rating so that feed queries can filter by gender/age/city without
// joining the user-profile-service database.
func (r *Rating) UpdateProfile(gender string, age int, city string) {
	r.Gender = gender
	r.Age = age
	r.City = city
}

// MakeLog creates a RatingLog entry capturing the transition from the
// previous combined score to the current one.
func (r *Rating) MakeLog(oldCombined float64, reason string) *RatingLog {
	return &RatingLog{
		UserID:      r.UserID,
		OldCombined: oldCombined,
		NewCombined: r.CombinedRating,
		Reason:      reason,
		ChangedAt:   time.Now(),
	}
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

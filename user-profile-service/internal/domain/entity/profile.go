package entity

import "time"

type Profile struct {
	ID              int64
	UserID          int64
	Age             int
	Gender          Gender
	City            string
	Interests       []string
	PhotosCount     int
	FullnessPercent float64
	UpdatedAt       time.Time
}

type Gender string

const (
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
	GenderOther  Gender = "other"
)

func NewProfile(userID int64, age int, gender Gender, city string, interests []string) *Profile {
	now := time.Now()
	return &Profile{
		UserID:          userID,
		Age:             age,
		Gender:          gender,
		City:            city,
		Interests:       interests,
		PhotosCount:     0,
		FullnessPercent: 0,
		UpdatedAt:       now,
	}
}

func (p *Profile) CalculateFullness() {
	fields := 0
	filled := 0

	// age
	fields++
	if p.Age > 0 {
		filled++
	}
	// gender
	fields++
	if p.Gender != "" {
		filled++
	}
	// city
	fields++
	if p.City != "" {
		filled++
	}
	// interests
	fields++
	if len(p.Interests) > 0 {
		filled++
	}

	if fields > 0 {
		p.FullnessPercent = float64(filled) / float64(fields)
	}
	p.UpdatedAt = time.Now()
}

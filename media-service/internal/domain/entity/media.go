package entity

import "time"

type Media struct {
	ID               int64
	UserID           int64
	S3Key            string
	OriginalFilename string
	MimeType         string
	FileSize         int64
	IsMain           bool
	UploadedAt       time.Time
}

func NewMedia(userID int64, s3Key, originalFilename, mimeType string, fileSize int64, isMain bool) *Media {
	return &Media{
		UserID:           userID,
		S3Key:            s3Key,
		OriginalFilename: originalFilename,
		MimeType:         mimeType,
		FileSize:         fileSize,
		IsMain:           isMain,
		UploadedAt:       time.Now(),
	}
}

var AllowedMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

const MaxPhotosPerUser = 6

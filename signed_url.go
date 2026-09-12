package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presingClient := s3.NewPresignClient(s3Client)

	presignedReq, err := presingClient.PresignGetObject(
		context.Background(),
		&s3.GetObjectInput{
			Bucket: &bucket,
			Key:    &key,
		},
		s3.WithPresignExpires(expireTime),
	)
	if err != nil {
		return "", err
	}

	return presignedReq.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	if video.VideoURL == nil {
		return database.Video{}, fmt.Errorf("video url is not set")
	}

	bucket, key, found := strings.Cut(*video.VideoURL, ",")
	if !found {
		return database.Video{}, fmt.Errorf("bucket and key not found")
	}

	presignedURL, err := generatePresignedURL(cfg.s3Client, bucket, key, time.Hour)
	if err != nil {
		return database.Video{}, err
	}

	video.VideoURL = &presignedURL
	return video, nil
}

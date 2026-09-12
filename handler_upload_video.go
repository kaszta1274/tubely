package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 10 << 20
	err := r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse multipart form", err)
		return
	}

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't get video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Invalid user ID", err)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse video", err)
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse media type", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid media type", errors.New("video must be an mp4"))
		return
	}

	tempVideoFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create temp video file", err)
		return
	}
	defer os.Remove(tempVideoFile.Name())
	defer tempVideoFile.Close()

	_, err = io.Copy(tempVideoFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't copy contents over from the wire to the temp file", err)
		return
	}

	tempVideoFile.Sync()

	_, err = tempVideoFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't reset the tempFile's file pointer to the beginning", err)
		return
	}

	aspectRatio, err := getVideoAspectRatio(tempVideoFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video aspect ratio of the temp file", err)
		return
	}

	ratioPrefix := getVideoAspectRatioPrefix(aspectRatio)

	videoKey := make([]byte, 32)
	rand.Read(videoKey)
	videoKeyFilePath := ratioPrefix + "/" + base64.RawURLEncoding.EncodeToString(videoKey) + ".mp4"

	processedVideoFilePath, err := processVideoForFastStart(tempVideoFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't proccess video for fast start", err)
		return
	}

	processedVideoFile, err := os.Open(processedVideoFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't open proccessed video", err)
		return
	}
	defer os.Remove(processedVideoFile.Name())
	defer processedVideoFile.Close()

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &videoKeyFilePath,
		Body:        processedVideoFile,
		ContentType: &contentType,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't upload video to s3", err)
		return
	}

	videoURL := fmt.Sprintf("%s,%s", cfg.s3Bucket, videoKeyFilePath)
	video.VideoURL = &videoURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	signedVideo, err := cfg.dbVideoToSignedVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create signed video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, signedVideo)
}

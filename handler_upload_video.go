package main

import (
	"crypto/rand"
	"encoding/base64"
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
	const maxUpload = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxUpload)) // limit the number of bytes that can be sent over

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to parse the video id from the path value", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't find the JWT token", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "JWT token is invalid", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Video ID is not in the database", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "The video does not belong to the associated user ID", err)
		return
	}

	const maxMemory = 1 << 20
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to parse multipart", err)
		return
	}

	wireFile, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to parse form file", err)
		return
	}
	defer wireFile.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to parse media type", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid file type", err)
		return
	}

	// save the uploaded video file(or chunk) to a temporary file, before routing it to s3
	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create the temporary file on filesystem", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, wireFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to copy the contents from uploaded file to temporary file", err)
		return
	}
	tempFile.Seek(0, io.SeekStart)

	processedFileName, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to process the video for fast start", err)
		return
	}

	processedFile, err := os.Open(processedFileName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to open the processed file", err)
		return
	}
	defer os.Remove(processedFileName)
	defer processedFile.Close()

	aspectRatio, err := getVideoAspectRatio(processedFileName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get the video aspect ratio", err)
		return
	}

	fileKeyBytes := make([]byte, 32)
	_, err = rand.Read(fileKeyBytes)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate the file key", err)
		return
	}
	fileKey := base64.RawURLEncoding.EncodeToString(fileKeyBytes) + ".mp4"

	switch aspectRatio {
	case "16:9":
		fileKey = fmt.Sprintf("%v/%v", "landscape", fileKey)
	case "9:16":
		fileKey = fmt.Sprintf("%v/%v", "portrait", fileKey)
	case "other":
		fileKey = fmt.Sprintf("%v/%v", "other", fileKey)
	}

	bucketName := "tubely-87237"
	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &bucketName,
		Key:         &fileKey,
		Body:        processedFile,
		ContentType: &mediaType,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to put the video in the bucket", err)
		return
	}

	s3URL := fmt.Sprintf("https://%v.s3.%v.amazonaws.com/%v", bucketName, cfg.s3Region, fileKey)
	video.VideoURL = &s3URL
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update video", err)
		return
	}
}

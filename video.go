package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
)

type VideoAspectRatioData struct {
	Streams []struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"streams"`
}

func getVideoAspectRatio(filePath string) (string, error) {
	command := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)

	result := bytes.Buffer{}
	command.Stdout = &result

	err := command.Run()
	if err != nil {
		return "", fmt.Errorf("ffprobe failed: %w", err)
	}

	var aspectRatioData VideoAspectRatioData
	if err := json.Unmarshal(result.Bytes(), &aspectRatioData); err != nil {
		return "", err
	}

	if len(aspectRatioData.Streams) == 0 {
		return "", fmt.Errorf("no streams found")
	}
	if aspectRatioData.Streams[0].Height == 0 {
		return "", fmt.Errorf("height must be greater than 0")
	}

	aspectRatio := float64(aspectRatioData.Streams[0].Width) / float64(aspectRatioData.Streams[0].Height)
	const tolerance = 0.01

	if math.Abs(aspectRatio-16.0/9.0) < tolerance {
		return "16:9", nil
	} else if math.Abs(aspectRatio-9.0/16.0) < tolerance {
		return "9:16", nil
	} else {
		return "other", nil
	}
}

func getVideoAspectRatioPrefix(aspectRetio string) string {
	switch aspectRetio {
	case "16:9":
		return "landscape"
	case "9:16":
		return "portrait"
	default:
		return "other"
	}
}

func processVideoForFastStart(filePath string) (string, error) {
	outputFilePath := filePath + ".processing"
	command := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputFilePath)

	if err := command.Run(); err != nil {
		return "", err
	}

	return outputFilePath, nil
}

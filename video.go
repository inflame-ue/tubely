package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
)

func getVideoAspectRatio(filePath string) (string, error) {
	commandName := "ffprobe"
	commandArgs := []string{
		"-v",
		"error",
		"-print_format",
		"json",
		"-show_streams",
		filePath,
	}
	cmd := exec.Command(commandName, commandArgs...)

	// this will hold the command output as bytes
	var commandOutput bytes.Buffer
	cmd.Stdout = &commandOutput

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to execute the command: %v", err)
	}

	type ffprobeOutput struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}
	var data ffprobeOutput
	err = json.Unmarshal(commandOutput.Bytes(), &data)
	if err != nil {
		return "", fmt.Errorf("failed to unmarhsal ffprobe's output into the struct: %v", err)
	}

	// this finds the aspect ration
	// gcd -> divide by gcd -> the aspect ration is w:h in reduced numbers
	width, height := data.Streams[0].Width, data.Streams[0].Height
	aspect_ratio := float64(width) / float64(height)
	landscape, portrait := float64(16)/9, float64(9)/16
	epsilon := 0.01

	if math.Abs(float64(landscape)-aspect_ratio) < epsilon {
		return "16:9", nil
	} else if math.Abs(float64(portrait)-aspect_ratio) < epsilon {
		return "9:16", nil
	} else {
		return "other", nil
	}
}

func processVideoForFastStart(filePath string) (string, error) {
	outputFilePath := filePath + ".processing"
	commandName := "ffmpeg"
	commandArgs := []string{
		"-i",
		filePath,
		"-c",
		"copy",
		"-movflags",
		"faststart",
		"-f",
		"mp4",
		outputFilePath,
	}

	cmd := exec.Command(commandName, commandArgs...)
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to move the flags: %v", err)
	}

	return outputFilePath, nil
}

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

func findGCD(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func getVideoAspectRatio(filePath string) (string, error) {
	commandName := "ffprobe"
	commandArgs := []string{
		"-v",
		"error",
		"print_format",
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
	gcd := findGCD(width, height)
	width /= gcd
	height /= gcd

	if width == 16 && height == 9 {
		return "16:9", nil
	} else if width == 9 && height == 16 {
		return "9:16", nil
	} else {
		return "other", nil
	}
}

package frames

import (
	"fmt"
	"os"
	"time"
)

type FrameType struct {
	GetFrame  func(int) string
	GetLength func() int
	GetDelay  func() time.Duration
}

// Create a function that returns the next frame, based on length
func DefaultGetFrame(frames []string) func(int) string {
	return func(i int) string {
		return frames[i%(len(frames)-1)]
	}
}

// Create a function that returns frame length
func DefaultGetLength(frames []string) func() int {
	return func() int {
		return len(frames)
	}
}

// Given frames, create a FrameType with those frames
func DefaultFrameType(frames []string, delay int) FrameType {
	return FrameType{
		GetFrame:  DefaultGetFrame(frames),
		GetLength: DefaultGetLength(frames),
		GetDelay: func() time.Duration {
			return time.Duration(delay) * time.Millisecond
		},
	}
}

func LoadFrameSet(frameSet string) FrameType {
	frameDir, err := os.ReadDir("ansi_frames/" + frameSet)
	if err != nil {
		panic(err)
	}

	var delay int = 0
	var frames []string

	for _, frameFile := range frameDir {
		if !frameFile.IsDir() {
			content, err := os.ReadFile("ansi_frames/" + frameSet + "/" + frameFile.Name())
			if err != nil {
				panic(err)
			}

			if frameFile.Name() == "delay.env" {
				_, err := fmt.Sscanf(string(content), "%d", &delay)
				if err != nil {
					panic(err)
				}
			} else {
				frames = append(frames, string(content))
			}
		}
	}

	return DefaultFrameType(frames, delay)
}

func LoadFrames() map[string]FrameType {
	var frameDirs []string
	framesDir, err := os.ReadDir("ansi_frames")

	if err != nil {
		panic(err)
	}

	for _, frameDir := range framesDir {
		if frameDir.IsDir() {
			frameDirs = append(frameDirs, frameDir.Name())
		}
	}

	frameMap := make(map[string]FrameType)
	for _, frameDir := range frameDirs {
		frameMap[frameDir] = LoadFrameSet(frameDir)
	}

	return frameMap
}

func UpdateFrames() {
	FrameMap = LoadFrames()
}

var FrameMap = LoadFrames()

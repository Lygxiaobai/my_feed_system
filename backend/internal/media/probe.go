package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const ffprobeBinary = "ffprobe"

type probeStream struct {
	CodecType string `json:"codec_type"`
	CodecName string `json:"codec_name"`
	PixFmt    string `json:"pix_fmt"`
}

type probeResult struct {
	Streams []probeStream `json:"streams"`
}

func probeMedia(ctx context.Context, input string) (probeResult, error) {
	command := exec.CommandContext(ctx, ffprobeBinary,
		"-v", "error",
		"-show_entries", "stream=codec_type,codec_name,pix_fmt",
		"-of", "json",
		input,
	)
	output, err := command.Output()
	if err != nil {
		return probeResult{}, fmt.Errorf("probe media: %w", err)
	}
	var result probeResult
	if err := json.Unmarshal(output, &result); err != nil {
		return probeResult{}, fmt.Errorf("parse media probe: %w", err)
	}
	return result, nil
}

// canRemux 为真时，源已经是浏览器能播的 H.264/AAC，只需重封装加 faststart，不必再压一遍。
func canRemux(result probeResult) bool {
	var videos, audios int
	for _, stream := range result.Streams {
		switch stream.CodecType {
		case "video":
			videos++
			if stream.CodecName != "h264" {
				return false
			}
			pix := strings.ToLower(stream.PixFmt)
			if pix != "yuv420p" && pix != "yuvj420p" {
				return false
			}
		case "audio":
			audios++
			if stream.CodecName != "aac" {
				return false
			}
		}
	}
	return videos == 1
}

package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const ffmpegBinary = "ffmpeg"

type Processor struct {
	uploadDir string
}

func NewProcessor(uploadDir string) *Processor {
	return &Processor{uploadDir: uploadDir}
}

// Transcode 将任意 ffmpeg 可读的视频统一输出为浏览器兼容的 MP4 和 JPEG poster。
func (p *Processor) Transcode(ctx context.Context, task *Task) (string, string, error) {
	if task == nil || task.ID == 0 || strings.TrimSpace(task.SourcePath) == "" {
		return "", "", errors.New("invalid media task")
	}
	if err := p.validateSourcePath(task.SourcePath); err != nil {
		return "", "", err
	}

	videoDir := filepath.Join(p.uploadDir, "videos")
	coverDir := filepath.Join(p.uploadDir, "covers")
	if err := os.MkdirAll(videoDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create video output directory: %w", err)
	}
	if err := os.MkdirAll(coverDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create poster output directory: %w", err)
	}

	videoName := fmt.Sprintf("%d.mp4", task.ID)
	posterName := fmt.Sprintf("%d.jpg", task.ID)
	videoPath := filepath.Join(videoDir, videoName)
	posterPath := filepath.Join(coverDir, posterName)
	// 临时文件保留真实扩展名，让 ffmpeg 能按输出扩展名选择容器和图片格式。
	videoTemp := videoPath + ".processing.mp4"
	posterTemp := posterPath + ".processing.jpg"
	defer func() {
		_ = os.Remove(videoTemp)
		_ = os.Remove(posterTemp)
	}()

	if err := normalizeVideo(ctx, task.SourcePath, videoTemp); err != nil {
		return "", "", fmt.Errorf("transcode video: %w", err)
	}
	if err := ensureNonEmptyFile(videoTemp); err != nil {
		return "", "", err
	}

	if err := runFFmpeg(ctx, videoTemp, posterTemp,
		"-frames:v", "1",
		"-vf", "scale=720:-2",
		"-q:v", "3",
	); err != nil {
		return "", "", fmt.Errorf("generate video poster: %w", err)
	}
	if err := ensureNonEmptyFile(posterTemp); err != nil {
		return "", "", err
	}

	if err := os.Rename(videoTemp, videoPath); err != nil {
		return "", "", fmt.Errorf("install transcoded video: %w", err)
	}
	if err := os.Rename(posterTemp, posterPath); err != nil {
		_ = os.Remove(videoPath)
		return "", "", fmt.Errorf("install video poster: %w", err)
	}

	return "/static/videos/" + videoName, "/static/covers/" + posterName, nil
}

func normalizeVideo(ctx context.Context, input string, output string) error {
	probe, err := probeMedia(ctx, input)
	if err == nil && canRemux(probe) {
		// 已经是 H.264/AAC/yuv420p 时只重封装。整段重编码在这台 4G 机器上要数分钟。
		remuxErr := runFFmpeg(ctx, input, output,
			"-map", "0:v:0",
			"-map", "0:a?",
			"-c", "copy",
			"-movflags", "+faststart",
		)
		if remuxErr == nil {
			return nil
		}
	}
	return runFFmpeg(ctx, input, output,
		"-map", "0:v:0",
		"-map", "0:a?",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "23",
		"-pix_fmt", "yuv420p",
		"-threads", "2",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
	)
}

func runFFmpeg(ctx context.Context, input string, output string, options ...string) error {
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y", "-i", input}
	args = append(args, options...)
	args = append(args, output)
	command := exec.CommandContext(ctx, ffmpegBinary, args...)
	outputBytes, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(outputBytes))
		if len(message) > 1200 {
			message = message[len(message)-1200:]
		}
		if message == "" {
			message = err.Error()
		}
		return errors.New(message)
	}
	return nil
}

func ensureNonEmptyFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("check media output: %w", err)
	}
	if info.Size() == 0 {
		return errors.New("media output is empty")
	}
	return nil
}

func (p *Processor) validateSourcePath(sourcePath string) error {
	root, err := filepath.Abs(p.uploadDir)
	if err != nil {
		return fmt.Errorf("resolve upload directory: %w", err)
	}
	source, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("resolve source path: %w", err)
	}
	relative, err := filepath.Rel(root, source)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || !strings.HasPrefix(relative, "sources"+string(filepath.Separator)) {
		return errors.New("media source path is outside the private source directory")
	}
	return nil
}

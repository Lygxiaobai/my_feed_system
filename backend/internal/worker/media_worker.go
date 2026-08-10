package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"gorm.io/gorm"

	"my_feed_system/internal/media"
	"my_feed_system/internal/mq"
)

type MediaWorker struct {
	db        *gorm.DB
	repo      *media.Repo
	processor *media.Processor
}

func NewMediaWorker(db *gorm.DB, uploadDir string) *MediaWorker {
	return &MediaWorker{
		db:        db,
		repo:      media.NewRepo(db),
		processor: media.NewProcessor(uploadDir),
	}
}

func (w *MediaWorker) Handle(ctx context.Context, event mq.Envelope) error {
	if event.EventType != mq.EventTypeMediaTranscodeRequested {
		return fmt.Errorf("media worker unsupported event: %s", event.EventType)
	}

	var payload mq.MediaTranscodePayload
	if err := event.DecodePayload(&payload); err != nil {
		return fmt.Errorf("decode media.transcode.requested payload: %w", err)
	}
	if payload.TaskID == 0 || payload.AccountID == 0 {
		return errors.New("invalid media.transcode.requested payload")
	}

	task, err := w.repo.FindByID(payload.TaskID)
	if err != nil {
		return err
	}
	if task.AccountID != payload.AccountID {
		return errors.New("media task account mismatch")
	}
	if task.Status != media.StatusProcessing {
		return nil
	}

	playURL, posterURL, processErr := w.processor.Transcode(ctx, task)
	if processErr != nil {
		if err := w.markFailed(event, task.ID, processErr.Error()); err != nil {
			return err
		}
		// 任务已经进入 failed，源文件不再用于重试，避免坏文件长期占用共享卷。
		removeSource(task.SourcePath)
		slog.ErrorContext(ctx, "transcode failed", slog.Uint64("task_id", task.ID), slog.String("error", processErr.Error()))
		return nil
	}

	if err := w.db.Transaction(func(tx *gorm.DB) error {
		if err := mq.MarkProcessed(tx, "media-worker", event); err != nil {
			if errors.Is(err, mq.ErrAlreadyProcessed) {
				return nil
			}
			return err
		}
		return w.repo.MarkReady(tx, task.ID, playURL, posterURL)
	}); err != nil {
		return err
	}

	removeSource(task.SourcePath)
	return nil
}

func (w *MediaWorker) markFailed(event mq.Envelope, taskID uint64, message string) error {
	return w.db.Transaction(func(tx *gorm.DB) error {
		if err := mq.MarkProcessed(tx, "media-worker", event); err != nil {
			if errors.Is(err, mq.ErrAlreadyProcessed) {
				return nil
			}
			return err
		}
		return w.repo.MarkFailed(tx, taskID, message)
	})
}

func removeSource(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Warn("remove media source failed", slog.String("path", path), slog.String("error", err.Error()))
	}
}

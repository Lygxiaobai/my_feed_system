package media

import (
	"errors"

	"gorm.io/gorm"
)

var ErrTaskNotFound = errors.New("media task not found")

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Create(tx *gorm.DB, task *Task) error {
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.Create(task).Error
}

func (r *Repo) FindByID(id uint64) (*Task, error) {
	var task Task
	if err := r.db.First(&task, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	return &task, nil
}

func (r *Repo) FindByIDForAccount(id uint64, accountID uint64) (*Task, error) {
	var task Task
	if err := r.db.Where("id = ? AND account_id = ?", id, accountID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	return &task, nil
}

func (r *Repo) MarkReady(tx *gorm.DB, id uint64, playURL string, posterURL string) error {
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.Model(&Task{}).Where("id = ? AND status = ?", id, StatusProcessing).Updates(map[string]any{
		"status":        StatusReady,
		"play_url":      playURL,
		"poster_url":    posterURL,
		"error_message": "",
	}).Error
}

func (r *Repo) MarkFailed(tx *gorm.DB, id uint64, message string) error {
	db := r.db
	if tx != nil {
		db = tx
	}
	if len(message) > 2000 {
		message = message[:2000]
	}
	return db.Model(&Task{}).Where("id = ? AND status = ?", id, StatusProcessing).Updates(map[string]any{
		"status":        StatusFailed,
		"error_message": message,
	}).Error
}

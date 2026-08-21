package recommend

import (
	"time"

	"my_feed_system/internal/video"
)

// VideoEmbedding 是一条已过审视频的标题/描述向量。
// 与 videos 一对一，不塞进 videos 行，避免详情缓存被大 blob 拖胖。
type VideoEmbedding struct {
	VideoID   uint64    `gorm:"primaryKey" json:"video_id"`
	Model     string    `gorm:"size:128;not null;index" json:"model"`
	Dim       int       `gorm:"not null" json:"dim"`
	Vector    []byte    `gorm:"type:blob;not null" json:"-"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (VideoEmbedding) TableName() string {
	return "video_embeddings"
}

// UserEmbedding 是一个账号的兴趣向量主数据。
// 推荐和后续推送都读这里，不能只靠请求时现场聚合 likes/tips。
type UserEmbedding struct {
	AccountID uint64    `gorm:"primaryKey" json:"account_id"`
	Model     string    `gorm:"size:128;not null;index" json:"model"`
	Dim       int       `gorm:"not null" json:"dim"`
	Vector    []byte    `gorm:"type:blob;not null" json:"-"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserEmbedding) TableName() string {
	return "user_embeddings"
}

// ListRequest 用已出 id 排除集分页，避免热度快照游标把混排打散。
type ListRequest struct {
	Limit      int64    `json:"limit"`
	ExcludeIDs []uint64 `json:"exclude_ids"`
}

// ListResult 返回本页卡片与更新后的排除集，供下一页原样带回。
type ListResult struct {
	Videos     []video.Video
	Scores     map[uint64]int64
	ExcludeIDs []uint64
	HasMore    bool
}

type interestSignal struct {
	VideoID uint64
	Weight  float64
	At      time.Time
}

type candidate struct {
	Video         video.Video
	FollowerCount int64
	Vector        []float32
	Cosine        float64
	Rel           float64
	Hot           int64
}

type queueKind int

const (
	queueInterest queueKind = iota
	queueSmall
	queueHot
)

const (
	maxExcludeIDs    = 200
	maxCandidates    = 2000
	maxInterestItems = 50
	tipWeight        = 3.0
	likeWeight       = 1.0
	interestHalfLife = 30 * 24 * time.Hour
	authorWindow     = 4
	nearDupCosine    = 0.9
	interestCacheTTL = time.Hour
	defaultPageLimit = int64(10)
	maxPageLimit     = int64(20)
)

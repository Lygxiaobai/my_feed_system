package mq

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type DeadLetterMessage struct {
	ID             uint64 `gorm:"primaryKey"`
	EventID        string `gorm:"size:64;not null;index:idx_dead_letter_messages_event_id"`
	EventType      string `gorm:"size:64;not null;index:idx_dead_letter_messages_event_type"`
	SourceQueue    string `gorm:"size:128;not null;index:idx_dead_letter_messages_source_queue"`
	DLQ            string `gorm:"size:128;not null;index:idx_dead_letter_messages_dlq;uniqueIndex:uk_dead_letter_messages_dlq_body,priority:1"`
	BodySHA256     string `gorm:"size:64;not null;uniqueIndex:uk_dead_letter_messages_dlq_body,priority:2"`
	Exchange       string `gorm:"size:128;not null"`
	RoutingKey     string `gorm:"size:128;not null"`
	MessageID      string `gorm:"size:128;not null"`
	DeathReason    string `gorm:"size:64;not null"`
	DeathCount     int64  `gorm:"not null;default:0"`
	FirstDeathAt   *time.Time
	LastDeathAt    *time.Time
	Headers        string    `gorm:"type:longtext;not null"`
	Body           string    `gorm:"type:longtext;not null"`
	HandledStatus  string    `gorm:"size:20;not null;index:idx_dead_letter_messages_status_created"`
	HandledComment string    `gorm:"type:text"`
	CreatedAt      time.Time `gorm:"autoCreateTime;index:idx_dead_letter_messages_status_created"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}

func (DeadLetterMessage) TableName() string {
	return "dead_letter_messages"
}

func NewDeadLetterMessage(dlq string, d amqp.Delivery, event Envelope) DeadLetterMessage {
	death := firstDeath(d.Headers)
	headers, _ := json.Marshal(d.Headers)

	row := DeadLetterMessage{
		EventID:       event.EventID,
		EventType:     event.EventType,
		SourceQueue:   death.Queue,
		DLQ:           dlq,
		BodySHA256:    bodySHA256(d.Body),
		Exchange:      d.Exchange,
		RoutingKey:    d.RoutingKey,
		MessageID:     d.MessageId,
		DeathReason:   death.Reason,
		DeathCount:    death.Count,
		FirstDeathAt:  death.FirstTime,
		LastDeathAt:   death.LastTime,
		Headers:       string(headers),
		Body:          string(d.Body),
		HandledStatus: "stored",
	}

	if row.EventID == "" {
		row.EventID = d.MessageId
	}
	if row.SourceQueue == "" {
		row.SourceQueue = "unknown"
	}
	if row.DeathReason == "" {
		row.DeathReason = "unknown"
	}
	if row.Headers == "" {
		row.Headers = "{}"
	}
	return row
}

func bodySHA256(body []byte) string {
	sum := sha256.Sum256(body)
	return fmt.Sprintf("%x", sum[:])
}

type deathInfo struct {
	Queue     string
	Reason    string
	Count     int64
	FirstTime *time.Time
	LastTime  *time.Time
}

func firstDeath(headers amqp.Table) deathInfo {
	raw, ok := headers["x-death"]
	if !ok {
		return deathInfo{}
	}

	deaths, ok := raw.([]interface{})
	if !ok || len(deaths) == 0 {
		return deathInfo{}
	}

	table, ok := deaths[0].(amqp.Table)
	if !ok {
		return deathInfo{}
	}

	info := deathInfo{
		Queue:  tableString(table, "queue"),
		Reason: tableString(table, "reason"),
		Count:  tableInt64(table, "count"),
	}
	info.FirstTime = tableTime(table, "time")
	info.LastTime = tableTime(table, "last-time")
	if info.LastTime == nil {
		info.LastTime = info.FirstTime
	}
	return info
}

func tableString(table amqp.Table, key string) string {
	value, ok := table[key]
	if !ok {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func tableInt64(table amqp.Table, key string) int64 {
	value, ok := table[key]
	if !ok {
		return 0
	}
	switch v := value.(type) {
	case int64:
		return v
	case int32:
		return int64(v)
	case int:
		return int64(v)
	case int16:
		return int64(v)
	case int8:
		return int64(v)
	case uint64:
		return int64(v)
	case uint32:
		return int64(v)
	case uint:
		return int64(v)
	case uint16:
		return int64(v)
	case uint8:
		return int64(v)
	default:
		return 0
	}
}

func tableTime(table amqp.Table, key string) *time.Time {
	value, ok := table[key]
	if !ok {
		return nil
	}
	t, ok := value.(time.Time)
	if !ok {
		return nil
	}
	return &t
}

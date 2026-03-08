package dao

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"cgoforum/internal/domain"
)

type EventLogDAO interface {
	// MarkProcessed records event consumption; returns true only on first successful insert.
	MarkProcessed(ctx context.Context, consumer, eventID string, occurredAt time.Time) (bool, error)
}

type eventLogDAO struct {
	db *gorm.DB
}

func NewEventLogDAO(db *gorm.DB) EventLogDAO {
	return &eventLogDAO{db: db}
}

func (d *eventLogDAO) MarkProcessed(ctx context.Context, consumer, eventID string, occurredAt time.Time) (bool, error) {
	row := domain.EventLog{
		EventID:    eventID,
		Consumer:   consumer,
		OccurredAt: occurredAt,
	}
	res := d.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&row)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

package call

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Tenant struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128;not null;uniqueIndex" json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Session struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	TenantID   uint       `gorm:"index;not null" json:"tenantId"`
	CallID     string     `gorm:"size:128;not null;uniqueIndex" json:"callId"`
	Direction  string     `gorm:"size:16;not null" json:"direction"`
	State      string     `gorm:"size:32;not null" json:"state"`
	FromNumber string     `gorm:"size:64" json:"fromNumber"`
	ToNumber   string     `gorm:"size:64" json:"toNumber"`
	StartedAt  time.Time  `json:"startedAt"`
	EndedAt    *time.Time `json:"endedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB, _ any) *Service {
	return &Service{db: db}
}

func (s *Service) CreateSession(ctx context.Context, session *Session) error {
	return s.db.WithContext(ctx).Create(session).Error
}

func (s *Service) ListSessions(ctx context.Context, limit int) ([]Session, error) {
	var sessions []Session
	err := s.db.WithContext(ctx).Order("id desc").Limit(limit).Find(&sessions).Error
	return sessions, err
}

func (s *Service) GetSessionByID(ctx context.Context, id uint) (*Session, error) {
	var sess Session
	if err := s.db.WithContext(ctx).First(&sess, id).Error; err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Service) UpdateSessionState(ctx context.Context, id uint, state string) error {
	return s.db.WithContext(ctx).Model(&Session{}).Where("id = ?", id).Update("state", state).Error
}

type ASRRecord struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SessionID *uint     `gorm:"index" json:"sessionId,omitempty"`
	PCID      string    `gorm:"size:128;index" json:"pcid"`
	Text      string    `gorm:"type:text" json:"text"`
	Final     bool      `json:"final"`
	Offset    int64     `json:"offset"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Service) CreateASRRecord(ctx context.Context, r *ASRRecord) error {
	return s.db.WithContext(ctx).Create(r).Error
}

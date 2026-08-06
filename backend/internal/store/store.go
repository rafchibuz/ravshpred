package store

import (
	"context"
	"errors"
	"time"

	"github.com/ravshann/predlozhka/backend/internal/domain"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("conflict")
	ErrDailyLimit      = errors.New("daily submission limit reached")
	ErrDuplicate       = errors.New("duplicate YouTube video")
	ErrCategoryInUse   = errors.New("category in use")
	ErrVersionConflict = errors.New("submission version conflict")
	ErrSelfVote        = errors.New("cannot vote for own submission")
)

type FeedParams struct {
	Category string
	Watched  *bool
	Sort     string
	Cursor   string
	Limit    int
	UserID   string
}

type CreateSubmissionInput struct {
	YouTubeID        string
	YouTubeURL       string
	Title            string
	ChannelTitle     string
	ThumbnailURL     string
	DurationSeconds  int
	ViewCount        int64
	YouTubeLikeCount int64
	AuthorID         string
	CategoryID       string
	Comment          string
	DailyLimit       int
}

type DecideInput struct {
	SubmissionID string
	ModeratorID  string
	Status       domain.SubmissionStatus
	ReasonCode   string
	Comment      string
	Version      int
}

type Session struct {
	User      domain.User
	CSRFHash  []byte
	ExpiresAt time.Time
}

type Store interface {
	Ping(context.Context) error
	Close()

	ListCategories(context.Context) ([]domain.Category, error)
	ListFeed(context.Context, FeedParams) ([]domain.Video, string, error)
	ListByStatus(context.Context, domain.SubmissionStatus, int) ([]domain.Video, error)
	ListMine(context.Context, string, int) ([]domain.Video, error)
	CreateSubmission(context.Context, CreateSubmissionInput) (domain.Video, error)
	Vote(context.Context, string, string, int) (int64, int, error)
	Decide(context.Context, DecideInput) (domain.Video, error)
	SetWatched(context.Context, string, string, bool) error
	UpdateVideoCategory(context.Context, string, string, string) error
	DeleteVideo(context.Context, string, string) error

	CreateCategory(context.Context, string, string, string) (domain.Category, error)
	DeleteCategory(context.Context, string, string) error
	AssignModerator(context.Context, string, string) (domain.User, error)
	RemoveModerator(context.Context, string, string) error
	ListModerators(context.Context) ([]domain.User, error)
	ListNotifications(context.Context, string, int) ([]domain.Notification, error)
	MarkNotificationRead(context.Context, string, string) error
	MarkAllNotificationsRead(context.Context, string) error
	ListAudit(context.Context, int) ([]domain.AuditEntry, error)
	GetSettings(context.Context) (domain.GlobalSettings, error)
	UpdateSettings(context.Context, domain.GlobalSettings, string) error
	UpsertTwitchUser(context.Context, string, string, string, string) (domain.User, error)
	PromoteTwitchOwner(context.Context, string) (domain.User, error)
	CreateOAuthState(context.Context, []byte, []byte, string, time.Time) error
	ConsumeOAuthState(context.Context, []byte) (string, error)

	SessionByTokenHash(context.Context, []byte) (Session, error)
	CreateSession(context.Context, string, []byte, []byte, time.Time) error
	RevokeSession(context.Context, []byte) error
}

package store

import (
	"context"
	"errors"
	"time"

	"github.com/ravshann/predlozhka/backend/internal/domain"
	"github.com/ravshann/predlozhka/backend/internal/twitch"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("conflict")
	ErrDailyLimit      = errors.New("daily submission limit reached")
	ErrDuplicate       = errors.New("duplicate YouTube video")
	ErrCategoryInUse   = errors.New("category in use")
	ErrVersionConflict = errors.New("submission version conflict")
	ErrSelfVote        = errors.New("cannot vote for own submission")
	ErrOpenAppeal      = errors.New("open unban appeal already exists")
)

type FeedParams struct {
	Category string
	Watched  *bool
	Sort     string
	Cursor   string
	Limit    int
	UserID   string
}

type UserStatsParams struct {
	UserID string
	Query  string
	Role   string
	Sort   string
	Limit  int
	Offset int
}

type AuditParams struct {
	Query      string
	UserID     string
	Action     string
	TargetType string
	From       *time.Time
	To         *time.Time
	Limit      int
	Offset     int
}

type CreateSubmissionInput struct {
	ContentKind      string
	SourceType       string
	SourceURL        string
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
	KinopoiskURL     string
	MovieTitle       string
	MovieYear        *int
	MovieStudio      string
	MovieRating      *float64
	DailyLimit       int
}

type UpdateMovieInput struct {
	SubmissionID string
	ModeratorID  string
	KinopoiskURL string
	MovieTitle   string
	MovieYear    *int
	MovieStudio  string
	MovieRating  *float64
	Version      int
}

type UpdateSubmissionContentInput struct {
	SubmissionID string
	ModeratorID  string
	Title        string
	SourceURL    string
	Comment      string
	Version      int
}

type DecideInput struct {
	SubmissionID string
	ModeratorID  string
	Status       domain.SubmissionStatus
	ReasonCode   string
	Comment      string
	Version      int
}

type CreateUnbanAppealInput struct {
	AuthorID       string
	Platform       string
	Community      string
	BannedUsername string
	BanReason      string
	Statement      string
	Position       string
}

type UnbanAppealParams struct {
	Status   string
	Platform string
	Query    string
	Limit    int
	Offset   int
}

type ReviewUnbanAppealInput struct {
	AppealID         string
	ModeratorID      string
	Status           domain.UnbanAppealStatus
	ModeratorComment string
	InternalNote     string
}

type Session struct {
	User      domain.User
	CSRFHash  []byte
	ExpiresAt time.Time
}

type TwitchCacheStatus struct {
	Key       string    `json:"key"`
	ItemCount int       `json:"item_count"`
	UpdatedAt time.Time `json:"updated_at"`
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
	UpdateSubmissionContent(context.Context, UpdateSubmissionContentInput) (domain.Video, error)
	UpdateMovieMetadata(context.Context, UpdateMovieInput) (domain.Video, error)
	DeleteVideo(context.Context, string, string) error
	CreateUnbanAppeal(context.Context, CreateUnbanAppealInput) (domain.UnbanAppeal, error)
	ListMyUnbanAppeals(context.Context, string, int) ([]domain.UnbanAppeal, error)
	ListUnbanAppeals(context.Context, UnbanAppealParams) ([]domain.UnbanAppeal, int, error)
	ReviewUnbanAppeal(context.Context, ReviewUnbanAppealInput) (domain.UnbanAppeal, error)
	WithdrawUnbanAppeal(context.Context, string, string) (domain.UnbanAppeal, error)

	CreateCategory(context.Context, string, string, string) (domain.Category, error)
	DeleteCategory(context.Context, string, string) error
	AssignModerator(context.Context, string, string) (domain.User, error)
	RemoveModerator(context.Context, string, string) error
	ListModerators(context.Context) ([]domain.User, error)
	ListUsersStats(context.Context, UserStatsParams) ([]domain.UserStats, int, error)
	UserDetail(context.Context, string) (domain.UserDetail, error)
	ListNotifications(context.Context, string, int) ([]domain.Notification, error)
	MarkNotificationRead(context.Context, string, string) error
	MarkAllNotificationsRead(context.Context, string) error
	ListAudit(context.Context, AuditParams) ([]domain.AuditEntry, int, error)
	WriteAudit(context.Context, string, string, string, string, map[string]any) error
	GetSettings(context.Context) (domain.GlobalSettings, error)
	UpdateSettings(context.Context, domain.GlobalSettings, string) error
	UpsertTwitchUser(context.Context, string, string, string, string) (domain.User, error)
	PromoteTwitchOwner(context.Context, string) (domain.User, error)
	CreateOAuthState(context.Context, []byte, []byte, string, time.Time) error
	ConsumeOAuthState(context.Context, []byte) (string, error)
	ListNews(context.Context, int) ([]domain.NewsPost, error)
	CreateNewsPost(context.Context, string, string, string) (domain.NewsPost, error)
	DeleteNewsPost(context.Context, string, string) error
	CreateNewsComment(context.Context, string, string, string) (domain.NewsComment, error)
	DeleteNewsComment(context.Context, string, string, bool) error
	UpsertTwitchClips(context.Context, []twitch.Clip, string) error
	ListTwitchClips(context.Context, []string, *time.Time, *time.Time) ([]twitch.Clip, error)
	ListTwitchClipsByVideo(context.Context, string) ([]twitch.Clip, error)
	UpsertTwitchVideos(context.Context, []twitch.Video, string) error
	ListTwitchVideos(context.Context, []string) ([]twitch.Video, error)
	TwitchVideoByID(context.Context, string) (twitch.Video, error)
	TwitchCacheStatuses(context.Context) ([]TwitchCacheStatus, error)
	TwitchCacheStatus(context.Context, string) (TwitchCacheStatus, error)

	SessionByTokenHash(context.Context, []byte) (Session, error)
	CreateSession(context.Context, string, []byte, []byte, time.Time) error
	RevokeSession(context.Context, []byte) error
}

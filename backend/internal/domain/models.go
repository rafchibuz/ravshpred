package domain

import (
	"encoding/json"
	"time"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleModerator Role = "moderator"
	RoleOwner     Role = "owner"
)

type SubmissionStatus string

const (
	StatusPending          SubmissionStatus = "pending"
	StatusApproved         SubmissionStatus = "approved"
	StatusRejected         SubmissionStatus = "rejected"
	StatusChangesRequested SubmissionStatus = "changes_requested"
	StatusHidden           SubmissionStatus = "hidden"
)

type User struct {
	ID        string    `json:"id"`
	TwitchID  string    `json:"twitch_id"`
	Login     string    `json:"login"`
	Display   string    `json:"display_name"`
	AvatarURL string    `json:"avatar_url"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type Category struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	IsSystem  bool      `json:"is_system"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

type Video struct {
	ID               string           `json:"id"`
	YouTubeID        string           `json:"youtube_id"`
	YouTubeURL       string           `json:"youtube_url"`
	Title            string           `json:"title"`
	ChannelTitle     string           `json:"channel_title"`
	ThumbnailURL     string           `json:"thumbnail_url"`
	DurationSeconds  int              `json:"duration_seconds"`
	ViewCount        int64            `json:"view_count"`
	YouTubeLikeCount int64            `json:"youtube_like_count"`
	Author           User             `json:"author"`
	Category         Category         `json:"category"`
	Status           SubmissionStatus `json:"status"`
	SubmitterComment string           `json:"submitter_comment"`
	ModeratorComment string           `json:"moderator_comment,omitempty"`
	KinopoiskURL     string           `json:"kinopoisk_url,omitempty"`
	MovieTitle       string           `json:"movie_title,omitempty"`
	MovieYear        *int             `json:"movie_year,omitempty"`
	MovieStudio      string           `json:"movie_studio,omitempty"`
	MovieRating      *float64         `json:"movie_rating,omitempty"`
	ContentKind      string           `json:"content_kind"`
	SourceType       string           `json:"source_type"`
	SourceURL        string           `json:"source_url,omitempty"`
	Watched          bool             `json:"watched"`
	Rating           int64            `json:"rating"`
	UserVote         int              `json:"user_vote,omitempty"`
	RavshTOKStatus   string           `json:"ravshtok_status,omitempty"`
	RavshTOKError    string           `json:"ravshtok_error,omitempty"`
	RavshTOKAttempts int              `json:"ravshtok_attempts,omitempty"`
	Version          int              `json:"version"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

type RavshTOKItem struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	SourceURL       string    `json:"source_url"`
	Platform        string    `json:"platform"`
	MediaStatus     string    `json:"media_status"`
	PlaybackURL     string    `json:"playback_url,omitempty"`
	PosterURL       string    `json:"poster_url,omitempty"`
	DurationSeconds int       `json:"duration_seconds"`
	Width           int       `json:"width"`
	Height          int       `json:"height"`
	Likes           int64     `json:"likes"`
	Dislikes        int64     `json:"dislikes"`
	UserVote        int       `json:"user_vote"`
	UserViewed      bool      `json:"user_viewed"`
	StreamerWatched bool      `json:"streamer_watched"`
	Author          User      `json:"author"`
	CreatedAt       time.Time `json:"created_at"`
}

type RavshTOKFeed struct {
	Items   []RavshTOKItem `json:"items"`
	HasMore bool           `json:"has_more"`
}

type Notification struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type UnbanAppealStatus string

const (
	UnbanPending   UnbanAppealStatus = "pending"
	UnbanInReview  UnbanAppealStatus = "in_review"
	UnbanNeedsInfo UnbanAppealStatus = "needs_info"
	UnbanApproved  UnbanAppealStatus = "approved"
	UnbanRejected  UnbanAppealStatus = "rejected"
	UnbanWithdrawn UnbanAppealStatus = "withdrawn"
	UnbanDuplicate UnbanAppealStatus = "duplicate"
)

type UnbanAppeal struct {
	ID               string            `json:"id"`
	Author           User              `json:"author"`
	Platform         string            `json:"platform"`
	Community        string            `json:"community"`
	BannedUsername   string            `json:"banned_username"`
	BanReason        string            `json:"ban_reason"`
	Statement        string            `json:"statement"`
	Position         string            `json:"position"`
	Status           UnbanAppealStatus `json:"status"`
	ModeratorComment string            `json:"moderator_comment,omitempty"`
	InternalNote     string            `json:"internal_note,omitempty"`
	Moderator        *User             `json:"moderator,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
	ResolvedAt       *time.Time        `json:"resolved_at,omitempty"`
}

type NewsComment struct {
	ID        string    `json:"id"`
	PostID    string    `json:"post_id"`
	Author    User      `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type NewsPost struct {
	ID        string        `json:"id"`
	Author    User          `json:"author"`
	Title     string        `json:"title"`
	Body      string        `json:"body"`
	Comments  []NewsComment `json:"comments"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type AuditEntry struct {
	ID         int64           `json:"id"`
	Actor      *User           `json:"actor,omitempty"`
	Action     string          `json:"action"`
	TargetType string          `json:"target_type"`
	TargetID   string          `json:"target_id"`
	Metadata   json.RawMessage `json:"metadata"`
	CreatedAt  time.Time       `json:"created_at"`
}

type GlobalSettings struct {
	SubmissionDailyLimit int          `json:"submission_daily_limit"`
	RavshTOKDailyLimit   int          `json:"ravshtok_daily_limit"`
	CommentLimit         int          `json:"submission_comment_limit"`
	PublicFeedEnabled    bool         `json:"public_feed_enabled"`
	AllowSelfVote        bool         `json:"allow_self_vote"`
	Socials              SocialLinks  `json:"socials"`
	SocialItems          []SocialItem `json:"social_links"`
	SupportItems         []SocialItem `json:"support_links"`
	PartnerLinks         PartnerLinks `json:"partner_links"`
}

type PartnerLinks struct {
	BetBoom   string `json:"betboom"`
	Majestic  string `json:"majestic"`
	LitEnergy string `json:"lit_energy"`
}

type SocialItem struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Section string `json:"section,omitempty"`
	Icon    string `json:"icon,omitempty"`
}

type SocialLinks struct {
	Twitch   string `json:"twitch"`
	YouTube  string `json:"youtube"`
	Telegram string `json:"telegram"`
	VK       string `json:"vk"`
}

type StreamerStatus struct {
	Login        string       `json:"login"`
	DisplayName  string       `json:"display_name"`
	AvatarURL    string       `json:"avatar_url"`
	Live         bool         `json:"live"`
	Title        string       `json:"title,omitempty"`
	GameName     string       `json:"game_name,omitempty"`
	ViewerCount  int          `json:"viewer_count,omitempty"`
	StartedAt    *time.Time   `json:"started_at,omitempty"`
	ThumbnailURL string       `json:"thumbnail_url,omitempty"`
	Socials      SocialLinks  `json:"socials"`
	SocialItems  []SocialItem `json:"social_links"`
	SupportItems []SocialItem `json:"support_links"`
	PartnerLinks PartnerLinks `json:"partner_links"`
}

type UserStats struct {
	User             User       `json:"user"`
	LastLogin        *time.Time `json:"last_login_at,omitempty"`
	Total            int        `json:"total"`
	Videos           int        `json:"videos"`
	Ideas            int        `json:"ideas"`
	Pending          int        `json:"pending"`
	Approved         int        `json:"approved"`
	Rejected         int        `json:"rejected"`
	ChangesRequested int        `json:"changes_requested"`
	Hidden           int        `json:"hidden"`
	Watched          int        `json:"watched"`
	Deleted          int        `json:"deleted"`
	Comments         int        `json:"comments"`
	Votes            int        `json:"votes"`
	Actions          int        `json:"actions"`
}

type UserSubmissionActivity struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	ContentKind string           `json:"content_kind"`
	Status      SubmissionStatus `json:"status"`
	Deleted     bool             `json:"deleted"`
	Watched     bool             `json:"watched"`
	CreatedAt   time.Time        `json:"created_at"`
}

type UserDetail struct {
	Stats       UserStats                `json:"stats"`
	Submissions []UserSubmissionActivity `json:"submissions"`
	Audit       []AuditEntry             `json:"audit"`
}

type ViewerRatingEntry struct {
	Rank           int        `json:"rank"`
	TwitchID       string     `json:"twitch_id"`
	Login          string     `json:"login"`
	DisplayName    string     `json:"display_name"`
	AvatarURL      string     `json:"avatar_url,omitempty"`
	Role           string     `json:"role"`
	Score          int        `json:"score"`
	Messages       int        `json:"messages"`
	ActiveStreams  int        `json:"active_streams"`
	StreamCoverage int        `json:"stream_coverage"`
	ActiveDays     int        `json:"active_days"`
	ActiveWeeks    int        `json:"active_weeks"`
	LastActivity   *time.Time `json:"last_activity,omitempty"`
	Confidence     string     `json:"confidence"`
}

type ViewerRatingStatus struct {
	Channel       string     `json:"channel"`
	Status        string     `json:"status"`
	CollectorUser string     `json:"collector_user,omitempty"`
	LastEventAt   *time.Time `json:"last_event_at,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type ViewerRating struct {
	Preparing           bool                 `json:"preparing,omitempty"`
	Stale               bool                 `json:"stale,omitempty"`
	Channel             string               `json:"channel"`
	Period              string               `json:"period"`
	GeneratedAt         time.Time            `json:"generated_at"`
	CollectionStartedAt *time.Time           `json:"collection_started_at,omitempty"`
	StreamCount         int                  `json:"stream_count"`
	ParticipantCount    int                  `json:"participant_count"`
	Items               []ViewerRatingEntry  `json:"items"`
	Me                  *ViewerRatingEntry   `json:"me,omitempty"`
	Collectors          []ViewerRatingStatus `json:"collectors"`
}

func Can(role Role, action string) bool {
	switch action {
	case "view_feed", "open_video":
		return true
	case "submit", "vote", "view_profile", "comment_news", "submit_unban_appeal":
		return role == RoleUser || role == RoleModerator || role == RoleOwner
	case "moderate", "mark_watched":
		return role == RoleModerator || role == RoleOwner
	case "manage":
		return role == RoleOwner
	default:
		return false
	}
}

func CanTransition(from, to SubmissionStatus) bool {
	switch from {
	case StatusPending, StatusChangesRequested:
		return to == StatusApproved || to == StatusRejected || to == StatusChangesRequested || to == StatusHidden
	case StatusApproved:
		return to == StatusRejected || to == StatusHidden
	case StatusRejected:
		return to == StatusPending || to == StatusHidden
	case StatusHidden:
		return to == StatusApproved
	default:
		return false
	}
}

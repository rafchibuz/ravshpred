package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ravshann/predlozhka/backend/internal/config"
	"github.com/ravshann/predlozhka/backend/internal/domain"
	"github.com/ravshann/predlozhka/backend/internal/store"
	"github.com/ravshann/predlozhka/backend/internal/twitch"
	"github.com/ravshann/predlozhka/backend/internal/youtube"
)

type Server struct {
	cfg                config.Config
	store              store.Store
	youtube            youtube.Client
	twitch             *twitch.Client
	logger             *slog.Logger
	streamerMu         sync.Mutex
	streamerCache      domain.StreamerStatus
	streamerCacheUntil time.Time
	clipsMu            sync.Mutex
	clipsCache         map[string]clipsCacheEntry
}

type clipsCacheEntry struct {
	Items []twitch.Clip
	Until time.Time
}

type actorContext struct {
	User     domain.User
	CSRFHash []byte
}

type contextKey string

const actorKey contextKey = "actor"

func New(cfg config.Config, database store.Store, youtubeClient youtube.Client, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		cfg: cfg, store: database, youtube: youtubeClient,
		twitch:     twitch.New(cfg.TwitchClientID, cfg.TwitchClientSecret, cfg.TwitchRedirectURL),
		logger:     logger,
		clipsCache: make(map[string]clipsCacheEntry),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", s.live)
	mux.HandleFunc("GET /health/ready", s.ready)
	mux.HandleFunc("GET /api/categories", s.categories)
	mux.HandleFunc("GET /api/videos", s.feed)
	mux.HandleFunc("GET /api/streamer", s.streamer)
	mux.HandleFunc("GET /api/twitch/clips", s.twitchClips)
	mux.HandleFunc("GET /api/news", s.news)
	mux.HandleFunc("POST /api/news/{id}/comments", s.createNewsComment)
	mux.HandleFunc("DELETE /api/news/comments/{id}", s.deleteNewsComment)
	mux.HandleFunc("GET /api/me", s.me)
	mux.HandleFunc("GET /api/auth/twitch/start", s.twitchStart)
	mux.HandleFunc("GET /api/auth/twitch/callback", s.twitchCallback)
	mux.HandleFunc("POST /api/logout", s.logout)
	mux.HandleFunc("POST /api/submissions", s.createSubmission)
	mux.HandleFunc("GET /api/submissions/mine", s.mine)
	mux.HandleFunc("GET /api/notifications", s.notifications)
	mux.HandleFunc("POST /api/notifications/read-all", s.readAllNotifications)
	mux.HandleFunc("POST /api/notifications/{id}/read", s.readNotification)
	mux.HandleFunc("PUT /api/videos/{id}/vote", s.vote)
	mux.HandleFunc("GET /api/moderation/submissions", s.moderationList)
	mux.HandleFunc("PATCH /api/moderation/submissions/{id}", s.moderate)
	mux.HandleFunc("PATCH /api/moderation/submissions/{id}/watched", s.watched)
	mux.HandleFunc("PATCH /api/moderation/submissions/{id}/category", s.videoCategory)
	mux.HandleFunc("PATCH /api/moderation/submissions/{id}/content", s.videoContent)
	mux.HandleFunc("PATCH /api/moderation/submissions/{id}/movie", s.videoMovie)
	mux.HandleFunc("DELETE /api/moderation/submissions/{id}", s.deleteVideo)
	mux.HandleFunc("POST /api/owner/categories", s.createCategory)
	mux.HandleFunc("DELETE /api/owner/categories/{id}", s.deleteCategory)
	mux.HandleFunc("POST /api/owner/moderators", s.assignModerator)
	mux.HandleFunc("GET /api/owner/moderators", s.listModerators)
	mux.HandleFunc("GET /api/owner/users", s.listUsers)
	mux.HandleFunc("DELETE /api/owner/moderators/{id}", s.removeModerator)
	mux.HandleFunc("GET /api/owner/audit", s.audit)
	mux.HandleFunc("GET /api/owner/settings", s.settings)
	mux.HandleFunc("PUT /api/owner/settings", s.updateSettings)
	mux.HandleFunc("POST /api/owner/news", s.createNewsPost)
	mux.HandleFunc("DELETE /api/owner/news/{id}", s.deleteNewsPost)
	return s.middleware(mux)
}

func (s *Server) twitchClips(w http.ResponseWriter, r *http.Request) {
	if !s.twitch.Configured() {
		writeError(w, http.StatusServiceUnavailable, "twitch_not_configured", "Twitch API не настроен")
		return
	}
	cacheKey := r.URL.Query().Encode()
	channel := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("channel")))
	logins := []string{"ravshann", "ravshanbtw"}
	if channel != "" && channel != "all" {
		if channel != "ravshann" && channel != "ravshanbtw" {
			writeError(w, http.StatusBadRequest, "invalid_channel", "Неизвестный Twitch-канал")
			return
		}
		logins = []string{channel}
	}

	now := time.Now().UTC()
	var startedAt, endedAt *time.Time
	switch r.URL.Query().Get("period") {
	case "", "week":
		value := now.AddDate(0, 0, -7)
		startedAt, endedAt = &value, &now
	case "today":
		value := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		startedAt, endedAt = &value, &now
	case "month":
		value := now.AddDate(0, -1, 0)
		startedAt, endedAt = &value, &now
	case "year":
		value := now.AddDate(-1, 0, 0)
		startedAt, endedAt = &value, &now
	case "all":
	case "custom":
		from, fromErr := time.Parse("2006-01-02", r.URL.Query().Get("from"))
		to, toErr := time.Parse("2006-01-02", r.URL.Query().Get("to"))
		if fromErr != nil || toErr != nil || to.Before(from) {
			writeError(w, http.StatusBadRequest, "invalid_period", "Укажите корректный период")
			return
		}
		to = to.Add(24*time.Hour - time.Nanosecond)
		startedAt, endedAt = &from, &to
	default:
		writeError(w, http.StatusBadRequest, "invalid_period", "Неизвестный период")
		return
	}

	ttl := 10 * time.Minute
	if r.URL.Query().Get("period") == "all" {
		ttl = time.Hour
	}
	items := s.loadTwitchClips(r.Context(), cacheKey, logins, startedAt, endedAt, ttl)
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (s *Server) loadTwitchClips(ctx context.Context, cacheKey string, logins []string, startedAt, endedAt *time.Time, ttl time.Duration) []twitch.Clip {
	s.clipsMu.Lock()
	if cached, ok := s.clipsCache[cacheKey]; ok && time.Now().Before(cached.Until) {
		s.clipsMu.Unlock()
		return cached.Items
	}
	s.clipsMu.Unlock()
	items := make([]twitch.Clip, 0, 100)
	for _, login := range logins {
		clips, err := s.twitch.ClipsByLogin(ctx, login, startedAt, endedAt)
		if err != nil {
			s.logger.Warn("twitch clips unavailable", "login", login, "error", err)
			continue
		}
		items = append(items, clips...)
	}
	if ctx.Err() == nil {
		s.clipsMu.Lock()
		s.clipsCache[cacheKey] = clipsCacheEntry{Items: items, Until: time.Now().Add(ttl)}
		s.clipsMu.Unlock()
	}
	return items
}

func (s *Server) StartClipCacheWarmer(ctx context.Context) {
	if !s.twitch.Configured() {
		return
	}
	go func() {
		warm := func() {
			now := time.Now().UTC()
			weekStart := now.AddDate(0, 0, -7)
			channels := []string{"ravshann", "ravshanbtw"}
			s.loadTwitchClips(ctx, "channel=all&period=week", channels, &weekStart, &now, 10*time.Minute)
			if ctx.Err() == nil {
				s.loadTwitchClips(ctx, "channel=all&period=all", channels, nil, nil, time.Hour)
			}
		}
		warm()
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				warm()
			}
		}
	}()
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := randomToken(12)
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		if origin := r.Header.Get("Origin"); origin != "" && origin == s.cfg.FrontendURL {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type,X-CSRF-Token")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Error("panic", "request_id", requestID, "error", recovered)
				writeError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка")
			}
			s.logger.Info("request",
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_unavailable", "База данных недоступна")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) categories(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListCategories(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (s *Server) streamer(w http.ResponseWriter, r *http.Request) {
	s.streamerMu.Lock()
	defer s.streamerMu.Unlock()
	if time.Now().Before(s.streamerCacheUntil) {
		writeJSON(w, http.StatusOK, map[string]any{"data": s.streamerCache})
		return
	}
	settings, err := s.store.GetSettings(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	result := domain.StreamerStatus{Login: "ravshann", DisplayName: "RavshanN", Socials: settings.Socials, SocialItems: settings.SocialItems, SupportItems: settings.SupportItems}
	if !s.twitch.Configured() {
		s.streamerCache, s.streamerCacheUntil = result, time.Now().Add(time.Minute)
		writeJSON(w, http.StatusOK, map[string]any{"data": result})
		return
	}
	var fallbackUser *twitch.User
	failedLookups := 0
	for _, login := range []string{"ravshann", "ravshanbtw"} {
		user, stream, lookupErr := s.twitch.StreamByLogin(r.Context(), login)
		if lookupErr != nil {
			failedLookups++
			s.logger.Warn("twitch streamer status unavailable", "login", login, "error", lookupErr)
			continue
		}
		if fallbackUser == nil {
			fallbackUser = &user
		}
		if stream == nil {
			continue
		}
		result.Login, result.DisplayName, result.AvatarURL = user.Login, user.DisplayName, user.AvatarURL
		result.Live, result.Title, result.GameName = true, stream.Title, stream.GameName
		result.ViewerCount, result.StartedAt = stream.ViewerCount, &stream.StartedAt
		result.ThumbnailURL = strings.ReplaceAll(strings.ReplaceAll(stream.ThumbnailURL, "{width}", "1280"), "{height}", "720")
		break
	}
	if !result.Live && fallbackUser != nil {
		result.Login, result.DisplayName, result.AvatarURL = fallbackUser.Login, fallbackUser.DisplayName, fallbackUser.AvatarURL
	}
	cacheDuration := time.Minute
	if failedLookups == 2 {
		cacheDuration = 30 * time.Second
	}
	s.streamerCache, s.streamerCacheUntil = result, time.Now().Add(cacheDuration)
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (s *Server) news(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListNews(r.Context(), intQuery(r, "limit", 50))
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (s *Server) createNewsPost(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "manage")
	if actor == nil {
		return
	}
	var input struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Body = strings.TrimSpace(input.Body)
	if len([]rune(input.Title)) < 1 || len([]rune(input.Title)) > 160 ||
		len([]rune(input.Body)) < 1 || len([]rune(input.Body)) > 5000 {
		writeError(w, http.StatusBadRequest, "invalid_news_post", "Заголовок или текст новости вне допустимого размера")
		return
	}
	post, err := s.store.CreateNewsPost(r.Context(), actor.User.ID, input.Title, input.Body)
	if err != nil {
		s.storeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": post})
}

func (s *Server) deleteNewsPost(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "manage")
	if actor == nil {
		return
	}
	if err := s.store.DeleteNewsPost(r.Context(), r.PathValue("id"), actor.User.ID); err != nil {
		s.storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) createNewsComment(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "comment_news")
	if actor == nil {
		return
	}
	var input struct {
		Body string `json:"body"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Body = strings.TrimSpace(input.Body)
	if len([]rune(input.Body)) < 1 || len([]rune(input.Body)) > 1000 {
		writeError(w, http.StatusBadRequest, "invalid_news_comment", "Комментарий должен содержать от 1 до 1000 символов")
		return
	}
	comment, err := s.store.CreateNewsComment(r.Context(), r.PathValue("id"), actor.User.ID, input.Body)
	if err != nil {
		s.storeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": comment})
}

func (s *Server) deleteNewsComment(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "comment_news")
	if actor == nil {
		return
	}
	if err := s.store.DeleteNewsComment(
		r.Context(),
		r.PathValue("id"),
		actor.User.ID,
		actor.User.Role == domain.RoleOwner,
	); err != nil {
		s.storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) feed(w http.ResponseWriter, r *http.Request) {
	actor, _ := s.actor(r)
	params := store.FeedParams{
		Category: r.URL.Query().Get("category"),
		Sort:     r.URL.Query().Get("sort"),
		Cursor:   r.URL.Query().Get("cursor"),
		Limit:    intQuery(r, "limit", 20),
	}
	if actor != nil {
		params.UserID = actor.User.ID
	}
	if value := r.URL.Query().Get("watched"); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_filter", "Некорректный фильтр watched")
			return
		}
		params.Watched = &parsed
	}
	items, next, err := s.store.ListFeed(r.Context(), params)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items, "next_cursor": next})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actor(r)
	if err != nil || actor == nil {
		writeJSON(w, http.StatusOK, map[string]any{"user": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": actor.User})
}

func (s *Server) twitchStart(w http.ResponseWriter, r *http.Request) {
	if !s.twitch.Configured() {
		writeError(w, http.StatusServiceUnavailable, "twitch_not_configured", "Вход через Twitch ещё не настроен")
		return
	}
	state, verifier := randomToken(32), randomToken(32)
	returnTo := r.URL.Query().Get("return_to")
	if !strings.HasPrefix(returnTo, "/") || strings.HasPrefix(returnTo, "//") {
		returnTo = "/"
	}
	if err := s.store.CreateOAuthState(r.Context(), hash(state), hash(verifier), returnTo, time.Now().Add(10*time.Minute)); err != nil {
		s.internalError(w, err)
		return
	}
	http.Redirect(w, r, s.twitch.AuthorizationURL(state), http.StatusFound)
}

func (s *Server) twitchCallback(w http.ResponseWriter, r *http.Request) {
	if oauthError := r.URL.Query().Get("error"); oauthError != "" {
		http.Redirect(w, r, s.cfg.BaseURL+"/?auth_error=denied", http.StatusFound)
		return
	}
	state, code := r.URL.Query().Get("state"), r.URL.Query().Get("code")
	if state == "" || code == "" {
		http.Redirect(w, r, s.cfg.BaseURL+"/?auth_error=invalid_callback", http.StatusFound)
		return
	}
	returnTo, err := s.store.ConsumeOAuthState(r.Context(), hash(state))
	if err != nil {
		http.Redirect(w, r, s.cfg.BaseURL+"/?auth_error=expired_state", http.StatusFound)
		return
	}
	twitchUser, err := s.twitch.Exchange(r.Context(), code)
	if err != nil {
		s.logger.Error("twitch authentication failed", "error", err)
		http.Redirect(w, r, s.cfg.BaseURL+"/?auth_error=twitch", http.StatusFound)
		return
	}
	user, err := s.store.UpsertTwitchUser(r.Context(), twitchUser.ID, twitchUser.Login, twitchUser.DisplayName, twitchUser.AvatarURL)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if (s.cfg.OwnerTwitchID != "" && twitchUser.ID == s.cfg.OwnerTwitchID) ||
		strings.EqualFold(twitchUser.Login, s.cfg.OwnerTwitchLogin) {
		user, err = s.store.PromoteTwitchOwner(r.Context(), user.ID)
		if err != nil {
			s.internalError(w, err)
			return
		}
	}
	if err := s.issueSession(w, r, user); err != nil {
		s.internalError(w, err)
		return
	}
	http.Redirect(w, r, strings.TrimRight(s.cfg.BaseURL, "/")+returnTo, http.StatusFound)
}

func (s *Server) issueSession(w http.ResponseWriter, r *http.Request, user domain.User) error {
	token, csrf := randomToken(32), randomToken(24)
	expires := time.Now().Add(s.cfg.SessionTTL)
	if err := s.store.CreateSession(r.Context(), user.ID, hash(token), hash(csrf), expires); err != nil {
		return err
	}
	s.setSessionCookies(w, token, csrf, expires)
	return nil
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(s.cfg.SessionCookieName); err == nil {
		_ = s.store.RevokeSession(r.Context(), hash(cookie.Value))
	}
	s.clearSessionCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) createSubmission(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "submit")
	if actor == nil {
		return
	}
	var input struct {
		ContentKind  string   `json:"content_kind"`
		SourceType   string   `json:"source_type"`
		URL          string   `json:"url"`
		Title        string   `json:"title"`
		CategoryID   string   `json:"category_id"`
		Comment      string   `json:"comment"`
		KinopoiskURL string   `json:"kinopoisk_url"`
		MovieTitle   string   `json:"movie_title"`
		MovieYear    *int     `json:"movie_year"`
		MovieStudio  string   `json:"movie_studio"`
		MovieRating  *float64 `json:"movie_rating"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.ContentKind = strings.TrimSpace(input.ContentKind)
	input.SourceType = strings.TrimSpace(input.SourceType)
	if input.ContentKind == "" {
		input.ContentKind = "video"
	}
	if input.SourceType == "" {
		input.SourceType = "youtube"
	}
	if input.ContentKind != "video" && input.ContentKind != "stream_idea" {
		writeError(w, http.StatusBadRequest, "invalid_content_kind", "Выберите видео или идею для стрима")
		return
	}
	if input.ContentKind == "stream_idea" {
		input.SourceType = "idea"
	} else if input.SourceType != "youtube" && input.SourceType != "short_video" && input.SourceType != "external" {
		writeError(w, http.StatusBadRequest, "invalid_source_type", "Выберите источник предложения")
		return
	}
	settings, err := s.store.GetSettings(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	categories, err := s.store.ListCategories(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	movieCategory := false
	categoryExists := false
	for _, category := range categories {
		if category.ID == input.CategoryID {
			categoryExists = true
			name := strings.ToLower(category.Name)
			movieCategory = strings.Contains(name, "трейлер") || strings.Contains(name, "фильм") || strings.Contains(name, "сериал")
			break
		}
	}
	if !categoryExists {
		writeError(w, http.StatusBadRequest, "invalid_category", "Выберите существующую категорию")
		return
	}
	if input.ContentKind == "video" && movieCategory && strings.TrimSpace(input.KinopoiskURL) == "" {
		writeError(w, http.StatusBadRequest, "kinopoisk_required", "Для фильма нужна ссылка на Кинопоиск")
		return
	}
	if len([]rune(input.Comment)) > settings.CommentLimit {
		writeError(w, http.StatusBadRequest, "comment_too_long", "Комментарий превышает допустимую длину")
		return
	}
	if !validMovieMetadata(input.KinopoiskURL, input.MovieTitle, input.MovieYear, input.MovieStudio, input.MovieRating) {
		writeError(w, http.StatusBadRequest, "invalid_movie_metadata", "Проверьте ссылку на Кинопоиск и данные фильма")
		return
	}
	var youtubeID, canonicalURL, title, channelTitle, thumbnailURL string
	var durationSeconds int
	var viewCount, likeCount int64
	sourceURL := strings.TrimSpace(input.URL)
	if input.SourceType == "youtube" {
		id, parseErr := domain.ParseYouTubeID(sourceURL)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid_youtube_url", "Некорректная ссылка YouTube")
			return
		}
		metadata, fetchErr := s.youtube.Fetch(r.Context(), id)
		if fetchErr != nil {
			writeError(w, http.StatusUnprocessableEntity, "youtube_unavailable", fetchErr.Error())
			return
		}
		youtubeID, canonicalURL, sourceURL = id, domain.CanonicalYouTubeURL(id), domain.CanonicalYouTubeURL(id)
		title, channelTitle, thumbnailURL = metadata.Title, metadata.ChannelTitle, metadata.ThumbnailURL
		durationSeconds, viewCount, likeCount = metadata.DurationSeconds, metadata.ViewCount, metadata.YouTubeLikeCount
	} else if input.ContentKind == "stream_idea" {
		title = strings.TrimSpace(input.Title)
		if title == "" || len([]rune(title)) > 160 {
			writeError(w, http.StatusBadRequest, "invalid_idea_title", "Добавьте название идеи до 160 символов")
			return
		}
		channelTitle = "Идея для стрима"
		sourceURL = ""
	} else {
		parsedURL, parseErr := url.ParseRequestURI(sourceURL)
		if parseErr != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
			writeError(w, http.StatusBadRequest, "invalid_source_url", "Добавьте корректную ссылку http или https")
			return
		}
		title = strings.TrimSpace(input.Title)
		if title == "" || len([]rune(title)) > 160 {
			writeError(w, http.StatusBadRequest, "invalid_submission_title", "Добавьте название до 160 символов")
			return
		}
		if input.SourceType == "short_video" {
			channelTitle = "TikTok / Instagram"
		} else {
			channelTitle = parsedURL.Hostname()
		}
	}
	video, err := s.store.CreateSubmission(r.Context(), store.CreateSubmissionInput{
		ContentKind:      input.ContentKind,
		SourceType:       input.SourceType,
		SourceURL:        sourceURL,
		YouTubeID:        youtubeID,
		YouTubeURL:       canonicalURL,
		Title:            title,
		ChannelTitle:     channelTitle,
		ThumbnailURL:     thumbnailURL,
		DurationSeconds:  durationSeconds,
		ViewCount:        viewCount,
		YouTubeLikeCount: likeCount,
		AuthorID:         actor.User.ID,
		CategoryID:       input.CategoryID,
		Comment:          input.Comment,
		KinopoiskURL:     strings.TrimSpace(input.KinopoiskURL),
		MovieTitle:       strings.TrimSpace(input.MovieTitle),
		MovieYear:        input.MovieYear,
		MovieStudio:      strings.TrimSpace(input.MovieStudio),
		MovieRating:      input.MovieRating,
		DailyLimit:       settings.SubmissionDailyLimit,
	})
	if err != nil {
		s.storeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": video})
}

func (s *Server) mine(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "view_profile")
	if actor == nil {
		return
	}
	items, err := s.store.ListMine(r.Context(), actor.User.ID, intQuery(r, "limit", 50))
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (s *Server) notifications(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "view_profile")
	if actor == nil {
		return
	}
	items, err := s.store.ListNotifications(r.Context(), actor.User.ID, intQuery(r, "limit", 50))
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (s *Server) readNotification(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "view_profile")
	if actor == nil {
		return
	}
	if err := s.store.MarkNotificationRead(r.Context(), actor.User.ID, r.PathValue("id")); err != nil {
		s.storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) readAllNotifications(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "view_profile")
	if actor == nil {
		return
	}
	if err := s.store.MarkAllNotificationsRead(r.Context(), actor.User.ID); err != nil {
		s.internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) vote(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "vote")
	if actor == nil {
		return
	}
	var input struct {
		Value int `json:"value"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	rating, current, err := s.store.Vote(r.Context(), r.PathValue("id"), actor.User.ID, input.Value)
	if err != nil {
		s.storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rating": rating, "user_vote": current})
}

func (s *Server) moderationList(w http.ResponseWriter, r *http.Request) {
	if s.require(w, r, "moderate") == nil {
		return
	}
	status := domain.SubmissionStatus(r.URL.Query().Get("status"))
	if status == "" {
		status = domain.StatusPending
	}
	items, err := s.store.ListByStatus(r.Context(), status, intQuery(r, "limit", 50))
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (s *Server) moderate(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "moderate")
	if actor == nil {
		return
	}
	var input struct {
		Status     domain.SubmissionStatus `json:"status"`
		ReasonCode string                  `json:"reason_code"`
		Comment    string                  `json:"comment"`
		Version    int                     `json:"version"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	video, err := s.store.Decide(r.Context(), store.DecideInput{
		SubmissionID: r.PathValue("id"),
		ModeratorID:  actor.User.ID,
		Status:       input.Status,
		ReasonCode:   input.ReasonCode,
		Comment:      input.Comment,
		Version:      input.Version,
	})
	if err != nil {
		s.storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": video})
}

func (s *Server) watched(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "mark_watched")
	if actor == nil {
		return
	}
	var input struct {
		Watched bool `json:"watched"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.store.SetWatched(r.Context(), r.PathValue("id"), actor.User.ID, input.Watched); err != nil {
		s.storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) videoCategory(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "moderate")
	if actor == nil {
		return
	}
	var input struct {
		CategoryID string `json:"category_id"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.store.UpdateVideoCategory(r.Context(), r.PathValue("id"), input.CategoryID, actor.User.ID); err != nil {
		s.storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) videoMovie(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "moderate")
	if actor == nil {
		return
	}
	var input struct {
		KinopoiskURL string   `json:"kinopoisk_url"`
		MovieTitle   string   `json:"movie_title"`
		MovieYear    *int     `json:"movie_year"`
		MovieStudio  string   `json:"movie_studio"`
		MovieRating  *float64 `json:"movie_rating"`
		Version      int      `json:"version"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Version < 1 || !validMovieMetadata(input.KinopoiskURL, input.MovieTitle, input.MovieYear, input.MovieStudio, input.MovieRating) {
		writeError(w, http.StatusBadRequest, "invalid_movie_metadata", "Проверьте ссылку на Кинопоиск и данные фильма")
		return
	}
	video, err := s.store.UpdateMovieMetadata(r.Context(), store.UpdateMovieInput{
		SubmissionID: r.PathValue("id"), ModeratorID: actor.User.ID, Version: input.Version,
		KinopoiskURL: strings.TrimSpace(input.KinopoiskURL), MovieTitle: strings.TrimSpace(input.MovieTitle),
		MovieYear: input.MovieYear, MovieStudio: strings.TrimSpace(input.MovieStudio), MovieRating: input.MovieRating,
	})
	if err != nil {
		s.storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": video})
}

func (s *Server) videoContent(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "moderate")
	if actor == nil {
		return
	}
	var input struct {
		Title     string `json:"title"`
		SourceURL string `json:"source_url"`
		Comment   string `json:"comment"`
		Version   int    `json:"version"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Title, input.SourceURL, input.Comment = strings.TrimSpace(input.Title), strings.TrimSpace(input.SourceURL), strings.TrimSpace(input.Comment)
	if input.Version < 1 || input.Title == "" || len([]rune(input.Title)) > 160 || len([]rune(input.Comment)) > 500 {
		writeError(w, http.StatusBadRequest, "invalid_submission_content", "Проверьте название, ссылку и описание предложения")
		return
	}
	if input.SourceURL != "" {
		parsed, err := url.ParseRequestURI(input.SourceURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			writeError(w, http.StatusBadRequest, "invalid_source_url", "Добавьте корректную ссылку http или https")
			return
		}
	}
	video, err := s.store.UpdateSubmissionContent(r.Context(), store.UpdateSubmissionContentInput{
		SubmissionID: r.PathValue("id"), ModeratorID: actor.User.ID, Title: input.Title,
		SourceURL: input.SourceURL, Comment: input.Comment, Version: input.Version,
	})
	if err != nil {
		s.storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": video})
}

func (s *Server) deleteVideo(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "moderate")
	if actor == nil {
		return
	}
	if err := s.store.DeleteVideo(r.Context(), r.PathValue("id"), actor.User.ID); err != nil {
		s.storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) createCategory(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "manage")
	if actor == nil {
		return
	}
	var input struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Slug == "" {
		input.Slug = "category-" + strconv.FormatInt(time.Now().Unix(), 36)
	}
	category, err := s.store.CreateCategory(r.Context(), input.Slug, strings.TrimSpace(input.Name), actor.User.ID)
	if err != nil {
		s.storeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": category})
}

func (s *Server) deleteCategory(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "manage")
	if actor == nil {
		return
	}
	if err := s.store.DeleteCategory(r.Context(), r.PathValue("id"), actor.User.ID); err != nil {
		s.storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) assignModerator(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "manage")
	if actor == nil {
		return
	}
	var input struct {
		TwitchLogin string `json:"twitch_login"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	user, err := s.store.AssignModerator(r.Context(), strings.TrimPrefix(strings.TrimSpace(input.TwitchLogin), "@"), actor.User.ID)
	if err != nil {
		s.storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": user})
}

func (s *Server) listModerators(w http.ResponseWriter, r *http.Request) {
	if s.require(w, r, "manage") == nil {
		return
	}
	users, err := s.store.ListModerators(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": users})
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	if s.require(w, r, "manage") == nil {
		return
	}
	items, err := s.store.ListUsersStats(r.Context(), intQuery(r, "limit", 100))
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (s *Server) removeModerator(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "manage")
	if actor == nil {
		return
	}
	if err := s.store.RemoveModerator(r.Context(), r.PathValue("id"), actor.User.ID); err != nil {
		s.storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) audit(w http.ResponseWriter, r *http.Request) {
	if s.require(w, r, "manage") == nil {
		return
	}
	items, err := s.store.ListAudit(r.Context(), intQuery(r, "limit", 100))
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	if s.require(w, r, "manage") == nil {
		return
	}
	settings, err := s.store.GetSettings(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": settings})
}

func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "manage")
	if actor == nil {
		return
	}
	var input domain.GlobalSettings
	if !decodeJSON(w, r, &input) {
		return
	}
	validSocials := len(input.SocialItems) <= 12
	for _, item := range input.SocialItems {
		if len([]rune(strings.TrimSpace(item.Name))) < 1 || len([]rune(strings.TrimSpace(item.Name))) > 40 || !validOptionalURL(item.URL) || strings.TrimSpace(item.URL) == "" ||
			(item.Section != "primary" && item.Section != "more" && item.Section != "clips") {
			validSocials = false
		}
	}
	validSupport := len(input.SupportItems) <= 12
	for _, item := range input.SupportItems {
		if len([]rune(strings.TrimSpace(item.Name))) < 1 || len([]rune(strings.TrimSpace(item.Name))) > 40 || !validOptionalURL(item.URL) || strings.TrimSpace(item.URL) == "" || item.Section != "support" {
			validSupport = false
		}
	}
	if input.SubmissionDailyLimit < 1 || input.SubmissionDailyLimit > 20 ||
		input.CommentLimit < 100 || input.CommentLimit > 2000 ||
		!validSocials || !validSupport ||
		!validOptionalURL(input.Socials.Twitch) || !validOptionalURL(input.Socials.YouTube) ||
		!validOptionalURL(input.Socials.Telegram) || !validOptionalURL(input.Socials.VK) {
		writeError(w, http.StatusBadRequest, "invalid_settings", "Настройки вне допустимого диапазона")
		return
	}
	if err := s.store.UpdateSettings(r.Context(), input, actor.User.ID); err != nil {
		s.internalError(w, err)
		return
	}
	s.streamerMu.Lock()
	s.streamerCacheUntil = time.Time{}
	s.streamerMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"data": input})
}

func validOptionalURL(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" || strings.HasPrefix(value, "https://")
}

func validMovieMetadata(kinopoiskURL, title string, year *int, studio string, rating *float64) bool {
	if kinopoiskURL != "" && !validKinopoiskURL(kinopoiskURL) {
		return false
	}
	if len([]rune(strings.TrimSpace(title))) > 200 || len([]rune(strings.TrimSpace(studio))) > 200 {
		return false
	}
	if year != nil && (*year < 1888 || *year > 2100) {
		return false
	}
	return rating == nil || (*rating >= 0 && *rating <= 10)
}

func validKinopoiskURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "https" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "kinopoisk.ru" || strings.HasSuffix(host, ".kinopoisk.ru")
}

func (s *Server) actor(r *http.Request) (*actorContext, error) {
	if value := r.Context().Value(actorKey); value != nil {
		return value.(*actorContext), nil
	}
	cookie, err := r.Cookie(s.cfg.SessionCookieName)
	if err != nil {
		return nil, store.ErrNotFound
	}
	session, err := s.store.SessionByTokenHash(r.Context(), hash(cookie.Value))
	if err != nil {
		return nil, err
	}
	return &actorContext{User: session.User, CSRFHash: session.CSRFHash}, nil
}

func (s *Server) require(w http.ResponseWriter, r *http.Request, action string) *actorContext {
	actor, err := s.actor(r)
	if err != nil || actor == nil {
		writeError(w, http.StatusUnauthorized, "authentication_required", "Требуется авторизация")
		return nil
	}
	if !domain.Can(actor.User.Role, action) {
		writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав")
		return nil
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		csrfCookie, err := r.Cookie(s.cfg.SessionCookieName + "_csrf")
		csrfHeader := r.Header.Get("X-CSRF-Token")
		if err != nil || csrfHeader == "" || csrfCookie.Value != csrfHeader || subtle.ConstantTimeCompare(hash(csrfHeader), actor.CSRFHash) != 1 {
			writeError(w, http.StatusForbidden, "csrf_failed", "CSRF-проверка не пройдена")
			return nil
		}
	}
	return actor
}

func (s *Server) setSessionCookies(w http.ResponseWriter, token, csrf string, expires time.Time) {
	secure := strings.HasPrefix(s.cfg.BaseURL, "https://")
	http.SetCookie(w, &http.Cookie{
		Name: s.cfg.SessionCookieName, Value: token, Path: "/", HttpOnly: true,
		Secure: secure, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: int(time.Until(expires).Seconds()),
	})
	http.SetCookie(w, &http.Cookie{
		Name: s.cfg.SessionCookieName + "_csrf", Value: csrf, Path: "/",
		Secure: secure, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: int(time.Until(expires).Seconds()),
	})
}

func (s *Server) clearSessionCookies(w http.ResponseWriter) {
	for _, name := range []string{s.cfg.SessionCookieName, s.cfg.SessionCookieName + "_csrf"} {
		http.SetCookie(w, &http.Cookie{Name: name, Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: name == s.cfg.SessionCookieName})
	}
}

func (s *Server) storeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Запись не найдена")
	case errors.Is(err, store.ErrConflict), errors.Is(err, store.ErrVersionConflict), errors.Is(err, store.ErrDuplicate):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, store.ErrSelfVote):
		writeError(w, http.StatusForbidden, "self_vote_forbidden", "Нельзя голосовать за собственное видео")
	case errors.Is(err, store.ErrDailyLimit):
		writeError(w, http.StatusTooManyRequests, "daily_limit", "Достигнут лимит отправок за 24 часа")
	default:
		s.internalError(w, err)
	}
}

func (s *Server) internalError(w http.ResponseWriter, err error) {
	s.logger.Error("request failed", "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON")
		return false
	}
	return true
}

func intQuery(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func randomToken(bytes int) string {
	raw := make([]byte, bytes)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	return hex.EncodeToString(raw)
}

func hash(value string) []byte {
	sum := sha256.Sum256([]byte(value))
	return sum[:]
}

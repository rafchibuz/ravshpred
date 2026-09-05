package httpapi

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ravshann/predlozhka/backend/internal/config"
	"github.com/ravshann/predlozhka/backend/internal/domain"
	"github.com/ravshann/predlozhka/backend/internal/store"
	"github.com/ravshann/predlozhka/backend/internal/twitch"
	"github.com/ravshann/predlozhka/backend/internal/youtube"
)

type stubStore struct {
	store.Store
	categories   []domain.Category
	news         []domain.NewsPost
	sessionRole  domain.Role
	createdInput *store.CreateSubmissionInput
	updatedInput *store.UpdateSubmissionContentInput
	unbanInput   *store.CreateUnbanAppealInput
	rating       domain.ViewerRating
	ratingCalls  int
}

func (s *stubStore) Ping(context.Context) error { return nil }
func (s *stubStore) Close()                     {}
func (s *stubStore) ListCategories(context.Context) ([]domain.Category, error) {
	return s.categories, nil
}
func (s *stubStore) SessionByTokenHash(context.Context, []byte) (store.Session, error) {
	if s.sessionRole == "" {
		return store.Session{}, store.ErrNotFound
	}
	return store.Session{
		User: domain.User{
			ID:      "00000000-0000-0000-0000-000000000001",
			Display: "Twitch User",
			Role:    s.sessionRole,
		},
		CSRFHash: hash("csrf"),
	}, nil
}
func (s *stubStore) CreateCategory(_ context.Context, slug, name, _ string) (domain.Category, error) {
	return domain.Category{ID: "category-id", Slug: slug, Name: name}, nil
}
func (s *stubStore) ListNews(context.Context, int) ([]domain.NewsPost, error) {
	return s.news, nil
}
func (s *stubStore) CreateNewsPost(_ context.Context, authorID, title, body string) (domain.NewsPost, error) {
	return domain.NewsPost{ID: "news-id", Title: title, Body: body, Author: domain.User{ID: authorID}}, nil
}

type newsBroadcasterStub struct {
	stubStore
	notifyUsers []bool
}

func (s *newsBroadcasterStub) CreateNewsPostWithNotification(_ context.Context, authorID, title, body string, notifyUsers bool) (domain.NewsPost, error) {
	s.notifyUsers = append(s.notifyUsers, notifyUsers)
	return domain.NewsPost{ID: "news-id", Title: title, Body: body, Author: domain.User{ID: authorID}}, nil
}
func (s *stubStore) GetSettings(context.Context) (domain.GlobalSettings, error) {
	return domain.GlobalSettings{SubmissionDailyLimit: 5, CommentLimit: 500}, nil
}
func (s *stubStore) ViewerRating(context.Context, []string, *time.Time, bool, string, int) (domain.ViewerRating, error) {
	s.ratingCalls++
	return s.rating, nil
}
func (s *stubStore) CreateSubmission(_ context.Context, input store.CreateSubmissionInput) (domain.Video, error) {
	s.createdInput = &input
	return domain.Video{ID: "idea-id", Title: input.Title, ContentKind: input.ContentKind, SourceType: input.SourceType}, nil
}
func (s *stubStore) UpdateSubmissionContent(_ context.Context, input store.UpdateSubmissionContentInput) (domain.Video, error) {
	s.updatedInput = &input
	return domain.Video{ID: input.SubmissionID, Title: input.Title, SourceURL: input.SourceURL, SubmitterComment: input.Comment, Version: input.Version + 1}, nil
}
func (s *stubStore) CreateUnbanAppeal(_ context.Context, input store.CreateUnbanAppealInput) (domain.UnbanAppeal, error) {
	s.unbanInput = &input
	return domain.UnbanAppeal{ID: "appeal-id", Platform: input.Platform, Community: input.Community, BannedUsername: input.BannedUsername, Status: domain.UnbanPending}, nil
}

type stubYouTube struct{}

func (stubYouTube) Fetch(context.Context, string) (youtube.Metadata, error) {
	return youtube.Metadata{}, nil
}

func testServer(database store.Store) http.Handler {
	cfg := config.Config{
		BaseURL:           "http://localhost:8080",
		FrontendURL:       "http://localhost:5173",
		SessionCookieName: "test_session",
		SessionTTL:        time.Hour,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(cfg, database, stubYouTube{}, logger).Handler()
}

func TestPublicHealthAndCategories(t *testing.T) {
	t.Parallel()
	handler := testServer(&stubStore{categories: []domain.Category{{ID: "1", Slug: "funny", Name: "Смешное"}}})

	for _, path := range []string{"/health/live", "/health/ready", "/api/categories", "/api/news", "/api/rating"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s returned %d: %s", path, response.Code, response.Body.String())
		}
	}
}

func TestViewerRatingUsesServerCache(t *testing.T) {
	database := &stubStore{rating: domain.ViewerRating{ParticipantCount: 12}}
	handler := testServer(database)
	for range 2 {
		request := httptest.NewRequest(http.MethodGet, "/api/rating?channel=all&period=1y", nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("rating returned %d: %s", response.Code, response.Body.String())
		}
	}
	if database.ratingCalls != 1 {
		t.Fatalf("rating calculated %d times, want 1", database.ratingCalls)
	}
}

type snapshotStub struct {
	stubStore
	reads int
}

func TestStatusWriterRecordsActualResponse(t *testing.T) {
	for _, status := range []int{200, 400, 403, 500} {
		recorder := httptest.NewRecorder()
		writer := &statusWriter{ResponseWriter: recorder}
		writer.WriteHeader(status)
		writer.WriteHeader(201)
		_, _ = writer.Write([]byte("test"))
		if writer.status != status || recorder.Code != status {
			t.Fatalf("status=%d recorded=%d", status, writer.status)
		}
	}
}

func (s *snapshotStub) RunRatingSnapshots(context.Context, *slog.Logger) {}
func (s *snapshotStub) ReadRatingSnapshot(_ context.Context, channel, period, user string) (domain.ViewerRating, error) {
	s.reads++
	return domain.ViewerRating{Channel: channel, Period: period, Preparing: true, Items: []domain.ViewerRatingEntry{}}, nil
}
func TestColdSnapshotDoesNotCalculateHistory(t *testing.T) {
	database := &snapshotStub{}
	handler := testServer(database)
	for range 2 {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", "/api/rating?channel=ravshann&period=1y", nil))
		if response.Code != 200 || !strings.Contains(response.Body.String(), `"preparing":true`) {
			t.Fatal(response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("missing private cache policy")
		}
	}
	if database.ratingCalls != 0 || database.reads != 2 {
		t.Fatalf("history calculated: %+v", database)
	}
}

func TestOnlyOwnerCanCreateNewsPost(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		role domain.Role
		want int
	}{
		{role: domain.RoleUser, want: http.StatusForbidden},
		{role: domain.RoleModerator, want: http.StatusForbidden},
		{role: domain.RoleOwner, want: http.StatusCreated},
	} {
		handler := testServer(&stubStore{sessionRole: testCase.role})
		request := httptest.NewRequest(http.MethodPost, "/api/owner/news", strings.NewReader(`{"title":"Обновление","body":"Новая версия сайта"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-CSRF-Token", "csrf")
		request.AddCookie(&http.Cookie{Name: "test_session", Value: "session"})
		request.AddCookie(&http.Cookie{Name: "test_session_csrf", Value: "csrf"})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != testCase.want {
			t.Fatalf("role %s: got %d, want %d: %s", testCase.role, response.Code, testCase.want, response.Body.String())
		}
	}
}

func TestOwnerCanChooseNewsNotifications(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		body string
		want bool
	}{
		{body: `{"title":"Обновление","body":"Новая версия сайта"}`, want: true},
		{body: `{"title":"Обновление","body":"Новая версия сайта","notify_users":false}`, want: false},
	} {
		database := &newsBroadcasterStub{stubStore: stubStore{sessionRole: domain.RoleOwner}}
		handler := testServer(database)
		request := httptest.NewRequest(http.MethodPost, "/api/owner/news", strings.NewReader(testCase.body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-CSRF-Token", "csrf")
		request.AddCookie(&http.Cookie{Name: "test_session", Value: "session"})
		request.AddCookie(&http.Cookie{Name: "test_session_csrf", Value: "csrf"})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201: %s", response.Code, response.Body.String())
		}
		if len(database.notifyUsers) != 1 || database.notifyUsers[0] != testCase.want {
			t.Fatalf("notify_users = %#v, want %v", database.notifyUsers, testCase.want)
		}
	}
}

func TestOwnerEndpointRejectsModerator(t *testing.T) {
	t.Parallel()
	handler := testServer(&stubStore{sessionRole: domain.RoleModerator})
	request := httptest.NewRequest(http.MethodPost, "/api/owner/categories", strings.NewReader(`{"name":"Музыка","slug":"music"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", "csrf")
	request.AddCookie(&http.Cookie{Name: "test_session", Value: "session"})
	request.AddCookie(&http.Cookie{Name: "test_session_csrf", Value: "csrf"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403: %s", response.Code, response.Body.String())
	}
}

func TestOwnerCanCreateCategory(t *testing.T) {
	t.Parallel()
	handler := testServer(&stubStore{sessionRole: domain.RoleOwner})
	request := httptest.NewRequest(http.MethodPost, "/api/owner/categories", strings.NewReader(`{"name":"Музыка","slug":"music"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", "csrf")
	request.AddCookie(&http.Cookie{Name: "test_session", Value: "session"})
	request.AddCookie(&http.Cookie{Name: "test_session_csrf", Value: "csrf"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201: %s", response.Code, response.Body.String())
	}
}

func TestVoteRequiresAuthentication(t *testing.T) {
	t.Parallel()
	handler := testServer(&stubStore{})
	request := httptest.NewRequest(http.MethodPut, "/api/videos/video-id/vote", strings.NewReader(`{"value":1}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401: %s", response.Code, response.Body.String())
	}
}

func TestUserCanSubmitStreamIdeaWithoutVideoURL(t *testing.T) {
	t.Parallel()
	database := &stubStore{
		sessionRole: domain.RoleUser,
		categories:  []domain.Category{{ID: "ideas-id", Slug: "stream-ideas", Name: "Идеи для стрима"}},
	}
	handler := testServer(database)
	request := httptest.NewRequest(http.MethodPost, "/api/submissions", strings.NewReader(`{"content_kind":"stream_idea","source_type":"idea","title":"Турнир подписчиков","category_id":"ideas-id","comment":"Провести турнир во время эфира"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", "csrf")
	request.AddCookie(&http.Cookie{Name: "test_session", Value: "session"})
	request.AddCookie(&http.Cookie{Name: "test_session_csrf", Value: "csrf"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201: %s", response.Code, response.Body.String())
	}
	if database.createdInput == nil || database.createdInput.ContentKind != "stream_idea" || database.createdInput.SourceURL != "" {
		t.Fatalf("unexpected input: %#v", database.createdInput)
	}
}

func TestUserCanCreateManualUnbanAppeal(t *testing.T) {
	t.Parallel()
	database := &stubStore{sessionRole: domain.RoleUser}
	handler := testServer(database)
	request := httptest.NewRequest(http.MethodPost, "/api/unban-appeals", strings.NewReader(`{
		"platform":"twitch","community":"ravshann","banned_username":"viewer_name",
		"ban_reason":"Получил бан после конфликта в чате", "statement":"Признаю, что повёл себя неправильно, и впредь буду соблюдать правила.",
		"position":"admit","rules_accepted":true
	}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", "csrf")
	request.AddCookie(&http.Cookie{Name: "test_session", Value: "session"})
	request.AddCookie(&http.Cookie{Name: "test_session_csrf", Value: "csrf"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201: %s", response.Code, response.Body.String())
	}
	if database.unbanInput == nil || database.unbanInput.Community != "ravshann" || database.unbanInput.BannedUsername != "viewer_name" {
		t.Fatalf("unexpected input: %#v", database.unbanInput)
	}
}

func TestUnbanAppealRequiresRulesAcceptance(t *testing.T) {
	t.Parallel()
	database := &stubStore{sessionRole: domain.RoleUser}
	handler := testServer(database)
	request := httptest.NewRequest(http.MethodPost, "/api/unban-appeals", strings.NewReader(`{
		"platform":"telegram","community":"ravshann_telegram","banned_username":"viewer_name",
		"ban_reason":"Получил бан после конфликта в чате", "statement":"Считаю, что решение можно пересмотреть после моего объяснения.",
		"position":"unsure","rules_accepted":false
	}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", "csrf")
	request.AddCookie(&http.Cookie{Name: "test_session", Value: "session"})
	request.AddCookie(&http.Cookie{Name: "test_session_csrf", Value: "csrf"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400: %s", response.Code, response.Body.String())
	}
}

func TestUserCanSubmitShortAndExternalLinks(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name       string
		sourceType string
		url        string
	}{
		{name: "short video", sourceType: "short_video", url: "https://www.instagram.com/reel/example/"},
		{name: "external", sourceType: "external", url: "https://kappa.lol/example"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			database := &stubStore{sessionRole: domain.RoleUser, categories: []domain.Category{{ID: "category-id", Slug: "funny", Name: "Смешное"}}}
			handler := testServer(database)
			body := fmt.Sprintf(`{"content_kind":"video","source_type":%q,"url":%q,"title":"Тестовое видео","category_id":"category-id"}`, testCase.sourceType, testCase.url)
			request := httptest.NewRequest(http.MethodPost, "/api/submissions", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-CSRF-Token", "csrf")
			request.AddCookie(&http.Cookie{Name: "test_session", Value: "session"})
			request.AddCookie(&http.Cookie{Name: "test_session_csrf", Value: "csrf"})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusCreated {
				t.Fatalf("got %d, want 201: %s", response.Code, response.Body.String())
			}
			if database.createdInput == nil || database.createdInput.SourceType != testCase.sourceType || database.createdInput.SourceURL != testCase.url {
				t.Fatalf("unexpected input: %#v", database.createdInput)
			}
		})
	}
}

func TestModeratorCanEditSubmissionContent(t *testing.T) {
	t.Parallel()
	database := &stubStore{sessionRole: domain.RoleModerator}
	handler := testServer(database)
	request := httptest.NewRequest(http.MethodPatch, "/api/moderation/submissions/video-id/content", strings.NewReader(`{"title":"Исправленное название","source_url":"https://kappa.lol/fixed","comment":"Уточнённое описание","version":2}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", "csrf")
	request.AddCookie(&http.Cookie{Name: "test_session", Value: "session"})
	request.AddCookie(&http.Cookie{Name: "test_session_csrf", Value: "csrf"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", response.Code, response.Body.String())
	}
	if database.updatedInput == nil || database.updatedInput.Title != "Исправленное название" || database.updatedInput.Version != 2 {
		t.Fatalf("unexpected input: %#v", database.updatedInput)
	}
}

func TestClipsFromLastCompletedStreamsUsesLatestVODPerChannel(t *testing.T) {
	t.Parallel()
	clips := []twitch.Clip{
		{ID: "ravshan-old", BroadcasterName: "RavshanN", VideoID: "vod-1", CreatedAt: time.Date(2026, 8, 7, 18, 0, 0, 0, time.UTC)},
		{ID: "ravshan-last", BroadcasterName: "RavshanN", VideoID: "vod-2", CreatedAt: time.Date(2026, 8, 8, 18, 0, 0, 0, time.UTC)},
		{ID: "ravshan-live", BroadcasterName: "RavshanN", VideoID: "", CreatedAt: time.Date(2026, 8, 9, 18, 0, 0, 0, time.UTC)},
		{ID: "btw-a", BroadcasterName: "ravshanbtw", VideoID: "vod-btw", CreatedAt: time.Date(2026, 8, 8, 20, 0, 0, 0, time.UTC)},
		{ID: "btw-b", BroadcasterName: "ravshanbtw", VideoID: "vod-btw", CreatedAt: time.Date(2026, 8, 8, 20, 5, 0, 0, time.UTC)},
	}
	result := clipsFromLastCompletedStreams(clips)
	if len(result) != 3 || result[0].ID != "ravshan-last" || result[1].ID != "btw-a" || result[2].ID != "btw-b" {
		t.Fatalf("unexpected clips: %#v", result)
	}
}

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
	"github.com/ravshann/predlozhka/backend/internal/youtube"
)

type stubStore struct {
	store.Store
	categories   []domain.Category
	news         []domain.NewsPost
	sessionRole  domain.Role
	createdInput *store.CreateSubmissionInput
	updatedInput *store.UpdateSubmissionContentInput
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
func (s *stubStore) GetSettings(context.Context) (domain.GlobalSettings, error) {
	return domain.GlobalSettings{SubmissionDailyLimit: 5, CommentLimit: 500}, nil
}
func (s *stubStore) CreateSubmission(_ context.Context, input store.CreateSubmissionInput) (domain.Video, error) {
	s.createdInput = &input
	return domain.Video{ID: "idea-id", Title: input.Title, ContentKind: input.ContentKind, SourceType: input.SourceType}, nil
}
func (s *stubStore) UpdateSubmissionContent(_ context.Context, input store.UpdateSubmissionContentInput) (domain.Video, error) {
	s.updatedInput = &input
	return domain.Video{ID: input.SubmissionID, Title: input.Title, SourceURL: input.SourceURL, SubmitterComment: input.Comment, Version: input.Version + 1}, nil
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

	for _, path := range []string{"/health/live", "/health/ready", "/api/categories", "/api/news"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s returned %d: %s", path, response.Code, response.Body.String())
		}
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

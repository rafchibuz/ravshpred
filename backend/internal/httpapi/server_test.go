package httpapi

import (
	"context"
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
	categories []domain.Category
}

func (s *stubStore) Ping(context.Context) error { return nil }
func (s *stubStore) Close()                     {}
func (s *stubStore) ListCategories(context.Context) ([]domain.Category, error) {
	return s.categories, nil
}
func (s *stubStore) EnsureDevUser(_ context.Context, role domain.Role) (domain.User, error) {
	return domain.User{ID: "00000000-0000-0000-0000-000000000001", Display: "Dev", Role: role}, nil
}
func (s *stubStore) CreateCategory(_ context.Context, slug, name, _ string) (domain.Category, error) {
	return domain.Category{ID: "category-id", Slug: slug, Name: name}, nil
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
		AllowDevAuth:      true,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(cfg, database, stubYouTube{}, logger).Handler()
}

func TestPublicHealthAndCategories(t *testing.T) {
	t.Parallel()
	handler := testServer(&stubStore{categories: []domain.Category{{ID: "1", Slug: "funny", Name: "Смешное"}}})

	for _, path := range []string{"/health/live", "/health/ready", "/api/categories"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s returned %d: %s", path, response.Code, response.Body.String())
		}
	}
}

func TestOwnerEndpointRejectsModerator(t *testing.T) {
	t.Parallel()
	handler := testServer(&stubStore{})
	request := httptest.NewRequest(http.MethodPost, "/api/owner/categories", strings.NewReader(`{"name":"Музыка","slug":"music"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Dev-Role", "moderator")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403: %s", response.Code, response.Body.String())
	}
}

func TestOwnerCanCreateCategory(t *testing.T) {
	t.Parallel()
	handler := testServer(&stubStore{})
	request := httptest.NewRequest(http.MethodPost, "/api/owner/categories", strings.NewReader(`{"name":"Музыка","slug":"music"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Dev-Role", "owner")
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

package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRatingSinceThreeDays(t *testing.T) {
	now := time.Date(2026, time.October, 27, 12, 0, 0, 0, time.UTC)
	got := ratingSince("3d", now)
	if got == nil || !got.Equal(now.AddDate(0, 0, -3)) {
		t.Fatalf("ratingSince(3d) = %v", got)
	}
}

func TestViewerRatingAcceptsPublicPeriods(t *testing.T) {
	handler := testServer(&stubStore{})
	for _, period := range []string{"3d", "7d", "30d", "90d", "1y", "all"} {
		t.Run(period, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/rating?period="+period, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("period %s returned %d: %s", period, response.Code, response.Body.String())
			}
		})
	}
}

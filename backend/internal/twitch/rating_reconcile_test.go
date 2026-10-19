package twitch

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type ratingRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn ratingRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type ratingCloseCall struct {
	channel  string
	streamID string
}

type ratingReconcileStore struct {
	mu     sync.Mutex
	closed []ratingCloseCall
}

func (*ratingReconcileStore) RatingCredentials(context.Context) (RatingCredentials, error) {
	return RatingCredentials{}, nil
}
func (*ratingReconcileStore) SaveRatingCredentials(context.Context, RatingCredentials) error {
	return nil
}
func (*ratingReconcileStore) UpsertRatingStream(context.Context, RatingStream) error { return nil }
func (s *ratingReconcileStore) CloseRatingStreams(_ context.Context, channel, streamID string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = append(s.closed, ratingCloseCall{channel: channel, streamID: streamID})
	return nil
}
func (*ratingReconcileStore) InsertRatingMessage(context.Context, RatingMessage) error { return nil }
func (*ratingReconcileStore) SetRatingCollectorStatus(context.Context, []string, string, string, string, *time.Time) error {
	return nil
}

func TestReconcileClosesDatabaseStreamsAfterCollectorRestart(t *testing.T) {
	store := &ratingReconcileStore{}
	collector := NewRatingCollector("client", "secret", "", "", store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	collector.credentials = RatingCredentials{AccessToken: "token"}
	collector.channelIDs = map[string]string{"ravshann": "1", "ravshanbtw": "2"}
	collector.http = &http.Client{Transport: ratingRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
			Request:    request,
		}, nil
	})}

	if err := collector.reconcileStreams(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.closed) != 2 {
		t.Fatalf("expected both channels to be reconciled, got %+v", store.closed)
	}
	for _, call := range store.closed {
		if call.streamID != "" {
			t.Fatalf("stale database streams must be closed without an in-memory id: %+v", call)
		}
	}
}

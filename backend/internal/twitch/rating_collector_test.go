package twitch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestRatingReaderRespondsToPingWhileConsumerIsBusy(t *testing.T) {
	pingResult := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			pingResult <- err
			return
		}
		defer conn.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		// Server must read to receive pong as well.
		conn.CloseRead(ctx)
		for i := 0; i < 20; i++ {
			var e eventSubEnvelope
			e.Metadata.MessageType = "notification"
			if err := wsjson.Write(ctx, conn, e); err != nil {
				pingResult <- err
				return
			}
		}
		pingResult <- conn.Ping(ctx)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	events, failures := readRatingEvents(ctx, conn)
	// Deliberately do not consume events until ping completes (simulates slow SQL).
	select {
	case err := <-pingResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("reader blocked behind consumer")
	}
	count := 0
	for range events {
		count++
	}
	<-failures
	if count != 20 {
		t.Fatalf("events lost: %d", count)
	}
}

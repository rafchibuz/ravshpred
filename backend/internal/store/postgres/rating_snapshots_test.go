package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ravshann/predlozhka/backend/internal/domain"
)

func TestRatingSnapshotsPersonalPlaceAndRollback(t *testing.T) {
	s := testDatabase(t)
	ctx := context.Background()
	initial, err := s.ReadRatingSnapshot(ctx, "all", "30d", "")
	if err != nil || !initial.Preparing {
		t.Fatalf("cold read: %+v %v", initial, err)
	}
	value := domain.ViewerRating{Channel: "all", Period: "30d", GeneratedAt: time.Now().Add(-time.Hour), ParticipantCount: 125}
	for i := 1; i <= 125; i++ {
		value.Items = append(value.Items, domain.ViewerRatingEntry{Rank: i, TwitchID: fmt.Sprint(i), Score: 50})
	}
	if err := s.publishRatingSnapshot(ctx, "all:30d", value); err != nil {
		t.Fatal(err)
	}
	// A new Store instance reads persisted results; no in-memory warmup is needed.
	restarted := &Store{pool: s.pool}
	got, err := restarted.ReadRatingSnapshot(ctx, "all", "30d", "125")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 100 || got.Me == nil || got.Me.Rank != 125 || !got.Stale {
		t.Fatalf("invalid snapshot: %+v", got)
	}
	guest, err := s.ReadRatingSnapshot(ctx, "all", "30d", "")
	if err != nil || guest.Me != nil {
		t.Fatalf("personal data leaked: %+v %v", guest.Me, err)
	}
	// Duplicate IDs fail COPY: previously published header and entries must survive.
	value.Items = append(value.Items, value.Items[0])
	value.ParticipantCount = 999
	if err := s.publishRatingSnapshot(ctx, "all:30d", value); err == nil {
		t.Fatal("expected duplicate failure")
	}
	got, err = s.ReadRatingSnapshot(ctx, "all", "30d", "125")
	if err != nil || got.ParticipantCount != 125 || got.Me == nil {
		t.Fatalf("partial publication: %+v %v", got, err)
	}
}

func TestSnapshotPeriods(t *testing.T) {
	now := time.Now()
	for period, days := range map[string]int{"1d": 1, "7d": 7, "30d": 30, "90d": 90, "1y": 365} {
		got := snapshotSince(period, now)
		if got == nil || now.Sub(*got) != time.Duration(days)*24*time.Hour {
			t.Fatalf("wrong period %s", period)
		}
	}
	if snapshotSince("last_stream", now) != nil || snapshotSince("all", now) != nil {
		t.Fatal("unexpected cutoff")
	}
	leap := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	if !snapshotSince("1y", leap).Equal(leap.AddDate(-1, 0, 0)) {
		t.Fatal("year must preserve calendar semantics")
	}
}

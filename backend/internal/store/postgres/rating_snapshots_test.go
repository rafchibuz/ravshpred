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
	if len(got.Items) != 100 || got.Me == nil || got.Me.Rank != 125 || got.Stale {
		t.Fatalf("invalid snapshot: %+v", got)
	}
	// Snapshot age alone is harmless while the source is idle. A later stream
	// event makes the same saved result stale until the worker refreshes it.
	if _, err := s.pool.Exec(ctx, `INSERT INTO twitch_rating_streams(id,channel_login,started_at,observed_live)
		VALUES('newer-stream','ravshann',now(),true)`); err != nil {
		t.Fatal(err)
	}
	got, err = s.ReadRatingSnapshot(ctx, "all", "30d", "125")
	if err != nil || !got.Stale {
		t.Fatalf("source update must mark snapshot stale: %+v %v", got, err)
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

func TestPendingRatingSnapshotJobsSkipIdleAndTargetChangedChannel(t *testing.T) {
	s := testDatabase(t)
	ctx := context.Background()
	for _, period := range ratingSnapshotPeriods {
		for _, channel := range ratingSnapshotChannels {
			value := domain.ViewerRating{Channel: channel, Period: period, GeneratedAt: time.Now()}
			if err := s.publishRatingSnapshot(ctx, channel+":"+period, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	jobs, err := s.pendingRatingSnapshotJobs(ctx)
	if err != nil || len(jobs) != 0 {
		t.Fatalf("idle snapshots must be reused: %+v %v", jobs, err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE twitch_rating_state
		SET last_event_at=now()+interval '1 minute' WHERE channel_login='ravshann'`); err != nil {
		t.Fatal(err)
	}
	jobs, err = s.pendingRatingSnapshotJobs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != len(ratingSnapshotPeriods)*2 {
		t.Fatalf("ravshann change must refresh its own and combined snapshots only: %+v", jobs)
	}
	for _, job := range jobs {
		if job.channel == "ravshanbtw" {
			t.Fatalf("unchanged channel scheduled for refresh: %+v", job)
		}
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

package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestViewerRatingUsesLiveStreamAndCapsBurstMessages(t *testing.T) {
	s := testDatabase(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO twitch_rating_streams(id,channel_login,started_at,ended_at,observed_live) VALUES
		('completed','ravshann',$1,$2,true),
		('live','ravshann',$3,NULL,true)`, now.Add(-48*time.Hour), now.Add(-46*time.Hour), now.Add(-2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	insert := func(user, stream string, at time.Time, reply bool) {
		t.Helper()
		id := fmt.Sprintf("%s-%s-%d", user, stream, at.UnixNano())
		_, insertErr := s.pool.Exec(ctx, `
			INSERT INTO twitch_rating_messages(id,channel_login,user_id,login,display_name,sent_at,stream_id,is_reply,text_length)
			VALUES($1,'ravshann',$2,$2,$2,$3,$4,$5,10)`, id, user, at, stream, reply)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
	}
	insert("old-viewer", "completed", now.Add(-47*time.Hour), false)
	insert("old-viewer", "completed", now.Add(-47*time.Hour+6*time.Minute), false)
	insert("old-viewer", "completed", now.Add(-47*time.Hour+12*time.Minute), false)
	bucketBase := now.Truncate(5 * time.Minute).Add(-110*time.Minute + 10*time.Second)
	for i := 0; i < 8; i++ {
		insert("distributed", "live", bucketBase.Add(time.Duration(i)*5*time.Minute), false)
	}
	burstBase := now.Truncate(5 * time.Minute).Add(-100*time.Minute + 10*time.Second)
	for i := 0; i < 30; i++ {
		insert("burst", "live", burstBase.Add(time.Duration(i)*time.Second), false)
	}
	insert("burst", "live", burstBase.Add(6*time.Minute), false)

	rating, err := s.ViewerRating(ctx, []string{"ravshann"}, nil, true, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if rating.StreamCount != 1 || len(rating.Items) != 2 {
		t.Fatalf("live stream selection failed: %+v", rating)
	}
	if rating.Items[0].Login != "distributed" || rating.Items[0].CountedMessages != 8 {
		t.Fatalf("distributed activity should lead: %+v", rating.Items)
	}
	if rating.Items[1].Login != "burst" || rating.Items[1].Messages != 31 || rating.Items[1].CountedMessages != 4 {
		t.Fatalf("burst cap failed: %+v", rating.Items[1])
	}
	if rating.Items[0].Score <= rating.Items[1].Score {
		t.Fatalf("distributed score %d must exceed burst score %d", rating.Items[0].Score, rating.Items[1].Score)
	}
}

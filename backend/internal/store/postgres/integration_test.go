package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravshann/predlozhka/backend/internal/domain"
	"github.com/ravshann/predlozhka/backend/internal/store"
)

// TEST_DATABASE_URL must point to a disposable LOCAL test database.
// Each test owns a separate schema; application tables are never touched.
func testDatabase(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); admin.Close() })
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	base, err := os.ReadFile("../../../migrations/000001_init.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{string(base), newsSchema, portalSchema, moviesSchema, submissionKindsSchema, twitchCacheSchema, unbanAppealsSchema, viewerRatingSchema, ratingSnapshotsSchema, ratingAvatarsSchema, newsNotificationsSchema, ravshTOKSchema} {
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	return &Store{pool: pool}
}

func TestLoginAuditAndCategoryAtomicity(t *testing.T) {
	s := testDatabase(t)
	ctx := context.Background()
	u, err := s.UpsertTwitchUser(ctx, "test-login", "tester", "Tester", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.UpsertTwitchUser(ctx, "test-login", "tester", "Renamed", ""); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = s.pool.QueryRow(ctx, "SELECT count(*) FROM audit_log WHERE action='login' AND target_id=$1", u.ID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("login audit count=%d error=%v", count, err)
	}
	var first, second, submission string
	if err = s.pool.QueryRow(ctx, "SELECT id::text FROM categories WHERE slug='uncategorized'").Scan(&first); err != nil {
		t.Fatal(err)
	}
	if err = s.pool.QueryRow(ctx, "SELECT id::text FROM categories WHERE slug='funny'").Scan(&second); err != nil {
		t.Fatal(err)
	}
	if err = s.pool.QueryRow(ctx, `INSERT INTO submissions(youtube_id,youtube_url,title,channel_title,thumbnail_url,author_id,category_id) VALUES('abcdefghijk','https://www.youtube.com/watch?v=abcdefghijk','Test','','',$1,$2) RETURNING id::text`, u.ID, first).Scan(&submission); err != nil {
		t.Fatal(err)
	}
	if err = s.UpdateVideoCategory(ctx, submission, second, u.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.pool.QueryRow(ctx, "SELECT count(*) FROM audit_log WHERE action='category_change' AND metadata->>'category_id'=$1", second).Scan(&count); err != nil || count != 1 {
		t.Fatalf("category audit count=%d error=%v", count, err)
	}
	// A missing actor makes the audit FK fail: the category/version must roll back.
	if err = s.UpdateVideoCategory(ctx, submission, first, "00000000-0000-0000-0000-000000000000"); err == nil {
		t.Fatal("expected audit failure")
	}
	var actual string
	var version int
	if err = s.pool.QueryRow(ctx, "SELECT category_id::text,version FROM submissions WHERE id::text=$1", submission).Scan(&actual, &version); err != nil {
		t.Fatal(err)
	}
	if actual != second || version != 2 {
		t.Fatalf("partial update: category=%s version=%d", actual, version)
	}
}

func TestRavshTOKLifecycle(t *testing.T) {
	s := testDatabase(t)
	ctx := context.Background()
	author, err := s.UpsertTwitchUser(ctx, "ravshtok-author", "ravshtok_author", "RavshTOK Author", "")
	if err != nil {
		t.Fatal(err)
	}
	viewer, err := s.UpsertTwitchUser(ctx, "ravshtok-viewer", "ravshtok_viewer", "RavshTOK Viewer", "")
	if err != nil {
		t.Fatal(err)
	}
	var categoryID string
	if err := s.pool.QueryRow(ctx, "SELECT id::text FROM categories WHERE slug='funny'").Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	video, err := s.CreateSubmission(ctx, store.CreateSubmissionInput{
		ContentKind: "video", SourceType: "short_video", SourceURL: "https://www.tiktok.com/@ravshann/video/123",
		Title: "RavshTOK test", ChannelTitle: "TikTok / Reels", AuthorID: author.ID, CategoryID: categoryID,
		DailyLimit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	mine, err := s.ListMine(ctx, author.ID, 10)
	if err != nil || len(mine) != 1 || mine[0].RavshTOKStatus != "pending" {
		t.Fatalf("unexpected initial media state: %+v error=%v", mine, err)
	}
	if _, err := s.CreateSubmission(ctx, store.CreateSubmissionInput{
		ContentKind: "video", SourceType: "external", SourceURL: "https://example.com/video",
		Title: "Regular video", ChannelTitle: "example.com", AuthorID: author.ID, CategoryID: categoryID,
		DailyLimit: 1,
	}); err != nil {
		t.Fatalf("regular submission must use a separate daily limit: %v", err)
	}
	if _, err := s.CreateSubmission(ctx, store.CreateSubmissionInput{
		ContentKind: "video", SourceType: "short_video", SourceURL: "https://www.tiktok.com/@ravshann/video/456",
		Title: "Second RavshTOK", ChannelTitle: "TikTok / Reels", AuthorID: author.ID, CategoryID: categoryID,
		DailyLimit: 1,
	}); !errors.Is(err, store.ErrDailyLimit) {
		t.Fatalf("second RavshTOK must hit its own limit, got %v", err)
	}
	if _, err := s.pool.Exec(ctx, "UPDATE submissions SET status='approved' WHERE id::text=$1", video.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, "UPDATE ravshtok_media SET status='ready',media_path='ravshtok/test.mp4',poster_path='ravshtok/test.webp' WHERE submission_id::text=$1", video.ID); err != nil {
		t.Fatal(err)
	}
	ordinaryFeed, _, err := s.ListFeed(ctx, store.FeedParams{UserID: viewer.ID, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range ordinaryFeed {
		if item.ID == video.ID {
			t.Fatal("RavshTOK item leaked into the ordinary proposal feed")
		}
	}

	feed, err := s.ListRavshTOK(ctx, store.RavshTOKParams{UserID: viewer.ID, Role: domain.RoleOwner, Mode: "new", Platform: "all", Sort: "new", Limit: 12})
	if err != nil || len(feed.Items) != 1 || feed.Items[0].PlaybackURL != "/media/ravshtok/test.mp4" {
		t.Fatalf("unexpected new feed: %+v error=%v", feed, err)
	}
	if err := s.MarkRavshTOKViewed(ctx, video.ID, viewer.ID, domain.RoleOwner); err != nil {
		t.Fatal(err)
	}
	feed, err = s.ListRavshTOK(ctx, store.RavshTOKParams{UserID: viewer.ID, Role: domain.RoleOwner, Mode: "watched", Platform: "all", Sort: "new", Limit: 12})
	if err != nil || len(feed.Items) != 1 || !feed.Items[0].UserViewed || !feed.Items[0].StreamerWatched {
		t.Fatalf("unexpected watched feed: %+v error=%v", feed, err)
	}
}

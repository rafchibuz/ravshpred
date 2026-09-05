package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
	for _, sql := range []string{string(base), newsSchema, portalSchema, moviesSchema, submissionKindsSchema, twitchCacheSchema, unbanAppealsSchema, viewerRatingSchema} {
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

package store

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func newGraphStoreTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if strings.TrimSpace(dsn) == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if strings.TrimSpace(dsn) == "" {
		t.Skip("skipping graph store integration tests: TEST_DATABASE_URL or DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("create pgx pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping postgres: %v", err)
	}

	if err := ensureGraphTestSchema(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("ensure graph test schema: %v", err)
	}

	t.Cleanup(pool.Close)
	return pool
}

func ensureGraphTestSchema(ctx context.Context, pool *pgxpool.Pool) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS works (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			normalized_title TEXT,
			fingerprint TEXT,
			first_pub_year INTEGER,
			series_name TEXT,
			series_index DOUBLE PRECISION,
			subjects TEXT[],
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`ALTER TABLE works ADD COLUMN IF NOT EXISTS series_name TEXT`,
		`ALTER TABLE works ADD COLUMN IF NOT EXISTS series_index DOUBLE PRECISION`,
		`ALTER TABLE works ADD COLUMN IF NOT EXISTS subjects TEXT[]`,
		`CREATE TABLE IF NOT EXISTS series (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			normalized_name TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_series_normalized ON series(normalized_name)`,
		`CREATE TABLE IF NOT EXISTS series_entries (
			series_id TEXT NOT NULL REFERENCES series(id) ON DELETE CASCADE,
			work_id TEXT NOT NULL REFERENCES works(id) ON DELETE CASCADE,
			series_index DOUBLE PRECISION NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			PRIMARY KEY(series_id, work_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_series_entries_series ON series_entries(series_id, series_index)`,
		`CREATE INDEX IF NOT EXISTS idx_series_entries_work ON series_entries(work_id)`,
		`CREATE TABLE IF NOT EXISTS subjects (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			normalized_name TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_subjects_normalized ON subjects(normalized_name)`,
		`CREATE TABLE IF NOT EXISTS work_subjects (
			work_id TEXT NOT NULL REFERENCES works(id) ON DELETE CASCADE,
			subject_id TEXT NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			PRIMARY KEY(work_id, subject_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_work_subjects_subject ON work_subjects(subject_id)`,
		`CREATE TABLE IF NOT EXISTS work_relationships (
			source_work_id TEXT NOT NULL REFERENCES works(id) ON DELETE CASCADE,
			target_work_id TEXT NOT NULL REFERENCES works(id) ON DELETE CASCADE,
			relationship_type TEXT NOT NULL,
			confidence DOUBLE PRECISION NOT NULL DEFAULT 0.5,
			provider TEXT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			PRIMARY KEY(source_work_id, target_work_id, relationship_type)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_workrels_source ON work_relationships(source_work_id)`,
		`CREATE INDEX IF NOT EXISTS idx_workrels_target ON work_relationships(target_work_id)`,
	}

	for _, stmt := range statements {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func seedWork(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id string, title string) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO works (id, title, normalized_title, fingerprint, first_pub_year)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO NOTHING`,
		id, title, strings.ToLower(title), id+":fp", 2024,
	)
	if err != nil {
		t.Fatalf("seed work %s: %v", id, err)
	}
}

func cleanupGraphStoreArtifacts(t *testing.T, ctx context.Context, pool *pgxpool.Pool, prefix string) {
	t.Helper()
	likePrefix := prefix + "%"
	statements := []struct {
		query string
		args  []any
	}{
		{`DELETE FROM work_relationships WHERE source_work_id LIKE $1 OR target_work_id LIKE $1`, []any{likePrefix}},
		{`DELETE FROM work_subjects WHERE work_id LIKE $1 OR subject_id LIKE $1`, []any{likePrefix}},
		{`DELETE FROM series_entries WHERE work_id LIKE $1 OR series_id LIKE $1`, []any{likePrefix}},
		{`DELETE FROM subjects WHERE id LIKE $1`, []any{likePrefix}},
		{`DELETE FROM series WHERE id LIKE $1`, []any{likePrefix}},
		{`DELETE FROM works WHERE id LIKE $1`, []any{likePrefix}},
	}
	for _, stmt := range statements {
		if _, err := pool.Exec(ctx, stmt.query, stmt.args...); err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	}
}

func TestSeriesStore_UpsertAndEntryIdempotency(t *testing.T) {
	pool := newGraphStoreTestPool(t)
	ctx := context.Background()
	store := NewSeriesStore(pool)

	prefix := fmt.Sprintf("test-store-series-%d:", time.Now().UnixNano())
	defer cleanupGraphStoreArtifacts(t, ctx, pool, prefix)

	workID := prefix + "w1"
	seedWork(t, ctx, pool, workID, "Series Work")

	beforeCount, err := store.CountSeries(ctx)
	if err != nil {
		t.Fatalf("count series before: %v", err)
	}

	normalized := prefix + "trilogy"
	seriesID, err := store.UpsertSeries(ctx, "Trilogy One", normalized)
	if err != nil {
		t.Fatalf("upsert series first: %v", err)
	}
	seriesID2, err := store.UpsertSeries(ctx, "Trilogy One Updated", normalized)
	if err != nil {
		t.Fatalf("upsert series second: %v", err)
	}
	if seriesID != seriesID2 {
		t.Fatalf("expected same series id on idempotent upsert, got %q and %q", seriesID, seriesID2)
	}

	idx1 := 1.0
	if err := store.UpsertSeriesEntry(ctx, seriesID, workID, &idx1); err != nil {
		t.Fatalf("upsert series entry first: %v", err)
	}
	idx2 := 2.0
	if err := store.UpsertSeriesEntry(ctx, seriesID, workID, &idx2); err != nil {
		t.Fatalf("upsert series entry second: %v", err)
	}

	entries, err := store.GetSeriesEntries(ctx, seriesID)
	if err != nil {
		t.Fatalf("get series entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one series entry after idempotent upsert, got %d", len(entries))
	}
	if entries[0].SeriesIndex == nil || *entries[0].SeriesIndex != 2.0 {
		t.Fatalf("expected series index to be updated to 2.0, got %+v", entries[0].SeriesIndex)
	}

	forWork, err := store.GetSeriesForWork(ctx, workID)
	if err != nil {
		t.Fatalf("get series for work: %v", err)
	}
	if forWork.ID != seriesID {
		t.Fatalf("expected series id %q for work, got %q", seriesID, forWork.ID)
	}

	afterCount, err := store.CountSeries(ctx)
	if err != nil {
		t.Fatalf("count series after: %v", err)
	}
	if afterCount != beforeCount+1 {
		t.Fatalf("expected series count to increase by 1, before=%d after=%d", beforeCount, afterCount)
	}
}

func TestSubjectStore_SetWorkSubjectsReplaceSemantics(t *testing.T) {
	pool := newGraphStoreTestPool(t)
	ctx := context.Background()
	store := NewSubjectStore(pool)

	prefix := fmt.Sprintf("test-store-subject-%d:", time.Now().UnixNano())
	defer cleanupGraphStoreArtifacts(t, ctx, pool, prefix)

	workID := prefix + "w1"
	seedWork(t, ctx, pool, workID, "Subject Work")

	beforeCount, err := store.CountSubjects(ctx)
	if err != nil {
		t.Fatalf("count subjects before: %v", err)
	}

	s1, err := store.UpsertSubject(ctx, "Fantasy", prefix+"fantasy")
	if err != nil {
		t.Fatalf("upsert subject 1: %v", err)
	}
	s2, err := store.UpsertSubject(ctx, "Magic", prefix+"magic")
	if err != nil {
		t.Fatalf("upsert subject 2: %v", err)
	}
	s3, err := store.UpsertSubject(ctx, "Adventure", prefix+"adventure")
	if err != nil {
		t.Fatalf("upsert subject 3: %v", err)
	}

	if err := store.SetWorkSubjects(ctx, workID, []string{s1, s2, s2}); err != nil {
		t.Fatalf("set initial work subjects: %v", err)
	}
	if err := store.SetWorkSubjects(ctx, workID, []string{s2, s3}); err != nil {
		t.Fatalf("set replacement work subjects: %v", err)
	}

	subjects, err := store.GetSubjectsForWork(ctx, workID)
	if err != nil {
		t.Fatalf("get subjects for work: %v", err)
	}
	if len(subjects) != 2 {
		t.Fatalf("expected exactly 2 subjects after replacement, got %d", len(subjects))
	}
	gotIDs := []string{subjects[0].ID, subjects[1].ID}
	sort.Strings(gotIDs)
	wantIDs := []string{s2, s3}
	sort.Strings(wantIDs)
	if gotIDs[0] != wantIDs[0] || gotIDs[1] != wantIDs[1] {
		t.Fatalf("unexpected subject ids, got=%v want=%v", gotIDs, wantIDs)
	}

	worksForSubject, err := store.GetWorksForSubject(ctx, s2, 10, 0)
	if err != nil {
		t.Fatalf("get works for subject: %v", err)
	}
	if len(worksForSubject) != 1 || worksForSubject[0].ID != workID {
		t.Fatalf("expected work %q for subject %q, got %+v", workID, s2, worksForSubject)
	}

	afterCount, err := store.CountSubjects(ctx)
	if err != nil {
		t.Fatalf("count subjects after: %v", err)
	}
	if afterCount != beforeCount+3 {
		t.Fatalf("expected subject count to increase by 3, before=%d after=%d", beforeCount, afterCount)
	}
}

func TestWorkRelationshipStore_UpsertAndDeleteSemantics(t *testing.T) {
	pool := newGraphStoreTestPool(t)
	ctx := context.Background()
	store := NewWorkRelationshipStore(pool)

	prefix := fmt.Sprintf("test-store-rel-%d:", time.Now().UnixNano())
	defer cleanupGraphStoreArtifacts(t, ctx, pool, prefix)

	sourceID := prefix + "w1"
	targetID := prefix + "w2"
	otherID := prefix + "w3"
	seedWork(t, ctx, pool, sourceID, "Source")
	seedWork(t, ctx, pool, targetID, "Target")
	seedWork(t, ctx, pool, otherID, "Other")

	countsBefore, err := store.CountRelationshipsByType(ctx)
	if err != nil {
		t.Fatalf("count relationships before: %v", err)
	}

	provider := "derived"
	if err := store.UpsertRelationship(ctx, sourceID, targetID, "same_author", 0.7, nil); err != nil {
		t.Fatalf("upsert same_author first: %v", err)
	}
	if err := store.UpsertRelationship(ctx, sourceID, targetID, "same_author", 0.9, &provider); err != nil {
		t.Fatalf("upsert same_author second: %v", err)
	}
	if err := store.UpsertRelationship(ctx, sourceID, otherID, "related_subject", 0.5, nil); err != nil {
		t.Fatalf("upsert related_subject: %v", err)
	}

	sameAuthor := "same_author"
	rels, err := store.GetRelatedWorks(ctx, sourceID, &sameAuthor, 10)
	if err != nil {
		t.Fatalf("get related works by type: %v", err)
	}
	if len(rels) != 1 {
		t.Fatalf("expected one same_author relationship after upsert dedupe, got %d", len(rels))
	}
	if rels[0].Confidence != 0.9 {
		t.Fatalf("expected upsert to refresh confidence to 0.9, got %v", rels[0].Confidence)
	}
	if rels[0].Provider == nil || *rels[0].Provider != provider {
		t.Fatalf("expected provider to be updated to %q, got %+v", provider, rels[0].Provider)
	}

	allRels, err := store.GetRelatedWorks(ctx, sourceID, nil, 10)
	if err != nil {
		t.Fatalf("get all related works: %v", err)
	}
	if len(allRels) != 2 {
		t.Fatalf("expected two relationships before deletes, got %d", len(allRels))
	}

	if err := store.DeleteRelationshipsForWork(ctx, sourceID, &sameAuthor); err != nil {
		t.Fatalf("delete by type: %v", err)
	}
	remaining, err := store.GetRelatedWorks(ctx, sourceID, nil, 10)
	if err != nil {
		t.Fatalf("get related works after typed delete: %v", err)
	}
	if len(remaining) != 1 || remaining[0].RelationshipType != "related_subject" {
		t.Fatalf("expected only related_subject to remain, got %+v", remaining)
	}

	if err := store.DeleteRelationshipsForWork(ctx, sourceID, nil); err != nil {
		t.Fatalf("delete all relationships for source: %v", err)
	}
	empty, err := store.GetRelatedWorks(ctx, sourceID, nil, 10)
	if err != nil {
		t.Fatalf("get related works after full delete: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected zero relationships after full delete, got %d", len(empty))
	}

	countsAfter, err := store.CountRelationshipsByType(ctx)
	if err != nil {
		t.Fatalf("count relationships after: %v", err)
	}
	if countsAfter["same_author"] != countsBefore["same_author"] {
		t.Fatalf("expected same_author count to return to baseline, before=%d after=%d", countsBefore["same_author"], countsAfter["same_author"])
	}
	if countsAfter["related_subject"] != countsBefore["related_subject"] {
		t.Fatalf("expected related_subject count to return to baseline, before=%d after=%d", countsBefore["related_subject"], countsAfter["related_subject"])
	}
}

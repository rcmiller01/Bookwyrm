package quality

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	DetectSeriesAnomalies(ctx context.Context, limit int) ([]SeriesAnomaly, error)
	DetectPublicationYearConflicts(ctx context.Context, limit int) ([]PublicationYearConflict, error)
	DetectDuplicateEditions(ctx context.Context, limit int) ([]DuplicateEdition, error)
	ListIdentifierCandidates(ctx context.Context, limit int) ([]IdentifierCandidate, error)
	RepairSeriesOrder(ctx context.Context, seriesID string) (int, error)
	SyncWorkFirstPublicationYear(ctx context.Context, workID string) (bool, error)
	RemoveIdentifier(ctx context.Context, id IdentifierCandidate) (bool, error)
}

type pgRepository struct {
	db *pgxpool.Pool
}

func NewPGRepository(db *pgxpool.Pool) Repository {
	return &pgRepository{db: db}
}

func (r *pgRepository) DetectSeriesAnomalies(ctx context.Context, limit int) ([]SeriesAnomaly, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			s.id,
			s.name,
			COUNT(*)::int AS entry_count,
			COUNT(*) FILTER (WHERE se.series_index IS NULL)::int AS missing_count,
			COUNT(se.series_index)::int AS indexed_count,
			COUNT(DISTINCT se.series_index)::int AS distinct_index_count
		FROM series s
		JOIN series_entries se ON se.series_id = s.id
		GROUP BY s.id, s.name
		HAVING COUNT(*) > 1
		   AND (
				COUNT(*) FILTER (WHERE se.series_index IS NULL) > 0
				OR COUNT(DISTINCT se.series_index) < COUNT(se.series_index)
		   )
		ORDER BY s.name ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SeriesAnomaly, 0)
	for rows.Next() {
		var seriesID string
		var seriesName string
		var entryCount int
		var missingCount int
		var indexedCount int
		var distinctIndexCount int
		if err := rows.Scan(&seriesID, &seriesName, &entryCount, &missingCount, &indexedCount, &distinctIndexCount); err != nil {
			return nil, err
		}

		reasons := make([]string, 0, 2)
		duplicateSlot := distinctIndexCount < indexedCount
		if missingCount > 0 {
			reasons = append(reasons, "contains entries with missing series index")
		}
		if duplicateSlot {
			reasons = append(reasons, "contains duplicate series index values")
		}

		out = append(out, SeriesAnomaly{
			SeriesID:      seriesID,
			SeriesName:    seriesName,
			EntryCount:    entryCount,
			MissingIndex:  missingCount,
			DuplicateSlot: duplicateSlot,
			Reason:        strings.Join(reasons, "; "),
		})
	}
	return out, rows.Err()
}

func (r *pgRepository) DetectPublicationYearConflicts(ctx context.Context, limit int) ([]PublicationYearConflict, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			w.id,
			w.title,
			ARRAY_AGG(DISTINCT e.publication_year ORDER BY e.publication_year) AS years
		FROM works w
		JOIN editions e ON e.work_id = w.id
		WHERE e.publication_year > 0
		GROUP BY w.id, w.title
		HAVING COUNT(DISTINCT e.publication_year) > 1
		ORDER BY w.title ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]PublicationYearConflict, 0)
	for rows.Next() {
		var item PublicationYearConflict
		var years32 []int32
		if err := rows.Scan(&item.WorkID, &item.Title, &years32); err != nil {
			return nil, err
		}
		item.Years = make([]int, 0, len(years32))
		for _, year := range years32 {
			item.Years = append(item.Years, int(year))
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *pgRepository) DetectDuplicateEditions(ctx context.Context, limit int) ([]DuplicateEdition, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			w.id,
			w.title,
			LOWER(TRIM(COALESCE(e.title, ''))) AS norm_title,
			LOWER(TRIM(COALESCE(e.format, ''))) AS norm_format,
			LOWER(TRIM(COALESCE(e.publisher, ''))) AS norm_publisher,
			COALESCE(e.publication_year, 0) AS pub_year,
			ARRAY_AGG(e.id ORDER BY e.id) AS edition_ids
		FROM works w
		JOIN editions e ON e.work_id = w.id
		GROUP BY
			w.id,
			w.title,
			LOWER(TRIM(COALESCE(e.title, ''))),
			LOWER(TRIM(COALESCE(e.format, ''))),
			LOWER(TRIM(COALESCE(e.publisher, ''))),
			COALESCE(e.publication_year, 0)
		HAVING COUNT(*) > 1
		ORDER BY w.title ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DuplicateEdition, 0)
	for rows.Next() {
		var workID string
		var workTitle string
		var normTitle string
		var normFormat string
		var normPublisher string
		var pubYear int
		var editionIDs []string
		if err := rows.Scan(&workID, &workTitle, &normTitle, &normFormat, &normPublisher, &pubYear, &editionIDs); err != nil {
			return nil, err
		}

		out = append(out, DuplicateEdition{
			WorkID:       workID,
			WorkTitle:    workTitle,
			CanonicalKey: fmt.Sprintf("%s|%s|%s|%d", normTitle, normFormat, normPublisher, pubYear),
			EditionIDs:   editionIDs,
		})
	}
	return out, rows.Err()
}

func (r *pgRepository) ListIdentifierCandidates(ctx context.Context, limit int) ([]IdentifierCandidate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT edition_id, type, value
		FROM identifiers
		ORDER BY id ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]IdentifierCandidate, 0)
	for rows.Next() {
		var item IdentifierCandidate
		if err := rows.Scan(&item.EditionID, &item.Type, &item.Value); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *pgRepository) RepairSeriesOrder(ctx context.Context, seriesID string) (int, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT work_id, series_index
		FROM series_entries
		WHERE series_id = $1
		ORDER BY series_index ASC NULLS LAST, created_at ASC, work_id ASC`, seriesID)
	if err != nil {
		return 0, err
	}

	type entry struct {
		workID string
		index  *float64
	}
	entries := make([]entry, 0)
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.workID, &e.index); err != nil {
			rows.Close()
			return 0, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	updates := 0
	for i, e := range entries {
		target := float64(i + 1)
		if e.index != nil && math.Abs(*e.index-target) < 0.0001 {
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE series_entries
			SET series_index = $1,
				updated_at = NOW()
			WHERE series_id = $2 AND work_id = $3`, target, seriesID, e.workID); err != nil {
			return 0, err
		}
		updates++
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return updates, nil
}

func (r *pgRepository) SyncWorkFirstPublicationYear(ctx context.Context, workID string) (bool, error) {
	cmd, err := r.db.Exec(ctx, `
		UPDATE works
		SET first_pub_year = src.min_year,
			updated_at = NOW()
		FROM (
			SELECT work_id, MIN(publication_year) AS min_year
			FROM editions
			WHERE work_id = $1 AND publication_year > 0
			GROUP BY work_id
		) AS src
		WHERE works.id = src.work_id
		  AND COALESCE(works.first_pub_year, 0) <> src.min_year`, workID)
	if err != nil {
		return false, err
	}
	return cmd.RowsAffected() > 0, nil
}

func (r *pgRepository) RemoveIdentifier(ctx context.Context, id IdentifierCandidate) (bool, error) {
	cmd, err := r.db.Exec(ctx, `
		DELETE FROM identifiers
		WHERE edition_id = $1 AND type = $2 AND value = $3`, id.EditionID, id.Type, id.Value)
	if err != nil {
		return false, err
	}
	return cmd.RowsAffected() > 0, nil
}

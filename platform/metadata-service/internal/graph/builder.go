package graph

import (
	"context"
	"errors"
	"strings"

	"metadata-service/internal/model"
	"metadata-service/internal/normalize"
	"metadata-service/internal/store"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	relationshipTypeSameAuthor     = "same_author"
	relationshipTypeSameSeries     = "same_series"
	relationshipTypeRelatedSubject = "related_subject"
)

const (
	confidenceSameSeries     = 0.9
	confidenceSameAuthor     = 0.7
	confidenceRelatedSubject = 0.5
)

type UpdateSummary struct {
	AddedByType    map[string]int
	SeriesUpdated  bool
	SubjectsUpdated int
}

type Builder struct {
	db       *pgxpool.Pool
	works    store.WorkStore
	series   store.SeriesStore
	subjects store.SubjectStore
	rels     store.WorkRelationshipStore

	maxSameAuthor     int
	maxSameSeries     int
	maxRelatedSubject int

	fetchAuthorIDs       func(ctx context.Context, workID string) ([]string, error)
	fetchWorksForAuthor  func(ctx context.Context, authorID string, excludeWorkID string, limit int) ([]string, error)
	fetchSeriesIndex     func(ctx context.Context, seriesID string, workID string) (float64, bool, error)
	fetchSeriesNeighbor  func(ctx context.Context, seriesID string, workID string, currentIndex float64, previous bool) (string, error)
	fetchSeriesCandidates func(ctx context.Context, seriesID string, workID string, limit int) ([]string, error)
}

func NewBuilder(
	db *pgxpool.Pool,
	works store.WorkStore,
	series store.SeriesStore,
	subjects store.SubjectStore,
	rels store.WorkRelationshipStore,
) *Builder {
	builder := &Builder{
		db:                 db,
		works:              works,
		series:             series,
		subjects:           subjects,
		rels:               rels,
		maxSameAuthor:      25,
		maxSameSeries:      10,
		maxRelatedSubject:  10,
	}
	builder.fetchAuthorIDs = builder.getAuthorIDsForWork
	builder.fetchWorksForAuthor = builder.getWorksForAuthor
	builder.fetchSeriesIndex = builder.getSeriesIndex
	builder.fetchSeriesNeighbor = builder.getSeriesNeighbor
	builder.fetchSeriesCandidates = builder.getSeriesCandidates
	return builder
}

func (b *Builder) UpdateGraphForWork(ctx context.Context, workID string) error {
	_, err := b.UpdateGraphForWorkWithSummary(ctx, workID)
	return err
}

func (b *Builder) UpdateGraphForWorkWithSummary(ctx context.Context, workID string) (UpdateSummary, error) {
	summary := UpdateSummary{AddedByType: map[string]int{}}
	if strings.TrimSpace(workID) == "" {
		return summary, errors.New("work id is required")
	}

	work, err := b.works.GetWorkByID(ctx, workID)
	if err != nil {
		return summary, err
	}

	seriesID, err := b.updateSeries(ctx, *work)
	if err != nil {
		return summary, err
	}
	summary.SeriesUpdated = seriesID != ""

	subjectIDs, err := b.updateSubjects(ctx, *work)
	if err != nil {
		return summary, err
	}
	summary.SubjectsUpdated = len(subjectIDs)

	authorAdded, err := b.rebuildSameAuthorEdges(ctx, workID)
	if err != nil {
		return summary, err
	}
	summary.AddedByType[relationshipTypeSameAuthor] = authorAdded

	seriesAdded, err := b.rebuildSameSeriesEdges(ctx, workID, seriesID)
	if err != nil {
		return summary, err
	}
	summary.AddedByType[relationshipTypeSameSeries] = seriesAdded

	subjectAdded, err := b.rebuildRelatedSubjectEdges(ctx, workID, subjectIDs)
	if err != nil {
		return summary, err
	}
	summary.AddedByType[relationshipTypeRelatedSubject] = subjectAdded

	return summary, nil
}

func (b *Builder) updateSeries(ctx context.Context, work model.Work) (string, error) {
	if work.SeriesName == nil {
		return "", nil
	}
	name := strings.TrimSpace(*work.SeriesName)
	if name == "" {
		return "", nil
	}
	normalized := normalize.NormalizeSeriesName(name)
	if normalized == "" {
		return "", nil
	}

	seriesID, err := b.series.UpsertSeries(ctx, name, normalized)
	if err != nil {
		return "", err
	}
	if err := b.series.UpsertSeriesEntry(ctx, seriesID, work.ID, work.SeriesIndex); err != nil {
		return "", err
	}
	return seriesID, nil
}

func (b *Builder) updateSubjects(ctx context.Context, work model.Work) ([]string, error) {
	subjectIDSet := map[string]struct{}{}
	for _, raw := range work.Subjects {
		normalized := normalize.NormalizeSubject(raw)
		if normalized == "" {
			continue
		}
		subjectID, err := b.subjects.UpsertSubject(ctx, strings.TrimSpace(raw), normalized)
		if err != nil {
			return nil, err
		}
		subjectIDSet[subjectID] = struct{}{}
	}

	subjectIDs := make([]string, 0, len(subjectIDSet))
	for id := range subjectIDSet {
		subjectIDs = append(subjectIDs, id)
	}
	if err := b.subjects.SetWorkSubjects(ctx, work.ID, subjectIDs); err != nil {
		return nil, err
	}
	return subjectIDs, nil
}

func (b *Builder) rebuildSameAuthorEdges(ctx context.Context, workID string) (int, error) {
	relType := relationshipTypeSameAuthor
	if err := b.rels.DeleteRelationshipsForWork(ctx, workID, &relType); err != nil {
		return 0, err
	}

	authorIDs, err := b.fetchAuthorIDs(ctx, workID)
	if err != nil {
		return 0, err
	}

	seenTargets := map[string]struct{}{}
	added := 0
	for _, authorID := range authorIDs {
		candidates, queryErr := b.fetchWorksForAuthor(ctx, authorID, workID, b.maxSameAuthor)
		if queryErr != nil {
			return 0, queryErr
		}
		for _, target := range candidates {
			if len(seenTargets) >= b.maxSameAuthor {
				break
			}
			if _, exists := seenTargets[target]; exists {
				continue
			}
			if err := b.rels.UpsertRelationship(ctx, workID, target, relationshipTypeSameAuthor, confidenceSameAuthor, nil); err != nil {
				return 0, err
			}
			seenTargets[target] = struct{}{}
			added++
		}
		if len(seenTargets) >= b.maxSameAuthor {
			break
		}
	}

	return added, nil
}

func (b *Builder) rebuildSameSeriesEdges(ctx context.Context, workID string, seriesID string) (int, error) {
	relType := relationshipTypeSameSeries
	if err := b.rels.DeleteRelationshipsForWork(ctx, workID, &relType); err != nil {
		return 0, err
	}
	if seriesID == "" {
		return 0, nil
	}

	currentIndex, hasIndex, err := b.fetchSeriesIndex(ctx, seriesID, workID)
	if err != nil {
		return 0, err
	}

	neighbors := make([]string, 0, 2)
	if hasIndex {
		prevID, prevErr := b.fetchSeriesNeighbor(ctx, seriesID, workID, currentIndex, true)
		if prevErr != nil {
			return 0, prevErr
		}
		if prevID != "" {
			neighbors = append(neighbors, prevID)
		}

		nextID, nextErr := b.fetchSeriesNeighbor(ctx, seriesID, workID, currentIndex, false)
		if nextErr != nil {
			return 0, nextErr
		}
		if nextID != "" {
			neighbors = append(neighbors, nextID)
		}
	} else {
		neighbors, err = b.fetchSeriesCandidates(ctx, seriesID, workID, b.maxSameSeries)
		if err != nil {
			return 0, err
		}
	}

	added := 0
	for _, target := range neighbors {
		if err := b.rels.UpsertRelationship(ctx, workID, target, relationshipTypeSameSeries, confidenceSameSeries, nil); err != nil {
			return 0, err
		}
		added++
	}
	return added, nil
}

func (b *Builder) rebuildRelatedSubjectEdges(ctx context.Context, workID string, subjectIDs []string) (int, error) {
	relType := relationshipTypeRelatedSubject
	if err := b.rels.DeleteRelationshipsForWork(ctx, workID, &relType); err != nil {
		return 0, err
	}

	if len(subjectIDs) == 0 {
		subjects, err := b.subjects.GetSubjectsForWork(ctx, workID)
		if err != nil {
			return 0, err
		}
		for _, subject := range subjects {
			subjectIDs = append(subjectIDs, subject.ID)
		}
	}
	if len(subjectIDs) == 0 {
		return 0, nil
	}

	seenTargets := map[string]struct{}{}
	added := 0
	for _, subjectID := range subjectIDs {
		works, err := b.subjects.GetWorksForSubject(ctx, subjectID, b.maxRelatedSubject, 0)
		if err != nil {
			return 0, err
		}
		for _, candidate := range works {
			if candidate.ID == workID {
				continue
			}
			if len(seenTargets) >= b.maxRelatedSubject {
				break
			}
			if _, exists := seenTargets[candidate.ID]; exists {
				continue
			}
			if err := b.rels.UpsertRelationship(ctx, workID, candidate.ID, relationshipTypeRelatedSubject, confidenceRelatedSubject, nil); err != nil {
				return 0, err
			}
			seenTargets[candidate.ID] = struct{}{}
			added++
		}
		if len(seenTargets) >= b.maxRelatedSubject {
			break
		}
	}
	return added, nil
}

func (b *Builder) getAuthorIDsForWork(ctx context.Context, workID string) ([]string, error) {
	rows, err := b.db.Query(ctx, `
		SELECT author_id
		FROM work_authors
		WHERE work_id = $1`, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var authorIDs []string
	for rows.Next() {
		var authorID string
		if scanErr := rows.Scan(&authorID); scanErr != nil {
			return nil, scanErr
		}
		authorIDs = append(authorIDs, authorID)
	}
	return authorIDs, rows.Err()
}

func (b *Builder) getWorksForAuthor(ctx context.Context, authorID string, excludeWorkID string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 25
	}
	rows, err := b.db.Query(ctx, `
		SELECT wa.work_id
		FROM work_authors wa
		JOIN works w ON w.id = wa.work_id
		WHERE wa.author_id = $1
		  AND wa.work_id <> $2
		ORDER BY w.updated_at DESC, w.created_at DESC, wa.work_id ASC
		LIMIT $3`,
		authorID, excludeWorkID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workIDs []string
	for rows.Next() {
		var relatedWorkID string
		if scanErr := rows.Scan(&relatedWorkID); scanErr != nil {
			return nil, scanErr
		}
		workIDs = append(workIDs, relatedWorkID)
	}
	return workIDs, rows.Err()
}

func (b *Builder) getSeriesIndex(ctx context.Context, seriesID string, workID string) (float64, bool, error) {
	row := b.db.QueryRow(ctx, `
		SELECT series_index
		FROM series_entries
		WHERE series_id = $1 AND work_id = $2`,
		seriesID, workID,
	)
	var index *float64
	if err := row.Scan(&index); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if index == nil {
		return 0, false, nil
	}
	return *index, true, nil
}

func (b *Builder) getSeriesNeighbor(ctx context.Context, seriesID string, workID string, currentIndex float64, previous bool) (string, error) {
	if previous {
		row := b.db.QueryRow(ctx, `
			SELECT work_id
			FROM series_entries
			WHERE series_id = $1
			  AND work_id <> $2
			  AND series_index IS NOT NULL
			  AND series_index < $3
			ORDER BY series_index DESC
			LIMIT 1`,
			seriesID, workID, currentIndex,
		)
		var neighbor string
		if err := row.Scan(&neighbor); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return "", nil
			}
			return "", err
		}
		return neighbor, nil
	}

	row := b.db.QueryRow(ctx, `
		SELECT work_id
		FROM series_entries
		WHERE series_id = $1
		  AND work_id <> $2
		  AND series_index IS NOT NULL
		  AND series_index > $3
		ORDER BY series_index ASC
		LIMIT 1`,
		seriesID, workID, currentIndex,
	)
	var neighbor string
	if err := row.Scan(&neighbor); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return neighbor, nil
}

func (b *Builder) getSeriesCandidates(ctx context.Context, seriesID string, workID string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := b.db.Query(ctx, `
		SELECT work_id
		FROM series_entries
		WHERE series_id = $1
		  AND work_id <> $2
		ORDER BY series_index ASC NULLS LAST, created_at ASC
		LIMIT $3`,
		seriesID, workID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workIDs []string
	for rows.Next() {
		var relatedWorkID string
		if scanErr := rows.Scan(&relatedWorkID); scanErr != nil {
			return nil, scanErr
		}
		workIDs = append(workIDs, relatedWorkID)
	}
	return workIDs, rows.Err()
}

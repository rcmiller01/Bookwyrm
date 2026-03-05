package store

import (
	"context"
	"math"

	"metadata-service/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SeriesNeighbor struct {
	WorkID     string
	SeriesName string
	Delta      float64
}

type SeriesCandidate struct {
	WorkID     string
	SeriesName string
	Delta      float64
}

type AuthorCandidate struct {
	WorkID        string
	SharedAuthors []string
}

type SubjectCandidate struct {
	WorkID         string
	SharedSubjects []string
	OverlapRatio   float64
}

type RelationshipCandidate struct {
	WorkID           string
	RelationshipType string
	Confidence       float64
}

type RecommendReadStore interface {
	GetWorkByID(ctx context.Context, id string) (*model.Work, error)
	GetWorksByIDs(ctx context.Context, ids []string) ([]model.Work, error)
	GetSeriesNeighborsForWork(ctx context.Context, workID string) (*SeriesNeighbor, *SeriesNeighbor, error)
	GetSeriesCandidatesForWork(ctx context.Context, workID string, limit int) ([]SeriesCandidate, error)
	GetAuthorCandidatesForWork(ctx context.Context, workID string, limit int) ([]AuthorCandidate, error)
	GetSubjectCandidatesForWork(ctx context.Context, workID string, maxSubjects int, maxWorksPerSubject int) ([]SubjectCandidate, error)
	GetRelationshipCandidatesForWork(ctx context.Context, workID string, limit int) ([]RelationshipCandidate, error)
	GetEditionsByWorkIDs(ctx context.Context, workIDs []string) (map[string][]model.Edition, error)
}

type pgRecommendReadStore struct {
	db *pgxpool.Pool
}

func NewRecommendReadStore(db *pgxpool.Pool) RecommendReadStore {
	return &pgRecommendReadStore{db: db}
}

func (s *pgRecommendReadStore) GetWorkByID(ctx context.Context, id string) (*model.Work, error) {
	row := s.db.QueryRow(ctx,
		`SELECT id, title, normalized_title, fingerprint, first_pub_year, series_name, series_index, COALESCE(subjects, '{}')
		 FROM works
		 WHERE id = $1`, id)
	var work model.Work
	if err := row.Scan(&work.ID, &work.Title, &work.NormalizedTitle, &work.Fingerprint, &work.FirstPubYear, &work.SeriesName, &work.SeriesIndex, &work.Subjects); err != nil {
		return nil, err
	}
	return &work, nil
}

func (s *pgRecommendReadStore) GetWorksByIDs(ctx context.Context, ids []string) ([]model.Work, error) {
	if len(ids) == 0 {
		return []model.Work{}, nil
	}
	rows, err := s.db.Query(ctx,
		`SELECT id, title, normalized_title, fingerprint, first_pub_year, series_name, series_index, COALESCE(subjects, '{}')
		 FROM works
		 WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	works := make([]model.Work, 0, len(ids))
	for rows.Next() {
		var work model.Work
		if scanErr := rows.Scan(&work.ID, &work.Title, &work.NormalizedTitle, &work.Fingerprint, &work.FirstPubYear, &work.SeriesName, &work.SeriesIndex, &work.Subjects); scanErr != nil {
			return nil, scanErr
		}
		works = append(works, work)
	}
	return works, rows.Err()
}

func (s *pgRecommendReadStore) GetSeriesNeighborsForWork(ctx context.Context, workID string) (*SeriesNeighbor, *SeriesNeighbor, error) {
	row := s.db.QueryRow(ctx, `
		SELECT se.series_id, sr.name, se.series_index
		FROM series_entries se
		JOIN series sr ON sr.id = se.series_id
		WHERE se.work_id = $1
		LIMIT 1`, workID)

	var seriesID string
	var seriesName string
	var currentIndex *float64
	if err := row.Scan(&seriesID, &seriesName, &currentIndex); err != nil {
		return nil, nil, nil
	}

	if currentIndex == nil {
		return nil, nil, nil
	}

	prevRow := s.db.QueryRow(ctx, `
		SELECT work_id, series_index
		FROM series_entries
		WHERE series_id = $1
		  AND work_id <> $2
		  AND series_index IS NOT NULL
		  AND series_index < $3
		ORDER BY series_index DESC
		LIMIT 1`, seriesID, workID, *currentIndex)
	var prevWorkID string
	var prevIndex *float64
	if err := prevRow.Scan(&prevWorkID, &prevIndex); err != nil {
		prevWorkID = ""
		prevIndex = nil
	}

	nextRow := s.db.QueryRow(ctx, `
		SELECT work_id, series_index
		FROM series_entries
		WHERE series_id = $1
		  AND work_id <> $2
		  AND series_index IS NOT NULL
		  AND series_index > $3
		ORDER BY series_index ASC
		LIMIT 1`, seriesID, workID, *currentIndex)
	var nextWorkID string
	var nextIndex *float64
	if err := nextRow.Scan(&nextWorkID, &nextIndex); err != nil {
		nextWorkID = ""
		nextIndex = nil
	}

	var prev *SeriesNeighbor
	if prevWorkID != "" && prevIndex != nil {
		prev = &SeriesNeighbor{WorkID: prevWorkID, SeriesName: seriesName, Delta: math.Abs(*currentIndex - *prevIndex)}
	}
	var next *SeriesNeighbor
	if nextWorkID != "" && nextIndex != nil {
		next = &SeriesNeighbor{WorkID: nextWorkID, SeriesName: seriesName, Delta: math.Abs(*nextIndex - *currentIndex)}
	}

	return prev, next, nil
}

func (s *pgRecommendReadStore) GetSeriesCandidatesForWork(ctx context.Context, workID string, limit int) ([]SeriesCandidate, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(ctx, `
		WITH current_series AS (
			SELECT se.series_id, se.series_index AS current_index
			FROM series_entries se
			WHERE se.work_id = $1
			LIMIT 1
		)
		SELECT se.work_id,
		       sr.name,
		       COALESCE(ABS(se.series_index - cs.current_index), 9999.0) AS delta
		FROM current_series cs
		JOIN series_entries se ON se.series_id = cs.series_id
		JOIN series sr ON sr.id = cs.series_id
		WHERE se.work_id <> $1
		ORDER BY delta ASC, se.created_at ASC
		LIMIT $2`, workID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SeriesCandidate, 0, limit)
	for rows.Next() {
		var candidate SeriesCandidate
		if scanErr := rows.Scan(&candidate.WorkID, &candidate.SeriesName, &candidate.Delta); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, candidate)
	}
	return out, rows.Err()
}

func (s *pgRecommendReadStore) GetAuthorCandidatesForWork(ctx context.Context, workID string, limit int) ([]AuthorCandidate, error) {
	if limit <= 0 {
		limit = 25
	}
	rows, err := s.db.Query(ctx, `
		SELECT wa2.work_id,
		       ARRAY_AGG(DISTINCT a.name ORDER BY a.name) AS shared_authors
		FROM work_authors wa1
		JOIN work_authors wa2 ON wa2.author_id = wa1.author_id AND wa2.work_id <> $1
		JOIN authors a ON a.id = wa1.author_id
		WHERE wa1.work_id = $1
		GROUP BY wa2.work_id
		ORDER BY COUNT(DISTINCT wa1.author_id) DESC, wa2.work_id ASC
		LIMIT $2`, workID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AuthorCandidate, 0, limit)
	for rows.Next() {
		var candidate AuthorCandidate
		if scanErr := rows.Scan(&candidate.WorkID, &candidate.SharedAuthors); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, candidate)
	}
	return out, rows.Err()
}

func (s *pgRecommendReadStore) GetSubjectCandidatesForWork(ctx context.Context, workID string, maxSubjects int, maxWorksPerSubject int) ([]SubjectCandidate, error) {
	if maxSubjects <= 0 {
		maxSubjects = 10
	}
	if maxWorksPerSubject <= 0 {
		maxWorksPerSubject = 10
	}

	seedRows, err := s.db.Query(ctx, `
		SELECT ws.subject_id
		FROM work_subjects ws
		JOIN subjects s ON s.id = ws.subject_id
		WHERE ws.work_id = $1
		ORDER BY s.normalized_name ASC
		LIMIT $2`, workID, maxSubjects)
	if err != nil {
		return nil, err
	}
	defer seedRows.Close()

	seedSubjectIDs := make([]string, 0, maxSubjects)
	for seedRows.Next() {
		var subjectID string
		if scanErr := seedRows.Scan(&subjectID); scanErr != nil {
			return nil, scanErr
		}
		seedSubjectIDs = append(seedSubjectIDs, subjectID)
	}
	if err := seedRows.Err(); err != nil {
		return nil, err
	}
	if len(seedSubjectIDs) == 0 {
		return []SubjectCandidate{}, nil
	}

	candidateLimit := maxSubjects * maxWorksPerSubject
	rows, err := s.db.Query(ctx, `
		SELECT ws.work_id,
		       ARRAY_AGG(DISTINCT s.name ORDER BY s.name) AS shared_subjects,
		       COUNT(DISTINCT ws.subject_id)::float8 / $3::float8 AS overlap_ratio
		FROM work_subjects ws
		JOIN subjects s ON s.id = ws.subject_id
		WHERE ws.subject_id = ANY($1)
		  AND ws.work_id <> $2
		GROUP BY ws.work_id
		ORDER BY COUNT(DISTINCT ws.subject_id) DESC, ws.work_id ASC
		LIMIT $4`, seedSubjectIDs, workID, float64(len(seedSubjectIDs)), candidateLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SubjectCandidate, 0, candidateLimit)
	for rows.Next() {
		var candidate SubjectCandidate
		if scanErr := rows.Scan(&candidate.WorkID, &candidate.SharedSubjects, &candidate.OverlapRatio); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, candidate)
	}
	return out, rows.Err()
}

func (s *pgRecommendReadStore) GetRelationshipCandidatesForWork(ctx context.Context, workID string, limit int) ([]RelationshipCandidate, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(ctx, `
		SELECT target_work_id, relationship_type, confidence
		FROM work_relationships
		WHERE source_work_id = $1
		ORDER BY confidence DESC, updated_at DESC, target_work_id ASC
		LIMIT $2`, workID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]RelationshipCandidate, 0, limit)
	for rows.Next() {
		var candidate RelationshipCandidate
		if scanErr := rows.Scan(&candidate.WorkID, &candidate.RelationshipType, &candidate.Confidence); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, candidate)
	}
	return out, rows.Err()
}

func (s *pgRecommendReadStore) GetEditionsByWorkIDs(ctx context.Context, workIDs []string) (map[string][]model.Edition, error) {
	if len(workIDs) == 0 {
		return map[string][]model.Edition{}, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, work_id, title, format, publisher, publication_year
		FROM editions
		WHERE work_id = ANY($1)`, workIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string][]model.Edition, len(workIDs))
	for rows.Next() {
		var edition model.Edition
		if scanErr := rows.Scan(&edition.ID, &edition.WorkID, &edition.Title, &edition.Format, &edition.Publisher, &edition.PublicationYear); scanErr != nil {
			return nil, scanErr
		}
		out[edition.WorkID] = append(out[edition.WorkID], edition)
	}
	return out, rows.Err()
}

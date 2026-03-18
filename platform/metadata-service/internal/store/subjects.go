package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"metadata-service/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SubjectStore manages graph subjects and work-subject links.
type SubjectStore interface {
	UpsertSubject(ctx context.Context, name string, normalized string) (string, error)
	SetWorkSubjects(ctx context.Context, workID string, subjectIDs []string) error
	GetSubjectByID(ctx context.Context, subjectID string) (*model.Subject, error)
	GetSubjectsForWork(ctx context.Context, workID string) ([]model.Subject, error)
	GetWorksForSubject(ctx context.Context, subjectID string, limit int, offset int) ([]model.Work, error)
	CountSubjects(ctx context.Context) (int64, error)
}

type pgSubjectStore struct {
	db *pgxpool.Pool
}

func NewSubjectStore(db *pgxpool.Pool) SubjectStore {
	return &pgSubjectStore{db: db}
}

func (s *pgSubjectStore) UpsertSubject(ctx context.Context, name string, normalized string) (string, error) {
	name = strings.TrimSpace(name)
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		return "", errors.New("normalized subject name is required")
	}
	id := fmt.Sprintf("subject:%s", normalized)

	var returnedID string
	err := s.db.QueryRow(ctx, `
		INSERT INTO subjects (id, name, normalized_name, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (normalized_name) DO UPDATE SET
			name = EXCLUDED.name,
			updated_at = NOW()
		RETURNING id`,
		id, name, normalized,
	).Scan(&returnedID)
	if err != nil {
		return "", err
	}
	return returnedID, nil
}

func (s *pgSubjectStore) SetWorkSubjects(ctx context.Context, workID string, subjectIDs []string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM work_subjects WHERE work_id = $1`, workID); err != nil {
		return err
	}

	for _, subjectID := range subjectIDs {
		if strings.TrimSpace(subjectID) == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO work_subjects (work_id, subject_id, created_at)
			VALUES ($1, $2, NOW())
			ON CONFLICT (work_id, subject_id) DO NOTHING`,
			workID, subjectID,
		); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *pgSubjectStore) GetSubjectByID(ctx context.Context, subjectID string) (*model.Subject, error) {
	row := s.db.QueryRow(ctx, `
		SELECT id, name, normalized_name, created_at, updated_at
		FROM subjects
		WHERE id = $1`,
		subjectID,
	)
	var subject model.Subject
	if err := row.Scan(&subject.ID, &subject.Name, &subject.NormalizedName, &subject.CreatedAt, &subject.UpdatedAt); err != nil {
		return nil, err
	}
	return &subject, nil
}

func (s *pgSubjectStore) GetSubjectsForWork(ctx context.Context, workID string) ([]model.Subject, error) {
	rows, err := s.db.Query(ctx, `
		SELECT s.id, s.name, s.normalized_name, s.created_at, s.updated_at
		FROM subjects s
		JOIN work_subjects ws ON ws.subject_id = s.id
		WHERE ws.work_id = $1
		ORDER BY s.normalized_name ASC`, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subjects []model.Subject
	for rows.Next() {
		var subject model.Subject
		if scanErr := rows.Scan(
			&subject.ID,
			&subject.Name,
			&subject.NormalizedName,
			&subject.CreatedAt,
			&subject.UpdatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		subjects = append(subjects, subject)
	}
	return subjects, rows.Err()
}

func (s *pgSubjectStore) GetWorksForSubject(ctx context.Context, subjectID string, limit int, offset int) ([]model.Work, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.Query(ctx, `
		SELECT w.id, w.title, w.normalized_title, w.fingerprint, w.first_pub_year
		FROM works w
		JOIN work_subjects ws ON ws.work_id = w.id
		WHERE ws.subject_id = $1
		ORDER BY w.title ASC
		LIMIT $2 OFFSET $3`,
		subjectID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var works []model.Work
	for rows.Next() {
		var work model.Work
		if scanErr := rows.Scan(
			&work.ID,
			&work.Title,
			&work.NormalizedTitle,
			&work.Fingerprint,
			&work.FirstPubYear,
		); scanErr != nil {
			return nil, scanErr
		}
		works = append(works, work)
	}
	return works, rows.Err()
}

func (s *pgSubjectStore) CountSubjects(ctx context.Context) (int64, error) {
	row := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM subjects`)
	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

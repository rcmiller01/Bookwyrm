package importer

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"app-backend/internal/downloadqueue"
	"app-backend/internal/metrics"
)

type PGStore struct {
	db *sql.DB
}

func NewPGStore(db *sql.DB) *PGStore {
	return &PGStore{db: db}
}

func (s *PGStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PGStore) CreateOrGetFromDownload(download downloadqueue.Job, targetRoot string) (Job, error) {
	row := s.db.QueryRowContext(context.Background(), `
		SELECT id,download_job_id,COALESCE(work_id,''),COALESCE(edition_id,''),source_path,target_root,COALESCE(target_path,''),status,attempt_count,max_attempts,COALESCE(rename_template,''),naming_result_json,decision_json,COALESCE(last_error,''),locked_at,COALESCE(locked_by,''),lease_expires_at,created_at,updated_at
		FROM import_jobs WHERE download_job_id=$1`, download.ID)
	job, err := scanJob(row)
	if err == nil {
		return job, nil
	}
	naming := map[string]any{}
	decision := map[string]any{}
	namingRaw, _ := json.Marshal(naming)
	decisionRaw, _ := json.Marshal(decision)
	insert := s.db.QueryRowContext(context.Background(), `
		INSERT INTO import_jobs(download_job_id,work_id,edition_id,source_path,target_root,status,attempt_count,max_attempts,rename_template,naming_result_json,decision_json,locked_at,locked_by,lease_expires_at,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,'queued',0,3,'',$6,$7,NULL,'',NULL,NOW(),NOW())
		RETURNING id,download_job_id,COALESCE(work_id,''),COALESCE(edition_id,''),source_path,target_root,COALESCE(target_path,''),status,attempt_count,max_attempts,COALESCE(rename_template,''),naming_result_json,decision_json,COALESCE(last_error,''),locked_at,COALESCE(locked_by,''),lease_expires_at,created_at,updated_at`,
		download.ID, download.WorkID, download.EditionID, download.OutputPath, targetRoot, namingRaw, decisionRaw,
	)
	created, scanErr := scanJob(insert)
	if scanErr == nil {
		metrics.ImportJobsCreatedTotal.Inc()
	}
	return created, scanErr
}

func (s *PGStore) ClaimNextQueued(workerID string, now time.Time) (Job, bool, error) {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return Job{}, false, err
	}
	defer tx.Rollback()
	row := tx.QueryRowContext(context.Background(), `
		SELECT id,download_job_id,COALESCE(work_id,''),COALESCE(edition_id,''),source_path,target_root,COALESCE(target_path,''),status,attempt_count,max_attempts,COALESCE(rename_template,''),naming_result_json,decision_json,COALESCE(last_error,''),locked_at,COALESCE(locked_by,''),lease_expires_at,created_at,updated_at
		FROM import_jobs
		WHERE status='queued'
		ORDER BY created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1`)
	job, err := scanJob(row)
	if err == sql.ErrNoRows {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, err
	}
	if _, err := tx.ExecContext(context.Background(), `
		UPDATE import_jobs
		SET status='running',attempt_count=attempt_count+1,locked_at=NOW(),locked_by=$2,lease_expires_at=NOW() + ($3 * INTERVAL '1 second'),updated_at=NOW(),decision_json=jsonb_set(COALESCE(decision_json,'{}'::jsonb), '{locked_by}', to_jsonb($2::text), true)
		WHERE id=$1`, job.ID, workerID, int(ImportJobLeaseTTL.Seconds()),
	); err != nil {
		return Job{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Job{}, false, err
	}
	updated, err := s.GetJob(job.ID)
	if err != nil {
		return Job{}, false, err
	}
	return updated, true, nil
}

func (s *PGStore) RecoverExpiredLeases(now time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(context.Background(), `
		SELECT id, attempt_count, max_attempts, created_at
		FROM import_jobs
		WHERE status='running'
		  AND lease_expires_at IS NOT NULL
		  AND lease_expires_at <= $1
		ORDER BY lease_expires_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT $2`, now.UTC(), limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	recovered := 0
	for rows.Next() {
		var id int64
		var attemptCount int
		var maxAttempts int
		var createdAt time.Time
		if scanErr := rows.Scan(&id, &attemptCount, &maxAttempts, &createdAt); scanErr != nil {
			return 0, scanErr
		}
		nextAttempt := attemptCount + 1
		status := string(JobStatusQueued)
		if nextAttempt >= maxAttempts {
			status = string(JobStatusFailed)
		}
		if _, execErr := tx.ExecContext(context.Background(), `
			UPDATE import_jobs
			SET status=$2,
			    attempt_count=$3,
			    last_error='lease expired; recovered',
			    locked_at=NULL,
			    locked_by='',
			    lease_expires_at=NULL,
			    updated_at=NOW()
			WHERE id=$1`, id, status, nextAttempt); execErr != nil {
			return 0, execErr
		}
		if _, execErr := tx.ExecContext(context.Background(), `
			INSERT INTO import_events(import_job_id,ts,event_type,message,payload)
			VALUES($1,NOW(),'lease_recovered','running lease expired; job recovered',jsonb_build_object('next_status',$2,'attempt_count',$3))`, id, status, nextAttempt); execErr != nil {
			return 0, execErr
		}
		if status == string(JobStatusFailed) {
			metrics.ImportJobsFailedTotal.Inc()
			metrics.ObserveImportTerminalDuration(createdAt.UTC())
		}
		recovered++
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return recovered, nil
}

func (s *PGStore) GetJob(id int64) (Job, error) {
	row := s.db.QueryRowContext(context.Background(), `
		SELECT id,download_job_id,COALESCE(work_id,''),COALESCE(edition_id,''),source_path,target_root,COALESCE(target_path,''),status,attempt_count,max_attempts,COALESCE(rename_template,''),naming_result_json,decision_json,COALESCE(last_error,''),locked_at,COALESCE(locked_by,''),lease_expires_at,created_at,updated_at
		FROM import_jobs WHERE id=$1`, id)
	return scanJob(row)
}

func (s *PGStore) ExistsDownloadJob(downloadJobID int64) bool {
	row := s.db.QueryRowContext(context.Background(), `SELECT 1 FROM import_jobs WHERE download_job_id=$1 LIMIT 1`, downloadJobID)
	var exists int
	if err := row.Scan(&exists); err != nil {
		return false
	}
	return exists == 1
}

func (s *PGStore) ListJobs(filter JobFilter) []Job {
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var rows *sql.Rows
	var err error
	if filter.Status == "" {
		rows, err = s.db.QueryContext(context.Background(), `
			SELECT id,download_job_id,COALESCE(work_id,''),COALESCE(edition_id,''),source_path,target_root,COALESCE(target_path,''),status,attempt_count,max_attempts,COALESCE(rename_template,''),naming_result_json,decision_json,COALESCE(last_error,''),locked_at,COALESCE(locked_by,''),lease_expires_at,created_at,updated_at
			FROM import_jobs
			ORDER BY created_at DESC
			LIMIT $1`, limit)
	} else {
		rows, err = s.db.QueryContext(context.Background(), `
			SELECT id,download_job_id,COALESCE(work_id,''),COALESCE(edition_id,''),source_path,target_root,COALESCE(target_path,''),status,attempt_count,max_attempts,COALESCE(rename_template,''),naming_result_json,decision_json,COALESCE(last_error,''),locked_at,COALESCE(locked_by,''),lease_expires_at,created_at,updated_at
			FROM import_jobs
			WHERE status=$1
			ORDER BY created_at DESC
			LIMIT $2`, string(filter.Status), limit)
	}
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		job, scanErr := scanJob(rows)
		if scanErr != nil {
			continue
		}
		out = append(out, job)
	}
	return out
}

func (s *PGStore) MarkImported(id int64, targetPath string, naming map[string]any, decision map[string]any) error {
	job, err := s.GetJob(id)
	if err != nil {
		return err
	}
	namingRaw, _ := json.Marshal(naming)
	decisionRaw, _ := json.Marshal(decision)
	tag, err := s.db.ExecContext(context.Background(), `
		UPDATE import_jobs
		SET status='imported',target_path=$2,naming_result_json=$3,decision_json=$4,last_error='',locked_at=NULL,locked_by='',lease_expires_at=NULL,updated_at=NOW()
		WHERE id=$1`, id, targetPath, namingRaw, decisionRaw)
	if err != nil {
		return err
	}
	if n, _ := tag.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	metrics.ImportJobsImportedTotal.Inc()
	metrics.ObserveImportTerminalDuration(job.CreatedAt)
	return nil
}

func (s *PGStore) MarkNeedsReview(id int64, reason string, naming map[string]any, decision map[string]any) error {
	namingRaw, _ := json.Marshal(naming)
	decisionRaw, _ := json.Marshal(decision)
	tag, err := s.db.ExecContext(context.Background(), `
		UPDATE import_jobs
		SET status='needs_review',naming_result_json=$3,decision_json=$4,last_error=$2,locked_at=NULL,locked_by='',lease_expires_at=NULL,updated_at=NOW()
		WHERE id=$1`, id, reason, namingRaw, decisionRaw)
	if err != nil {
		return err
	}
	if n, _ := tag.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	metrics.ImportJobsNeedsReviewTotal.Inc()
	return nil
}

func (s *PGStore) MarkFailed(id int64, errMsg string, terminal bool) error {
	job, err := s.GetJob(id)
	if err != nil {
		return err
	}
	status := "queued"
	if terminal {
		status = "failed"
	}
	tag, err := s.db.ExecContext(context.Background(), `
		UPDATE import_jobs SET status=$2,last_error=$3,locked_at=NULL,locked_by='',lease_expires_at=NULL,updated_at=NOW() WHERE id=$1`, id, status, errMsg)
	if err != nil {
		return err
	}
	if n, _ := tag.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if status == "failed" {
		metrics.ImportJobsFailedTotal.Inc()
		metrics.ObserveImportTerminalDuration(job.CreatedAt)
	}
	return nil
}

func (s *PGStore) Retry(id int64) error {
	tag, err := s.db.ExecContext(context.Background(), `UPDATE import_jobs SET status='queued',last_error='',locked_at=NULL,locked_by='',lease_expires_at=NULL,updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if n, _ := tag.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) Approve(id int64, workID string, editionID string, templateOverride string) error {
	tag, err := s.db.ExecContext(context.Background(), `
		UPDATE import_jobs
		SET work_id=$2,edition_id=$3,rename_template=CASE WHEN $4='' THEN rename_template ELSE $4 END,status='queued',last_error='',locked_at=NULL,locked_by='',lease_expires_at=NULL,updated_at=NOW()
		WHERE id=$1`, id, workID, editionID, templateOverride)
	if err != nil {
		return err
	}
	if n, _ := tag.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) Skip(id int64, reason string) error {
	job, err := s.GetJob(id)
	if err != nil {
		return err
	}
	tag, err := s.db.ExecContext(context.Background(), `UPDATE import_jobs SET status='skipped',last_error=$2,locked_at=NULL,locked_by='',lease_expires_at=NULL,updated_at=NOW() WHERE id=$1`, id, reason)
	if err != nil {
		return err
	}
	if n, _ := tag.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	metrics.ImportJobsSkippedTotal.Inc()
	metrics.ObserveImportTerminalDuration(job.CreatedAt)
	return nil
}

func (s *PGStore) AddEvent(importJobID int64, eventType string, message string, payload map[string]any) error {
	raw, _ := json.Marshal(payload)
	_, err := s.db.ExecContext(context.Background(), `INSERT INTO import_events(import_job_id,ts,event_type,message,payload) VALUES($1,NOW(),$2,$3,$4)`, importJobID, eventType, message, raw)
	return err
}

func (s *PGStore) ListEvents(importJobID int64) []Event {
	rows, err := s.db.QueryContext(context.Background(), `SELECT id,import_job_id,ts,event_type,COALESCE(message,''),payload FROM import_events WHERE import_job_id=$1 ORDER BY ts ASC`, importJobID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		var raw []byte
		if err := rows.Scan(&e.ID, &e.ImportJobID, &e.TS, &e.EventType, &e.Message, &raw); err != nil {
			continue
		}
		_ = json.Unmarshal(raw, &e.Payload)
		out = append(out, e)
	}
	return out
}

func (s *PGStore) UpsertLibraryItem(item LibraryItem) (LibraryItem, error) {
	row := s.db.QueryRowContext(context.Background(), `
		INSERT INTO library_items(work_id,edition_id,path,format,size_bytes,checksum,created_at)
		VALUES($1,$2,$3,$4,$5,'',NOW())
		ON CONFLICT(path) DO UPDATE SET
		  work_id=EXCLUDED.work_id,
		  edition_id=EXCLUDED.edition_id,
		  format=EXCLUDED.format,
		  size_bytes=EXCLUDED.size_bytes
		RETURNING id,created_at`,
		item.WorkID, item.EditionID, item.Path, item.Format, item.SizeBytes,
	)
	if err := row.Scan(&item.ID, &item.CreatedAt); err != nil {
		return LibraryItem{}, err
	}
	return item, nil
}

func (s *PGStore) ListLibraryItems(workID string, limit int) []LibraryItem {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var rows *sql.Rows
	var err error
	if workID == "" {
		rows, err = s.db.QueryContext(context.Background(), `SELECT id,work_id,COALESCE(edition_id,''),path,format,COALESCE(size_bytes,0),created_at FROM library_items ORDER BY created_at DESC LIMIT $1`, limit)
	} else {
		rows, err = s.db.QueryContext(context.Background(), `SELECT id,work_id,COALESCE(edition_id,''),path,format,COALESCE(size_bytes,0),created_at FROM library_items WHERE work_id=$1 ORDER BY created_at DESC LIMIT $2`, workID, limit)
	}
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []LibraryItem{}
	for rows.Next() {
		var it LibraryItem
		if err := rows.Scan(&it.ID, &it.WorkID, &it.EditionID, &it.Path, &it.Format, &it.SizeBytes, &it.CreatedAt); err != nil {
			continue
		}
		out = append(out, it)
	}
	return out
}

func (s *PGStore) CountJobsByStatus() map[JobStatus]int {
	rows, err := s.db.QueryContext(context.Background(), `
		SELECT status, COUNT(*)
		FROM import_jobs
		GROUP BY status`)
	if err != nil {
		return map[JobStatus]int{}
	}
	defer rows.Close()
	out := map[JobStatus]int{}
	for rows.Next() {
		var status string
		var count int
		if scanErr := rows.Scan(&status, &count); scanErr != nil {
			continue
		}
		out[JobStatus(status)] = count
	}
	return out
}

func (s *PGStore) NextRunnableAt() *time.Time {
	row := s.db.QueryRowContext(context.Background(), `
		SELECT MIN(created_at)
		FROM import_jobs
		WHERE status = 'queued'`)
	var ts sql.NullTime
	if err := row.Scan(&ts); err != nil || !ts.Valid {
		return nil
	}
	value := ts.Time.UTC()
	return &value
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (Job, error) {
	var job Job
	var status string
	var namingRaw, decisionRaw []byte
	if err := row.Scan(
		&job.ID,
		&job.DownloadJobID,
		&job.WorkID,
		&job.EditionID,
		&job.SourcePath,
		&job.TargetRoot,
		&job.TargetPath,
		&status,
		&job.AttemptCount,
		&job.MaxAttempts,
		&job.RenameTemplate,
		&namingRaw,
		&decisionRaw,
		&job.LastError,
		&job.LockedAt,
		&job.LockedBy,
		&job.LeaseExpiresAt,
		&job.CreatedAt,
		&job.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return Job{}, ErrNotFound
		}
		return Job{}, err
	}
	job.Status = JobStatus(status)
	_ = json.Unmarshal(namingRaw, &job.NamingResult)
	_ = json.Unmarshal(decisionRaw, &job.Decision)
	if job.NamingResult == nil {
		job.NamingResult = map[string]any{}
	}
	if job.Decision == nil {
		job.Decision = map[string]any{}
	}
	return job, nil
}

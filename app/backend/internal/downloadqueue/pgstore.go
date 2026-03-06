package downloadqueue

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"time"
)

type PGStore struct {
	db *sql.DB
}

func NewPGStore(db *sql.DB) *PGStore {
	return &PGStore{db: db}
}

func (s *PGStore) UpsertClient(rec DownloadClientRecord) DownloadClientRecord {
	if rec.Priority == 0 {
		rec.Priority = 100
	}
	cfg, _ := json.Marshal(rec.Config)
	_, _ = s.db.ExecContext(context.Background(), `
		INSERT INTO download_clients(id,name,client_type,enabled,priority,config_json,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,NOW(),NOW())
		ON CONFLICT(id) DO UPDATE SET
		  name=EXCLUDED.name,
		  client_type=EXCLUDED.client_type,
		  enabled=EXCLUDED.enabled,
		  priority=EXCLUDED.priority,
		  config_json=EXCLUDED.config_json,
		  updated_at=NOW()`,
		rec.ID, rec.Name, rec.ClientType, rec.Enabled, rec.Priority, cfg,
	)
	return rec
}

func (s *PGStore) ListClients() []DownloadClientRecord {
	rows, err := s.db.QueryContext(context.Background(), `SELECT id,name,client_type,enabled,priority,config_json,created_at,updated_at FROM download_clients ORDER BY priority ASC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []DownloadClientRecord{}
	for rows.Next() {
		var rec DownloadClientRecord
		var cfg []byte
		if err := rows.Scan(&rec.ID, &rec.Name, &rec.ClientType, &rec.Enabled, &rec.Priority, &cfg, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(cfg, &rec.Config)
		out = append(out, rec)
	}
	return out
}

func (s *PGStore) CreateJob(job Job) (Job, error) {
	if job.MaxAttempts <= 0 {
		job.MaxAttempts = 3
	}
	if job.NotBefore.IsZero() {
		job.NotBefore = time.Now().UTC()
	}
	payload, _ := json.Marshal(job.RequestPayload)
	row := s.db.QueryRowContext(context.Background(), `
		INSERT INTO download_jobs(grab_id,candidate_id,work_id,edition_id,protocol,client_name,status,request_payload,max_attempts,not_before,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,'queued',$7,$8,$9,NOW(),NOW())
		RETURNING id,status,attempt_count,created_at,updated_at`,
		job.GrabID, job.CandidateID, job.WorkID, job.EditionID, job.Protocol, job.ClientName, payload, job.MaxAttempts, job.NotBefore.UTC(),
	)
	if err := row.Scan(&job.ID, &job.Status, &job.AttemptCount, &job.CreatedAt, &job.UpdatedAt); err != nil {
		return Job{}, err
	}
	return job, nil
}

func (s *PGStore) GetJob(id int64) (Job, error) {
	row := s.db.QueryRowContext(context.Background(), `SELECT id,grab_id,candidate_id,work_id,COALESCE(edition_id,''),protocol,client_name,status,COALESCE(download_id,''),COALESCE(output_path,''),request_payload,COALESCE(last_error,''),attempt_count,max_attempts,not_before,locked_at,COALESCE(locked_by,''),created_at,updated_at FROM download_jobs WHERE id=$1`, id)
	return scanJob(row)
}

func (s *PGStore) ListJobs(filter JobFilter) []Job {
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var rows *sql.Rows
	var err error
	if filter.Status == "" {
		rows, err = s.db.QueryContext(context.Background(), `SELECT id,grab_id,candidate_id,work_id,COALESCE(edition_id,''),protocol,client_name,status,COALESCE(download_id,''),COALESCE(output_path,''),request_payload,COALESCE(last_error,''),attempt_count,max_attempts,not_before,locked_at,COALESCE(locked_by,''),created_at,updated_at FROM download_jobs ORDER BY created_at DESC LIMIT $1`, limit)
	} else {
		rows, err = s.db.QueryContext(context.Background(), `SELECT id,grab_id,candidate_id,work_id,COALESCE(edition_id,''),protocol,client_name,status,COALESCE(download_id,''),COALESCE(output_path,''),request_payload,COALESCE(last_error,''),attempt_count,max_attempts,not_before,locked_at,COALESCE(locked_by,''),created_at,updated_at FROM download_jobs WHERE status=$1 ORDER BY created_at DESC LIMIT $2`, string(filter.Status), limit)
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

func (s *PGStore) ClaimNextQueued(workerID string, now time.Time) (Job, bool, error) {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return Job{}, false, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(context.Background(), `
		SELECT id,grab_id,candidate_id,work_id,COALESCE(edition_id,''),protocol,client_name,status,COALESCE(download_id,''),COALESCE(output_path,''),request_payload,COALESCE(last_error,''),attempt_count,max_attempts,not_before,locked_at,COALESCE(locked_by,''),created_at,updated_at
		FROM download_jobs
		WHERE status='queued' AND not_before <= $1
		ORDER BY created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1`, now.UTC(),
	)
	job, err := scanJob(row)
	if err == sql.ErrNoRows {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, err
	}
	if _, err := tx.ExecContext(context.Background(), `
		UPDATE download_jobs
		SET status='submitted',attempt_count=attempt_count+1,locked_at=NOW(),locked_by=$2,updated_at=NOW()
		WHERE id=$1`, job.ID, workerID,
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

func (s *PGStore) ListActiveJobs(limit int) []Job {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(context.Background(), `
		SELECT id,grab_id,candidate_id,work_id,COALESCE(edition_id,''),protocol,client_name,status,COALESCE(download_id,''),COALESCE(output_path,''),request_payload,COALESCE(last_error,''),attempt_count,max_attempts,not_before,locked_at,COALESCE(locked_by,''),created_at,updated_at
		FROM download_jobs
		WHERE status IN ('submitted','downloading','repairing','unpacking')
		ORDER BY updated_at DESC
		LIMIT $1`, limit)
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
	sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out
}

func (s *PGStore) MarkSubmitted(id int64, downloadID string) error {
	tag, err := s.db.ExecContext(context.Background(), `UPDATE download_jobs SET download_id=$2,status='downloading',locked_at=NULL,locked_by='',updated_at=NOW() WHERE id=$1`, id, downloadID)
	if err != nil {
		return err
	}
	if n, _ := tag.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) UpdateProgress(id int64, status JobStatus, outputPath string, lastErr string) error {
	tag, err := s.db.ExecContext(context.Background(), `UPDATE download_jobs SET status=$2,output_path=CASE WHEN $3='' THEN output_path ELSE $3 END,last_error=$4,updated_at=NOW() WHERE id=$1`, id, string(status), outputPath, lastErr)
	if err != nil {
		return err
	}
	if n, _ := tag.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) Reschedule(id int64, errMsg string, notBefore time.Time, terminal bool) error {
	status := "queued"
	if terminal {
		status = "failed"
	}
	tag, err := s.db.ExecContext(context.Background(), `UPDATE download_jobs SET status=$2,last_error=$3,not_before=$4,locked_at=NULL,locked_by='',updated_at=NOW() WHERE id=$1`, id, status, errMsg, notBefore.UTC())
	if err != nil {
		return err
	}
	if n, _ := tag.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) CancelJob(id int64) error {
	tag, err := s.db.ExecContext(context.Background(), `UPDATE download_jobs SET status='canceled',locked_at=NULL,locked_by='',updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if n, _ := tag.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) RetryJob(id int64) error {
	tag, err := s.db.ExecContext(context.Background(), `UPDATE download_jobs SET status='queued',last_error='',not_before=NOW(),locked_at=NULL,locked_by='',updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if n, _ := tag.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) AddEvent(event Event) (Event, error) {
	payload, _ := json.Marshal(event.Data)
	row := s.db.QueryRowContext(context.Background(), `INSERT INTO download_events(job_id,event_type,message,data_json,created_at) VALUES($1,$2,$3,$4,NOW()) RETURNING id,created_at`,
		event.JobID, event.EventType, event.Message, payload)
	if err := row.Scan(&event.ID, &event.CreatedAt); err != nil {
		return Event{}, err
	}
	return event, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (Job, error) {
	var job Job
	var payload []byte
	var status string
	if err := row.Scan(
		&job.ID,
		&job.GrabID,
		&job.CandidateID,
		&job.WorkID,
		&job.EditionID,
		&job.Protocol,
		&job.ClientName,
		&status,
		&job.DownloadID,
		&job.OutputPath,
		&payload,
		&job.LastError,
		&job.AttemptCount,
		&job.MaxAttempts,
		&job.NotBefore,
		&job.LockedAt,
		&job.LockedBy,
		&job.CreatedAt,
		&job.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return Job{}, ErrNotFound
		}
		return Job{}, err
	}
	job.Status = JobStatus(status)
	_ = json.Unmarshal(payload, &job.RequestPayload)
	return job, nil
}

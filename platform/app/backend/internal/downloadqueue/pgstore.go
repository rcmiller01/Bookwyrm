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
	if rec.Tier == "" {
		rec.Tier = "unclassified"
	}
	if rec.ReliabilityScore == 0 {
		rec.ReliabilityScore = 0.70
	}
	cfg, _ := json.Marshal(rec.Config)
	_, _ = s.db.ExecContext(context.Background(), `
		INSERT INTO download_clients(id,name,client_type,enabled,tier,reliability_score,priority,config_json,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,NOW(),NOW())
		ON CONFLICT(id) DO UPDATE SET
		  name=EXCLUDED.name,
		  client_type=EXCLUDED.client_type,
		  enabled=EXCLUDED.enabled,
		  tier=EXCLUDED.tier,
		  reliability_score=EXCLUDED.reliability_score,
		  priority=EXCLUDED.priority,
		  config_json=EXCLUDED.config_json,
		  updated_at=NOW()`,
		rec.ID, rec.Name, rec.ClientType, rec.Enabled, rec.Tier, rec.ReliabilityScore, rec.Priority, cfg,
	)
	return rec
}

func (s *PGStore) ListClients() []DownloadClientRecord {
	rows, err := s.db.QueryContext(context.Background(), `SELECT id,name,client_type,enabled,COALESCE(tier,'unclassified'),COALESCE(reliability_score,0.70),priority,config_json,created_at,updated_at FROM download_clients ORDER BY priority ASC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []DownloadClientRecord{}
	for rows.Next() {
		var rec DownloadClientRecord
		var cfg []byte
		if err := rows.Scan(&rec.ID, &rec.Name, &rec.ClientType, &rec.Enabled, &rec.Tier, &rec.ReliabilityScore, &rec.Priority, &cfg, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
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
		INSERT INTO download_jobs(grab_id,candidate_id,work_id,edition_id,protocol,client_name,status,request_payload,max_attempts,not_before,imported,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,'queued',$7,$8,$9,false,NOW(),NOW())
		RETURNING id,status,attempt_count,imported,created_at,updated_at`,
		job.GrabID, job.CandidateID, job.WorkID, job.EditionID, job.Protocol, job.ClientName, payload, job.MaxAttempts, job.NotBefore.UTC(),
	)
	if err := row.Scan(&job.ID, &job.Status, &job.AttemptCount, &job.Imported, &job.CreatedAt, &job.UpdatedAt); err != nil {
		return Job{}, err
	}
	return job, nil
}

func (s *PGStore) GetJob(id int64) (Job, error) {
	row := s.db.QueryRowContext(context.Background(), `SELECT id,grab_id,candidate_id,work_id,COALESCE(edition_id,''),protocol,client_name,status,COALESCE(download_id,''),COALESCE(output_path,''),imported,request_payload,COALESCE(last_error,''),attempt_count,max_attempts,not_before,locked_at,COALESCE(locked_by,''),lease_expires_at,created_at,updated_at FROM download_jobs WHERE id=$1`, id)
	return scanJob(row)
}

func (s *PGStore) ListJobs(filter JobFilter) []Job {
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var rows *sql.Rows
	var err error
	if filter.Status == "" && filter.Imported == nil {
		rows, err = s.db.QueryContext(context.Background(), `SELECT id,grab_id,candidate_id,work_id,COALESCE(edition_id,''),protocol,client_name,status,COALESCE(download_id,''),COALESCE(output_path,''),imported,request_payload,COALESCE(last_error,''),attempt_count,max_attempts,not_before,locked_at,COALESCE(locked_by,''),lease_expires_at,created_at,updated_at FROM download_jobs ORDER BY created_at DESC LIMIT $1`, limit)
	} else if filter.Imported == nil {
		rows, err = s.db.QueryContext(context.Background(), `SELECT id,grab_id,candidate_id,work_id,COALESCE(edition_id,''),protocol,client_name,status,COALESCE(download_id,''),COALESCE(output_path,''),imported,request_payload,COALESCE(last_error,''),attempt_count,max_attempts,not_before,locked_at,COALESCE(locked_by,''),lease_expires_at,created_at,updated_at FROM download_jobs WHERE status=$1 ORDER BY created_at DESC LIMIT $2`, string(filter.Status), limit)
	} else if filter.Status == "" {
		rows, err = s.db.QueryContext(context.Background(), `SELECT id,grab_id,candidate_id,work_id,COALESCE(edition_id,''),protocol,client_name,status,COALESCE(download_id,''),COALESCE(output_path,''),imported,request_payload,COALESCE(last_error,''),attempt_count,max_attempts,not_before,locked_at,COALESCE(locked_by,''),lease_expires_at,created_at,updated_at FROM download_jobs WHERE imported=$1 ORDER BY created_at DESC LIMIT $2`, *filter.Imported, limit)
	} else {
		rows, err = s.db.QueryContext(context.Background(), `SELECT id,grab_id,candidate_id,work_id,COALESCE(edition_id,''),protocol,client_name,status,COALESCE(download_id,''),COALESCE(output_path,''),imported,request_payload,COALESCE(last_error,''),attempt_count,max_attempts,not_before,locked_at,COALESCE(locked_by,''),lease_expires_at,created_at,updated_at FROM download_jobs WHERE status=$1 AND imported=$2 ORDER BY created_at DESC LIMIT $3`, string(filter.Status), *filter.Imported, limit)
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

func (s *PGStore) CountJobsByStatus() map[JobStatus]int {
	rows, err := s.db.QueryContext(context.Background(), `
		SELECT status, COUNT(*)
		FROM download_jobs
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

func (s *PGStore) ClaimNextQueued(workerID string, now time.Time) (Job, bool, error) {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return Job{}, false, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(context.Background(), `
		SELECT id,grab_id,candidate_id,work_id,COALESCE(edition_id,''),protocol,client_name,status,COALESCE(download_id,''),COALESCE(output_path,''),imported,request_payload,COALESCE(last_error,''),attempt_count,max_attempts,not_before,locked_at,COALESCE(locked_by,''),lease_expires_at,created_at,updated_at
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
		SET status='submitted',attempt_count=attempt_count+1,locked_at=NOW(),locked_by=$2,lease_expires_at=NOW() + ($3 * INTERVAL '1 second'),updated_at=NOW()
		WHERE id=$1`, job.ID, workerID, int(DownloadJobLeaseTTL.Seconds()),
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
		SELECT id, attempt_count, max_attempts
		FROM download_jobs
		WHERE status='submitted'
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
		if scanErr := rows.Scan(&id, &attemptCount, &maxAttempts); scanErr != nil {
			return 0, scanErr
		}
		nextAttempt := attemptCount + 1
		status := string(JobStatusQueued)
		notBefore := now.UTC().Add(recoveryBackoffForAttempt(nextAttempt))
		if nextAttempt >= maxAttempts {
			status = string(JobStatusFailed)
			notBefore = now.UTC()
		}
		if _, execErr := tx.ExecContext(context.Background(), `
			UPDATE download_jobs
			SET status=$2,
			    attempt_count=$3,
			    last_error='lease expired; recovered',
			    not_before=$4,
			    locked_at=NULL,
			    locked_by='',
			    lease_expires_at=NULL,
			    updated_at=NOW()
			WHERE id=$1`, id, status, nextAttempt, notBefore); execErr != nil {
			return 0, execErr
		}
		if _, execErr := tx.ExecContext(context.Background(), `
			INSERT INTO download_events(job_id,event_type,message,data_json,created_at)
			VALUES($1,'lease_recovered','submitted lease expired; job recovered',jsonb_build_object('next_status',$2,'attempt_count',$3),NOW())`, id, status, nextAttempt); execErr != nil {
			return 0, execErr
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

func (s *PGStore) ListActiveJobs(limit int) []Job {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(context.Background(), `
		SELECT id,grab_id,candidate_id,work_id,COALESCE(edition_id,''),protocol,client_name,status,COALESCE(download_id,''),COALESCE(output_path,''),imported,request_payload,COALESCE(last_error,''),attempt_count,max_attempts,not_before,locked_at,COALESCE(locked_by,''),lease_expires_at,created_at,updated_at
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

func (s *PGStore) ListCompletedNotImported(limit int) []Job {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(context.Background(), `
		SELECT id,grab_id,candidate_id,work_id,COALESCE(edition_id,''),protocol,client_name,status,COALESCE(download_id,''),COALESCE(output_path,''),imported,request_payload,COALESCE(last_error,''),attempt_count,max_attempts,not_before,locked_at,COALESCE(locked_by,''),lease_expires_at,created_at,updated_at
		FROM download_jobs
		WHERE status='completed' AND imported=false
		ORDER BY updated_at ASC
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
	return out
}

func (s *PGStore) MarkSubmitted(id int64, downloadID string) error {
	tag, err := s.db.ExecContext(context.Background(), `UPDATE download_jobs SET download_id=$2,status='downloading',locked_at=NULL,locked_by='',lease_expires_at=NULL,updated_at=NOW() WHERE id=$1`, id, downloadID)
	if err != nil {
		return err
	}
	if n, _ := tag.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) UpdateProgress(id int64, status JobStatus, outputPath string, lastErr string) error {
	tag, err := s.db.ExecContext(context.Background(), `UPDATE download_jobs SET status=$2,output_path=CASE WHEN $3='' THEN output_path ELSE $3 END,last_error=$4,lease_expires_at=NULL,updated_at=NOW() WHERE id=$1`, id, string(status), outputPath, lastErr)
	if err != nil {
		return err
	}
	if n, _ := tag.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) MarkImported(id int64, imported bool) error {
	tag, err := s.db.ExecContext(context.Background(), `UPDATE download_jobs SET imported=$2,updated_at=NOW() WHERE id=$1`, id, imported)
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
	tag, err := s.db.ExecContext(context.Background(), `UPDATE download_jobs SET status=$2,last_error=$3,not_before=$4,locked_at=NULL,locked_by='',lease_expires_at=NULL,updated_at=NOW() WHERE id=$1`, id, status, errMsg, notBefore.UTC())
	if err != nil {
		return err
	}
	if n, _ := tag.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) CancelJob(id int64) error {
	tag, err := s.db.ExecContext(context.Background(), `UPDATE download_jobs SET status='canceled',locked_at=NULL,locked_by='',lease_expires_at=NULL,updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if n, _ := tag.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) RetryJob(id int64) error {
	tag, err := s.db.ExecContext(context.Background(), `UPDATE download_jobs SET status='queued',last_error='',not_before=NOW(),locked_at=NULL,locked_by='',lease_expires_at=NULL,updated_at=NOW() WHERE id=$1`, id)
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

func (s *PGStore) ListEvents(jobID int64) []Event {
	rows, err := s.db.QueryContext(context.Background(), `
		SELECT id, job_id, event_type, message, data_json, created_at
		FROM download_events
		WHERE job_id=$1
		ORDER BY created_at ASC, id ASC`, jobID)
	if err != nil {
		return []Event{}
	}
	defer rows.Close()
	out := make([]Event, 0)
	for rows.Next() {
		var event Event
		var payload []byte
		if scanErr := rows.Scan(&event.ID, &event.JobID, &event.EventType, &event.Message, &payload, &event.CreatedAt); scanErr != nil {
			continue
		}
		_ = json.Unmarshal(payload, &event.Data)
		if event.Data == nil {
			event.Data = map[string]any{}
		}
		out = append(out, event)
	}
	return out
}

func (s *PGStore) RecordClientResult(clientID string, success bool, latency time.Duration, terminalComplete bool) error {
	successInc := 0
	failureInc := 0
	completionInc := 0
	if success {
		successInc = 1
	} else {
		failureInc = 1
	}
	if terminalComplete {
		completionInc = 1
	}
	_, err := s.db.ExecContext(context.Background(), `
		INSERT INTO download_client_metrics(client_id,success_count,failure_count,total_latency_ms,poll_count,completion_count,updated_at)
		VALUES($1,$2,$3,$4,1,$5,NOW())
		ON CONFLICT(client_id) DO UPDATE SET
		  success_count=download_client_metrics.success_count + EXCLUDED.success_count,
		  failure_count=download_client_metrics.failure_count + EXCLUDED.failure_count,
		  total_latency_ms=download_client_metrics.total_latency_ms + EXCLUDED.total_latency_ms,
		  poll_count=download_client_metrics.poll_count + 1,
		  completion_count=download_client_metrics.completion_count + EXCLUDED.completion_count,
		  updated_at=NOW()`,
		clientID, successInc, failureInc, latency.Milliseconds(), completionInc,
	)
	return err
}

func (s *PGStore) RecomputeClientReliability() error {
	rows, err := s.db.QueryContext(context.Background(), `
		SELECT
		  c.id,
		  COALESCE(m.success_count,0),
		  COALESCE(m.failure_count,0),
		  COALESCE(m.total_latency_ms,0),
		  COALESCE(m.poll_count,0),
		  COALESCE(m.completion_count,0)
		FROM download_clients c
		LEFT JOIN download_client_metrics m ON m.client_id=c.id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for rows.Next() {
		var id string
		var successCount, failureCount, totalLatencyMS, pollCount, completionCount int64
		if err := rows.Scan(&id, &successCount, &failureCount, &totalLatencyMS, &pollCount, &completionCount); err != nil {
			return err
		}
		availability := 0.70
		total := successCount + failureCount
		if total > 0 {
			availability = float64(successCount) / float64(total)
		}
		latencyScore := 0.70
		if pollCount > 0 {
			avg := float64(totalLatencyMS) / float64(pollCount)
			latencyScore = 1.0 - (avg / 5000.0)
			if latencyScore < 0 {
				latencyScore = 0
			}
		}
		completion := 0.70
		if pollCount > 0 {
			completion = float64(completionCount) / float64(pollCount)
		}
		score := (availability * 0.50) + (latencyScore * 0.30) + (completion * 0.20)
		tier := reliabilityTier(score)
		if _, err := tx.ExecContext(context.Background(), `
			INSERT INTO download_client_reliability(client_id,availability_score,latency_score,completion_score,composite_score,tier,computed_at)
			VALUES($1,$2,$3,$4,$5,$6,NOW())
			ON CONFLICT(client_id) DO UPDATE SET
			  availability_score=EXCLUDED.availability_score,
			  latency_score=EXCLUDED.latency_score,
			  completion_score=EXCLUDED.completion_score,
			  composite_score=EXCLUDED.composite_score,
			  tier=EXCLUDED.tier,
			  computed_at=NOW()`,
			id, availability, latencyScore, completion, score, tier,
		); err != nil {
			return err
		}
		if _, err := tx.ExecContext(context.Background(), `UPDATE download_clients SET reliability_score=$2,tier=$3,updated_at=NOW() WHERE id=$1`, id, score, tier); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return tx.Commit()
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
		&job.Imported,
		&payload,
		&job.LastError,
		&job.AttemptCount,
		&job.MaxAttempts,
		&job.NotBefore,
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
	_ = json.Unmarshal(payload, &job.RequestPayload)
	return job, nil
}

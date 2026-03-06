package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PGStore struct {
	db *pgxpool.Pool
}

func NewPGStore(db *pgxpool.Pool) *PGStore {
	return &PGStore{db: db}
}

func (s *PGStore) UpsertBackend(rec BackendRecord) BackendRecord {
	if rec.ReliabilityScore == 0 {
		rec.ReliabilityScore = 0.70
	}
	if rec.Tier == "" {
		rec.Tier = TierUnclassified
	}
	cfg, _ := json.Marshal(rec.Config)
	_, _ = s.db.Exec(context.Background(), `
		INSERT INTO indexer_backends (id, name, backend_type, enabled, tier, reliability_score, priority, config_json, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW(),NOW())
		ON CONFLICT (id) DO UPDATE SET
		  name=EXCLUDED.name,
		  backend_type=EXCLUDED.backend_type,
		  enabled=EXCLUDED.enabled,
		  tier=EXCLUDED.tier,
		  reliability_score=EXCLUDED.reliability_score,
		  priority=EXCLUDED.priority,
		  config_json=EXCLUDED.config_json,
		  updated_at=NOW()`,
		rec.ID, rec.Name, string(rec.BackendType), rec.Enabled, string(rec.Tier), rec.ReliabilityScore, rec.Priority, cfg,
	)
	return rec
}

func (s *PGStore) ListBackends() []BackendRecord {
	rows, err := s.db.Query(context.Background(), `
		SELECT id, name, backend_type, enabled, tier, reliability_score, priority, config_json, created_at, updated_at
		FROM indexer_backends ORDER BY priority ASC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []BackendRecord{}
	for rows.Next() {
		var rec BackendRecord
		var backendType, tier string
		var cfg []byte
		if err := rows.Scan(&rec.ID, &rec.Name, &backendType, &rec.Enabled, &tier, &rec.ReliabilityScore, &rec.Priority, &cfg, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			continue
		}
		rec.BackendType = BackendType(backendType)
		rec.Tier = DispatchTier(tier)
		_ = json.Unmarshal(cfg, &rec.Config)
		out = append(out, rec)
	}
	return out
}

func (s *PGStore) SetBackendEnabled(id string, enabled bool) error {
	tag, err := s.db.Exec(context.Background(), `UPDATE indexer_backends SET enabled=$1, updated_at=NOW() WHERE id=$2`, enabled, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) SetBackendPriority(id string, priority int) error {
	tag, err := s.db.Exec(context.Background(), `UPDATE indexer_backends SET priority=$1, updated_at=NOW() WHERE id=$2`, priority, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) SetBackendReliability(id string, score float64, tier DispatchTier) error {
	tag, err := s.db.Exec(context.Background(), `UPDATE indexer_backends SET reliability_score=$1, tier=$2, updated_at=NOW() WHERE id=$3`, score, string(tier), id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) UpsertMCPServer(rec MCPServerRecord) MCPServerRecord {
	envSchema, _ := json.Marshal(rec.EnvSchema)
	envMapping, _ := json.Marshal(rec.EnvMapping)
	_, _ = s.db.Exec(context.Background(), `
		INSERT INTO mcp_servers (id, name, source, source_ref, enabled, base_url, env_schema, env_mapping, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW(),NOW())
		ON CONFLICT (id) DO UPDATE SET
		  name=EXCLUDED.name,
		  source=EXCLUDED.source,
		  source_ref=EXCLUDED.source_ref,
		  enabled=EXCLUDED.enabled,
		  base_url=EXCLUDED.base_url,
		  env_schema=EXCLUDED.env_schema,
		  env_mapping=EXCLUDED.env_mapping,
		  updated_at=NOW()`,
		rec.ID, rec.Name, rec.Source, rec.SourceRef, rec.Enabled, rec.BaseURL, envSchema, envMapping,
	)
	return rec
}

func (s *PGStore) ListMCPServers() []MCPServerRecord {
	rows, err := s.db.Query(context.Background(), `SELECT id,name,source,source_ref,enabled,COALESCE(base_url,''),env_schema,env_mapping,created_at,updated_at FROM mcp_servers ORDER BY id`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []MCPServerRecord{}
	for rows.Next() {
		var rec MCPServerRecord
		var envSchema, envMapping []byte
		if err := rows.Scan(&rec.ID, &rec.Name, &rec.Source, &rec.SourceRef, &rec.Enabled, &rec.BaseURL, &envSchema, &envMapping, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(envSchema, &rec.EnvSchema)
		_ = json.Unmarshal(envMapping, &rec.EnvMapping)
		out = append(out, rec)
	}
	return out
}

func (s *PGStore) GetMCPServer(id string) (MCPServerRecord, error) {
	var rec MCPServerRecord
	var envSchema, envMapping []byte
	row := s.db.QueryRow(context.Background(), `SELECT id,name,source,source_ref,enabled,COALESCE(base_url,''),env_schema,env_mapping,created_at,updated_at FROM mcp_servers WHERE id=$1`, id)
	if err := row.Scan(&rec.ID, &rec.Name, &rec.Source, &rec.SourceRef, &rec.Enabled, &rec.BaseURL, &envSchema, &envMapping, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return MCPServerRecord{}, ErrNotFound
		}
		return MCPServerRecord{}, err
	}
	_ = json.Unmarshal(envSchema, &rec.EnvSchema)
	_ = json.Unmarshal(envMapping, &rec.EnvMapping)
	return rec, nil
}

func (s *PGStore) SetMCPEnabled(id string, enabled bool) error {
	tag, err := s.db.Exec(context.Background(), `UPDATE mcp_servers SET enabled=$1, updated_at=NOW() WHERE id=$2`, enabled, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) SetMCPEnvMapping(id string, mapping map[string]string) error {
	rec, err := s.GetMCPServer(id)
	if err != nil {
		return err
	}
	if rec.EnvMapping == nil {
		rec.EnvMapping = map[string]string{}
	}
	for k, v := range mapping {
		rec.EnvMapping[k] = v
	}
	raw, _ := json.Marshal(rec.EnvMapping)
	tag, err := s.db.Exec(context.Background(), `UPDATE mcp_servers SET env_mapping=$1, updated_at=NOW() WHERE id=$2`, raw, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) CreateOrGetSearchRequest(requestKey string, query QuerySpec, maxAttempts int) SearchRequestRecord {
	row := s.db.QueryRow(context.Background(), `SELECT id,request_key,entity_type,entity_id,query_json,status,attempt_count,max_attempts,COALESCE(last_error,''),not_before,locked_at,COALESCE(locked_by,''),created_at,updated_at FROM indexer_search_requests WHERE request_key=$1`, requestKey)
	rec, err := scanSearchRequestRow(row)
	if err == nil {
		return rec
	}
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	queryJSON, _ := json.Marshal(query)
	insertRow := s.db.QueryRow(context.Background(), `
		INSERT INTO indexer_search_requests (request_key,entity_type,entity_id,query_json,status,attempt_count,max_attempts,last_error,not_before,created_at,updated_at)
		VALUES ($1,$2,$3,$4,'queued',0,$5,'',NOW(),NOW(),NOW())
		RETURNING id,request_key,entity_type,entity_id,query_json,status,attempt_count,max_attempts,COALESCE(last_error,''),not_before,locked_at,COALESCE(locked_by,''),created_at,updated_at`,
		requestKey, query.EntityType, query.EntityID, queryJSON, maxAttempts,
	)
	rec, _ = scanSearchRequestRow(insertRow)
	return rec
}

func (s *PGStore) GetSearchRequest(id int64) (SearchRequestRecord, error) {
	row := s.db.QueryRow(context.Background(), `SELECT id,request_key,entity_type,entity_id,query_json,status,attempt_count,max_attempts,COALESCE(last_error,''),not_before,locked_at,COALESCE(locked_by,''),created_at,updated_at FROM indexer_search_requests WHERE id=$1`, id)
	return scanSearchRequestRow(row)
}

func (s *PGStore) TryLockNextSearchRequest(workerID string, now time.Time) (SearchRequestRecord, bool, error) {
	tx, err := s.db.Begin(context.Background())
	if err != nil {
		return SearchRequestRecord{}, false, err
	}
	defer tx.Rollback(context.Background())
	row := tx.QueryRow(context.Background(), `
		SELECT id,request_key,entity_type,entity_id,query_json,status,attempt_count,max_attempts,COALESCE(last_error,''),not_before,locked_at,COALESCE(locked_by,''),created_at,updated_at
		FROM indexer_search_requests
		WHERE status='queued' AND not_before <= $1
		ORDER BY created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1`, now.UTC(),
	)
	rec, err := scanSearchRequestRow(row)
	if err != nil {
		if err == ErrNotFound {
			return SearchRequestRecord{}, false, nil
		}
		return SearchRequestRecord{}, false, err
	}
	_, err = tx.Exec(context.Background(), `
		UPDATE indexer_search_requests
		SET status='running', attempt_count=attempt_count+1, locked_at=NOW(), locked_by=$2, updated_at=NOW()
		WHERE id=$1`, rec.ID, workerID,
	)
	if err != nil {
		return SearchRequestRecord{}, false, err
	}
	if err := tx.Commit(context.Background()); err != nil {
		return SearchRequestRecord{}, false, err
	}
	updated, err := s.GetSearchRequest(rec.ID)
	if err != nil {
		return SearchRequestRecord{}, false, err
	}
	return updated, true, nil
}

func (s *PGStore) RescheduleSearchRequest(id int64, lastErr string, notBefore time.Time, terminal bool) error {
	status := "queued"
	if terminal {
		status = "failed"
	}
	tag, err := s.db.Exec(context.Background(), `
		UPDATE indexer_search_requests
		SET status=$2,last_error=$3,not_before=$4,locked_at=NULL,locked_by='',updated_at=NOW()
		WHERE id=$1`, id, status, lastErr, notBefore.UTC(),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) MarkSearchRequestSucceeded(id int64) error {
	tag, err := s.db.Exec(context.Background(), `
		UPDATE indexer_search_requests
		SET status='succeeded', last_error='', locked_at=NULL, locked_by='', updated_at=NOW()
		WHERE id=$1`, id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) ReplaceCandidates(requestID int64, candidates []Candidate) ([]CandidateRecord, error) {
	tx, err := s.db.Begin(context.Background())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(context.Background(), `DELETE FROM indexer_candidates WHERE search_request_id=$1`, requestID); err != nil {
		return nil, err
	}
	records := make([]CandidateRecord, 0, len(candidates))
	for _, c := range candidates {
		ids, _ := json.Marshal(c.Identifiers)
		attrs, _ := json.Marshal(c.Attributes)
		grab, _ := json.Marshal(c.GrabPayload)
		reasons, _ := json.Marshal(c.Reasons)
		row := tx.QueryRow(context.Background(), `
			INSERT INTO indexer_candidates
			(search_request_id, source_pipeline, source_backend_id, title, normalized_title, protocol, size_bytes, seeders, leechers, published_at, identifiers, attributes, grab_payload, score, reasons, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,NOW())
			RETURNING id, created_at`,
			requestID, c.SourcePipeline, c.SourceBackendID, c.Title, c.NormalizedTitle, c.Protocol, c.SizeBytes, c.Seeders, c.Leechers, c.PublishedAt, ids, attrs, grab, c.Score, reasons,
		)
		var rec CandidateRecord
		rec.SearchRequestID = requestID
		rec.Candidate = c
		if err := row.Scan(&rec.ID, &rec.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	if err := tx.Commit(context.Background()); err != nil {
		return nil, err
	}
	sort.SliceStable(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	return records, nil
}

func (s *PGStore) ListCandidates(requestID int64, limit int) ([]CandidateRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(context.Background(), `
		SELECT id, source_pipeline, source_backend_id, title, normalized_title, protocol, size_bytes, seeders, leechers, published_at, identifiers, attributes, grab_payload, score, reasons, created_at
		FROM indexer_candidates
		WHERE search_request_id=$1
		ORDER BY score DESC, id ASC
		LIMIT $2`, requestID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CandidateRecord{}
	for rows.Next() {
		var rec CandidateRecord
		rec.SearchRequestID = requestID
		var ids, attrs, grab, reasons []byte
		if err := rows.Scan(&rec.ID, &rec.Candidate.SourcePipeline, &rec.Candidate.SourceBackendID, &rec.Candidate.Title, &rec.Candidate.NormalizedTitle, &rec.Candidate.Protocol, &rec.Candidate.SizeBytes, &rec.Candidate.Seeders, &rec.Candidate.Leechers, &rec.Candidate.PublishedAt, &ids, &attrs, &grab, &rec.Candidate.Score, &reasons, &rec.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(ids, &rec.Candidate.Identifiers)
		_ = json.Unmarshal(attrs, &rec.Candidate.Attributes)
		_ = json.Unmarshal(grab, &rec.Candidate.GrabPayload)
		_ = json.Unmarshal(reasons, &rec.Candidate.Reasons)
		out = append(out, rec)
	}
	if len(out) == 0 {
		return nil, ErrNotFound
	}
	return out, nil
}

func (s *PGStore) GetCandidateByID(id int64) (CandidateRecord, error) {
	row := s.db.QueryRow(context.Background(), `
		SELECT search_request_id, source_pipeline, source_backend_id, title, normalized_title, protocol, size_bytes, seeders, leechers, published_at, identifiers, attributes, grab_payload, score, reasons, created_at
		FROM indexer_candidates WHERE id=$1`, id)
	var rec CandidateRecord
	rec.ID = id
	var ids, attrs, grab, reasons []byte
	if err := row.Scan(&rec.SearchRequestID, &rec.Candidate.SourcePipeline, &rec.Candidate.SourceBackendID, &rec.Candidate.Title, &rec.Candidate.NormalizedTitle, &rec.Candidate.Protocol, &rec.Candidate.SizeBytes, &rec.Candidate.Seeders, &rec.Candidate.Leechers, &rec.Candidate.PublishedAt, &ids, &attrs, &grab, &rec.Candidate.Score, &reasons, &rec.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return CandidateRecord{}, ErrNotFound
		}
		return CandidateRecord{}, err
	}
	_ = json.Unmarshal(ids, &rec.Candidate.Identifiers)
	_ = json.Unmarshal(attrs, &rec.Candidate.Attributes)
	_ = json.Unmarshal(grab, &rec.Candidate.GrabPayload)
	_ = json.Unmarshal(reasons, &rec.Candidate.Reasons)
	return rec, nil
}

func (s *PGStore) CreateGrab(candidateID int64, entityType string, entityID string) (GrabRecord, error) {
	row := s.db.QueryRow(context.Background(), `
		INSERT INTO indexer_grabs (candidate_id, entity_type, entity_id, status, created_at, updated_at)
		VALUES ($1,$2,$3,'created',NOW(),NOW())
		RETURNING id,status,downstream_ref,created_at,updated_at`,
		candidateID, entityType, entityID,
	)
	var rec GrabRecord
	rec.CandidateID = candidateID
	rec.EntityType = entityType
	rec.EntityID = entityID
	if err := row.Scan(&rec.ID, &rec.Status, &rec.DownstreamRef, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		return GrabRecord{}, err
	}
	return rec, nil
}

func (s *PGStore) GetGrabByID(id int64) (GrabRecord, error) {
	row := s.db.QueryRow(context.Background(), `
		SELECT candidate_id, entity_type, entity_id, status, COALESCE(downstream_ref,''), created_at, updated_at
		FROM indexer_grabs WHERE id=$1`, id)
	var rec GrabRecord
	rec.ID = id
	if err := row.Scan(&rec.CandidateID, &rec.EntityType, &rec.EntityID, &rec.Status, &rec.DownstreamRef, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return GrabRecord{}, ErrNotFound
		}
		return GrabRecord{}, err
	}
	return rec, nil
}

func (s *PGStore) RecordBackendSearchResult(backendID string, success bool, latency time.Duration, yielded bool) error {
	successInc := 0
	failureInc := 0
	yieldInc := 0
	if success {
		successInc = 1
	} else {
		failureInc = 1
	}
	if yielded {
		yieldInc = 1
	}
	_, err := s.db.Exec(context.Background(), `
		INSERT INTO indexer_metrics
			(backend_id, success_count, failure_count, total_latency_ms, search_count, candidate_yield_count, updated_at)
		VALUES
			($1,$2,$3,$4,1,$5,NOW())
		ON CONFLICT (backend_id) DO UPDATE SET
			success_count = indexer_metrics.success_count + EXCLUDED.success_count,
			failure_count = indexer_metrics.failure_count + EXCLUDED.failure_count,
			total_latency_ms = indexer_metrics.total_latency_ms + EXCLUDED.total_latency_ms,
			search_count = indexer_metrics.search_count + 1,
			candidate_yield_count = indexer_metrics.candidate_yield_count + EXCLUDED.candidate_yield_count,
			updated_at = NOW()`,
		backendID, successInc, failureInc, latency.Milliseconds(), yieldInc,
	)
	if err != nil {
		return err
	}
	return nil
}

func (s *PGStore) RecomputeReliability() error {
	rows, err := s.db.Query(context.Background(), `
		SELECT
			b.id,
			COALESCE(m.success_count, 0),
			COALESCE(m.failure_count, 0),
			COALESCE(m.total_latency_ms, 0),
			COALESCE(m.search_count, 0),
			COALESCE(m.candidate_yield_count, 0)
		FROM indexer_backends b
		LEFT JOIN indexer_metrics m ON m.backend_id = b.id`)
	if err != nil {
		return err
	}
	defer rows.Close()

	tx, err := s.db.Begin(context.Background())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())

	for rows.Next() {
		var backendID string
		var successCount int64
		var failureCount int64
		var totalLatencyMS int64
		var searchCount int64
		var yieldCount int64
		if err := rows.Scan(&backendID, &successCount, &failureCount, &totalLatencyMS, &searchCount, &yieldCount); err != nil {
			return err
		}

		availability := 0.70
		totalOutcomes := successCount + failureCount
		if totalOutcomes > 0 {
			availability = float64(successCount) / float64(totalOutcomes)
		}

		latencyScore := 0.70
		if searchCount > 0 {
			avgLatencyMS := float64(totalLatencyMS) / float64(searchCount)
			latencyScore = 1.0 - (avgLatencyMS / 5000.0)
			if latencyScore < 0 {
				latencyScore = 0
			}
		}

		yieldScore := 0.70
		if searchCount > 0 {
			yieldScore = float64(yieldCount) / float64(searchCount)
		}

		composite := (availability * 0.50) + (latencyScore * 0.30) + (yieldScore * 0.20)
		tier := tierForReliability(composite)
		if _, err := tx.Exec(context.Background(), `
			INSERT INTO indexer_reliability
				(backend_id, availability_score, latency_score, yield_score, composite_score, tier, computed_at)
			VALUES
				($1,$2,$3,$4,$5,$6,NOW())
			ON CONFLICT (backend_id) DO UPDATE SET
				availability_score=EXCLUDED.availability_score,
				latency_score=EXCLUDED.latency_score,
				yield_score=EXCLUDED.yield_score,
				composite_score=EXCLUDED.composite_score,
				tier=EXCLUDED.tier,
				computed_at=NOW()`,
			backendID, availability, latencyScore, yieldScore, composite, string(tier),
		); err != nil {
			return err
		}

		if _, err := tx.Exec(context.Background(), `
			UPDATE indexer_backends
			SET reliability_score=$2, tier=$3, updated_at=NOW()
			WHERE id=$1`, backendID, composite, string(tier),
		); err != nil {
			return err
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}
	if err := tx.Commit(context.Background()); err != nil {
		return err
	}
	return nil
}

type scanRow interface {
	Scan(dest ...any) error
}

func scanSearchRequestRow(row scanRow) (SearchRequestRecord, error) {
	var rec SearchRequestRecord
	var queryJSON []byte
	if err := row.Scan(
		&rec.ID,
		&rec.RequestKey,
		&rec.EntityType,
		&rec.EntityID,
		&queryJSON,
		&rec.Status,
		&rec.AttemptCount,
		&rec.MaxAttempts,
		&rec.LastError,
		&rec.NotBefore,
		&rec.LockedAt,
		&rec.LockedBy,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return SearchRequestRecord{}, ErrNotFound
		}
		return SearchRequestRecord{}, err
	}
	if err := json.Unmarshal(queryJSON, &rec.Query); err != nil {
		return SearchRequestRecord{}, fmt.Errorf("decode query_json: %w", err)
	}
	return rec, nil
}

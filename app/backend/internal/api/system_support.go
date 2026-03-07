package api

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"app-backend/internal/domain"
	"app-backend/internal/downloadqueue"
	"app-backend/internal/importer"
	"app-backend/internal/version"
)

const supportBundleMaxLogLines = 600

var sensitiveKeyPattern = regexp.MustCompile(`(?i)(password|passwd|secret|api[_-]?key|token|dsn|connection|string|bearer|cookie|credential|private)`)

func (h *Handlers) SupportBundle(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	archive, err := h.buildSupportBundle(ctx)
	if err != nil {
		writeError(w, "failed to build support bundle: "+err.Error(), http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("bookwyrm-support-%s.zip", time.Now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(archive)
}

func (h *Handlers) ActionRetryFailedDownloads(w http.ResponseWriter, _ *http.Request) {
	result := map[string]any{"action": "retry_failed_downloads", "retried": 0, "failed": 0, "errors": []string{}}
	if h.downloadMgr == nil {
		result["status"] = "skipped"
		result["reason"] = "download manager not configured"
		writeJSON(w, result)
		return
	}
	candidates := h.downloadMgr.ListJobs(downloadqueue.JobFilter{Status: downloadqueue.JobStatusFailed, Limit: 500})
	errorsOut := make([]string, 0)
	retried := 0
	failed := 0
	for _, job := range candidates {
		if err := h.downloadMgr.RetryJob(job.ID); err != nil {
			failed++
			errorsOut = append(errorsOut, fmt.Sprintf("job %d: %v", job.ID, err))
			continue
		}
		retried++
	}
	result["status"] = "ok"
	result["retried"] = retried
	result["failed"] = failed
	result["errors"] = errorsOut
	writeJSON(w, result)
}

func (h *Handlers) ActionRetryFailedImports(w http.ResponseWriter, _ *http.Request) {
	result := map[string]any{"action": "retry_failed_imports", "retried": 0, "failed": 0, "errors": []string{}}
	if h.importStore == nil {
		result["status"] = "skipped"
		result["reason"] = "import store not configured"
		writeJSON(w, result)
		return
	}
	candidates := h.importStore.ListJobs(importer.JobFilter{Status: importer.JobStatusFailed, Limit: 500})
	errorsOut := make([]string, 0)
	retried := 0
	failed := 0
	for _, job := range candidates {
		if err := h.importStore.Retry(job.ID); err != nil {
			failed++
			errorsOut = append(errorsOut, fmt.Sprintf("job %d: %v", job.ID, err))
			continue
		}
		retried++
	}
	result["status"] = "ok"
	result["retried"] = retried
	result["failed"] = failed
	result["errors"] = errorsOut
	writeJSON(w, result)
}

func (h *Handlers) ActionTestConnections(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	results := map[string]any{
		"action": "test_connections",
		"download_clients": map[string]any{
			"total":  0,
			"ok":     0,
			"failed": 0,
			"errors": []string{},
		},
		"services": map[string]any{},
	}

	if h.downloadMgr != nil && h.downloadService != nil {
		clients := h.downloadMgr.ListClients()
		errorsOut := make([]string, 0)
		okCount := 0
		total := 0
		for _, c := range clients {
			if !c.Enabled {
				continue
			}
			total++
			if err := h.downloadService.TestConnection(ctx, c.ID); err != nil {
				errorsOut = append(errorsOut, fmt.Sprintf("%s: %v", c.ID, err))
				continue
			}
			okCount++
		}
		results["download_clients"] = map[string]any{
			"total":  total,
			"ok":     okCount,
			"failed": total - okCount,
			"errors": errorsOut,
		}
	}

	serviceChecks := map[string]any{}
	if strings.TrimSpace(h.metadataHealthURL) != "" {
		serviceChecks["metadata_service"] = checkURLHealth(ctx, h.metadataHealthURL+"z", os.Getenv("METADATA_SERVICE_API_KEY"))
	}
	if strings.TrimSpace(h.indexerHealthURL) != "" {
		serviceChecks["indexer_service"] = checkURLHealth(ctx, h.indexerHealthURL, os.Getenv("INDEXER_SERVICE_API_KEY"))
	}
	results["services"] = serviceChecks
	results["status"] = "ok"
	writeJSON(w, results)
}

func (h *Handlers) ActionRunCleanup(w http.ResponseWriter, _ *http.Request) {
	result := map[string]any{"action": "run_cleanup"}
	if h.importEngine == nil {
		result["status"] = "skipped"
		result["reason"] = "import engine not configured"
		writeJSON(w, result)
		return
	}
	summary, err := h.importEngine.RunMaintenance(time.Now().UTC())
	if err != nil {
		result["status"] = "degraded"
		result["error"] = err.Error()
		result["summary"] = summary
		writeJSON(w, result)
		return
	}
	result["status"] = "ok"
	result["summary"] = summary
	writeJSON(w, result)
}

func (h *Handlers) ActionRecomputeReliability(w http.ResponseWriter, _ *http.Request) {
	result := map[string]any{"action": "recompute_reliability"}
	if h.downloadMgr == nil {
		result["status"] = "skipped"
		result["reason"] = "download manager not configured"
		writeJSON(w, result)
		return
	}
	if err := h.downloadMgr.RecomputeReliability(); err != nil {
		result["status"] = "degraded"
		result["error"] = err.Error()
		writeJSON(w, result)
		return
	}
	result["status"] = "ok"
	writeJSON(w, result)
}

func (h *Handlers) ActionRerunWantedSearches(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result := map[string]any{"action": "rerun_wanted_searches", "queued": 0, "errors": []string{}}
	if strings.TrimSpace(h.indexerBaseURL) == "" {
		result["status"] = "skipped"
		result["reason"] = "indexer service URL not configured"
		writeJSON(w, result)
		return
	}

	wanted, err := h.fetchJSONObject(ctx, h.indexerBaseURL+"/v1/indexer/wanted/works", os.Getenv("INDEXER_SERVICE_API_KEY"))
	if err != nil {
		result["status"] = "degraded"
		result["error"] = err.Error()
		writeJSON(w, result)
		return
	}
	rawItems, _ := wanted["items"].([]any)
	searchItems := make([]map[string]string, 0, len(rawItems))
	for _, raw := range rawItems {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if enabled, ok := item["enabled"].(bool); ok && !enabled {
			continue
		}
		workID, _ := item["work_id"].(string)
		if strings.TrimSpace(workID) == "" {
			continue
		}
		searchItems = append(searchItems, map[string]string{
			"entity_type": "work",
			"entity_id":   strings.TrimSpace(workID),
		})
	}
	if len(searchItems) == 0 {
		result["status"] = "ok"
		result["queued"] = 0
		writeJSON(w, result)
		return
	}

	payload := map[string]any{"items": searchItems}
	resp, err := h.postJSONObject(ctx, h.indexerBaseURL+"/v1/indexer/search/bulk", payload, os.Getenv("INDEXER_SERVICE_API_KEY"))
	if err != nil {
		result["status"] = "degraded"
		result["error"] = err.Error()
		writeJSON(w, result)
		return
	}
	if queued, ok := resp["items"].([]any); ok {
		result["queued"] = len(queued)
	} else {
		result["queued"] = len(searchItems)
	}
	result["status"] = "ok"
	writeJSON(w, result)
}

func (h *Handlers) ActionRerunEnrichment(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result := map[string]any{"action": "rerun_enrichment", "queued": 0, "errors": []string{}}

	if strings.TrimSpace(h.metadataBaseURL) == "" {
		result["status"] = "skipped"
		result["reason"] = "metadata service URL not configured"
		writeJSON(w, result)
		return
	}

	if h.importStore == nil {
		result["status"] = "skipped"
		result["reason"] = "import store not configured"
		writeJSON(w, result)
		return
	}

	items := h.importStore.ListLibraryItems("", 50)
	workSeen := map[string]struct{}{}
	workIDs := make([]string, 0, len(items))
	for _, item := range items {
		wid := strings.TrimSpace(item.WorkID)
		if wid == "" {
			continue
		}
		if _, ok := workSeen[wid]; ok {
			continue
		}
		workSeen[wid] = struct{}{}
		workIDs = append(workIDs, wid)
		if len(workIDs) >= 20 {
			break
		}
	}
	if len(workIDs) == 0 {
		result["status"] = "ok"
		writeJSON(w, result)
		return
	}

	errorsOut := make([]string, 0)
	queued := 0
	for _, wid := range workIDs {
		payload := map[string]any{
			"job_type":    "work_editions",
			"entity_type": "work",
			"entity_id":   wid,
		}
		if _, err := h.postJSONObject(ctx, h.metadataBaseURL+"/v1/enrichment/jobs", payload, os.Getenv("METADATA_SERVICE_API_KEY")); err != nil {
			errorsOut = append(errorsOut, fmt.Sprintf("%s: %v", wid, err))
			continue
		}
		queued++
	}
	result["queued"] = queued
	result["errors"] = errorsOut
	if len(errorsOut) > 0 {
		result["status"] = "degraded"
	} else {
		result["status"] = "ok"
	}
	writeJSON(w, result)
}

func (h *Handlers) buildSupportBundle(ctx context.Context) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	writeJSONFile := func(path string, data any) error {
		w, err := zw.Create(path)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}
	writeTextFile := func(path string, text string) error {
		w, err := zw.Create(path)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, text)
		return err
	}

	_ = writeJSONFile("system/build.json", map[string]any{
		"service":   "app-backend",
		"version":   version.Version,
		"commit":    version.Commit,
		"buildDate": version.BuildDate,
		"generated": time.Now().UTC().Format(time.RFC3339),
	})
	_ = writeJSONFile("system/config-summary.json", supportConfigSummary())
	_ = writeJSONFile("system/env-summary.json", supportEnvSummary())
	_ = writeJSONFile("system/status.json", h.safeFetchOrError(ctx, "backend_status", "http://local/api/v1/system/status"))
	_ = writeJSONFile("system/health-detail.json", h.safeFetchOrError(ctx, "backend_health_detail", "http://local/api/v1/system/health-detail"))
	_ = writeJSONFile("system/migration-status.json", map[string]any{
		"backend_downloadqueue_migrations": "unknown",
		"backend_importer_migrations":      "unknown",
		"notes":                            "Migration introspection is not exposed through current runtime interfaces.",
	})

	_ = writeJSONFile("queue/backend-jobs.json", h.supportJobsSnapshot())
	_ = writeJSONFile("queue/download-jobs.json", h.supportDownloadSnapshot())
	_ = writeJSONFile("queue/import-jobs.json", h.supportImportSnapshot())
	_ = writeJSONFile("queue/library-items.json", h.supportLibrarySnapshot())

	metadataKey := os.Getenv("METADATA_SERVICE_API_KEY")
	indexerKey := os.Getenv("INDEXER_SERVICE_API_KEY")

	_ = writeJSONFile("services/metadata-healthz.json", h.safeFetchHTTP(ctx, "metadata_healthz", h.metadataHealthURL+"z", metadataKey))
	_ = writeJSONFile("services/indexer-healthz.json", h.safeFetchHTTP(ctx, "indexer_healthz", h.indexerHealthURL, indexerKey))
	_ = writeJSONFile("services/metadata-providers-reliability.json", h.safeFetchHTTP(ctx, "metadata_reliability", h.metadataBaseURL+"/v1/providers/reliability", metadataKey))
	_ = writeJSONFile("services/indexer-backends-reliability.json", h.safeFetchHTTP(ctx, "indexer_reliability", h.indexerBaseURL+"/v1/indexer/backends/reliability", indexerKey))
	_ = writeJSONFile("services/wanted-works.json", h.safeFetchHTTP(ctx, "wanted_works", h.indexerBaseURL+"/v1/indexer/wanted/works", indexerKey))
	_ = writeJSONFile("services/wanted-authors.json", h.safeFetchHTTP(ctx, "wanted_authors", h.indexerBaseURL+"/v1/indexer/wanted/authors", indexerKey))
	_ = writeJSONFile("services/enrichment-stats.json", h.safeFetchHTTP(ctx, "enrichment_stats", h.metadataBaseURL+"/v1/enrichment/stats", metadataKey))

	logs := readRecentLogs(supportBundleMaxLogLines)
	for name, body := range logs {
		_ = writeTextFile(filepath.ToSlash(filepath.Join("logs", name)), redactSensitiveText(body))
	}
	if len(logs) == 0 {
		_ = writeTextFile("logs/README.txt", "No local log files were detected. Set BOOKWYRM_LOG_DIR or BOOKWYRM_LOG_FILE for log capture.")
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (h *Handlers) safeFetchOrError(_ context.Context, kind string, endpoint string) map[string]any {
	switch endpoint {
	case "http://local/api/v1/system/status":
		services := map[string]any{}
		if strings.TrimSpace(h.metadataHealthURL) != "" {
			services["metadata_service"] = map[string]any{"healthz": h.metadataHealthURL + "z"}
		}
		if strings.TrimSpace(h.indexerHealthURL) != "" {
			services["indexer_service"] = map[string]any{"healthz": h.indexerHealthURL}
		}
		return map[string]any{
			"type":             kind,
			"version":          version.Version,
			"commit":           version.Commit,
			"build":            version.BuildDate,
			"startup_time":     h.startupTime.Format(time.RFC3339),
			"library_root":     h.libraryRoot,
			"download_clients": len(h.supportDownloadClients()),
			"services":         services,
		}
	case "http://local/api/v1/system/health-detail":
		checks := make([]map[string]any, 0, 4)
		if strings.TrimSpace(h.metadataHealthURL) != "" {
			checks = append(checks, checkURLHealth(context.Background(), h.metadataHealthURL+"z", os.Getenv("METADATA_SERVICE_API_KEY")))
		}
		if strings.TrimSpace(h.indexerHealthURL) != "" {
			checks = append(checks, checkURLHealth(context.Background(), h.indexerHealthURL, os.Getenv("INDEXER_SERVICE_API_KEY")))
		}
		return map[string]any{"type": kind, "checks": checks}
	default:
		return map[string]any{"type": kind, "error": "unsupported"}
	}
}

func (h *Handlers) supportJobsSnapshot() map[string]any {
	if h.jobService == nil {
		return map[string]any{"available": false, "reason": "job service not configured"}
	}
	items := h.jobService.List(domain.JobFilter{Limit: 200})
	return map[string]any{
		"available": true,
		"count":     len(items),
		"items":     items,
	}
}

func (h *Handlers) supportDownloadSnapshot() map[string]any {
	if h.downloadMgr == nil {
		return map[string]any{"available": false, "reason": "download manager not configured"}
	}
	items := h.downloadMgr.ListJobs(downloadqueue.JobFilter{Limit: 300})
	return map[string]any{
		"available": true,
		"count":     len(items),
		"items":     items,
		"clients":   h.supportDownloadClients(),
	}
}

func (h *Handlers) supportImportSnapshot() map[string]any {
	if h.importStore == nil {
		return map[string]any{"available": false, "reason": "import store not configured"}
	}
	items := h.importStore.ListJobs(importer.JobFilter{Limit: 300})
	return map[string]any{
		"available":        true,
		"count":            len(items),
		"items":            items,
		"counts_by_status": h.importStore.CountJobsByStatus(),
	}
}

func (h *Handlers) supportLibrarySnapshot() map[string]any {
	if h.importStore == nil {
		return map[string]any{"available": false, "reason": "import store not configured"}
	}
	items := h.importStore.ListLibraryItems("", 200)
	return map[string]any{
		"available": true,
		"count":     len(items),
		"items":     items,
	}
}

func (h *Handlers) supportDownloadClients() []any {
	if h.downloadMgr == nil {
		return []any{}
	}
	out := make([]any, 0)
	for _, c := range h.downloadMgr.ListClients() {
		out = append(out, map[string]any{
			"id":                c.ID,
			"name":              c.Name,
			"type":              c.ClientType,
			"enabled":           c.Enabled,
			"tier":              c.Tier,
			"reliability_score": c.ReliabilityScore,
			"priority":          c.Priority,
		})
	}
	return out
}

func (h *Handlers) safeFetchHTTP(ctx context.Context, kind string, url string, apiKey string) map[string]any {
	url = strings.TrimSpace(url)
	if url == "" {
		return map[string]any{"type": kind, "available": false, "error": "url not configured"}
	}
	data, err := h.fetchJSONObject(ctx, url, apiKey)
	if err != nil {
		return map[string]any{"type": kind, "available": false, "error": err.Error()}
	}
	return map[string]any{"type": kind, "available": true, "data": redactSensitiveAny(data)}
}

func (h *Handlers) fetchJSONObject(ctx context.Context, url string, apiKey string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(url), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("X-API-Key", strings.TrimSpace(apiKey))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return out, nil
}

func (h *Handlers) postJSONObject(ctx context.Context, url string, body map[string]any, apiKey string) (map[string]any, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(url), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("X-API-Key", strings.TrimSpace(apiKey))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return out, nil
}

func checkURLHealth(ctx context.Context, url string, apiKey string) map[string]any {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(url), nil)
	if err != nil {
		return map[string]any{"url": url, "status": "error", "error": err.Error()}
	}
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("X-API-Key", strings.TrimSpace(apiKey))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return map[string]any{"url": url, "status": "unreachable", "error": err.Error()}
	}
	defer resp.Body.Close()
	out := map[string]any{"url": url}
	if resp.StatusCode >= 300 {
		out["status"] = "degraded"
		out["code"] = resp.StatusCode
		return out
	}
	out["status"] = "ok"
	out["code"] = resp.StatusCode
	return out
}

func supportConfigSummary() map[string]any {
	get := func(name string) map[string]any {
		v, ok := os.LookupEnv(name)
		return map[string]any{"env": name, "configured": ok && strings.TrimSpace(v) != ""}
	}
	return map[string]any{
		"library": map[string]any{
			"root":          get("LIBRARY_ROOT"),
			"trash_dir":     get("LIBRARY_TRASH_DIR"),
			"keep_incoming": get("IMPORT_KEEP_INCOMING"),
		},
		"services": map[string]any{
			"metadata_url": get("METADATA_SERVICE_URL"),
			"indexer_url":  get("INDEXER_SERVICE_URL"),
			"database_dsn": get("DATABASE_DSN"),
		},
		"download_clients": map[string]any{
			"qbittorrent_url": get("QBITTORRENT_BASE_URL"),
			"sabnzbd_url":     get("SABNZBD_BASE_URL"),
			"nzbget_url":      get("NZBGET_BASE_URL"),
		},
	}
}

func supportEnvSummary() map[string]any {
	entries := os.Environ()
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			names = append(names, strings.TrimSpace(parts[0]))
		}
	}
	sort.Strings(names)
	return map[string]any{
		"count": len(names),
		"names": names,
	}
}

func readRecentLogs(maxLines int) map[string]string {
	if maxLines <= 0 {
		maxLines = 300
	}
	out := map[string]string{}
	if f := strings.TrimSpace(os.Getenv("BOOKWYRM_LOG_FILE")); f != "" {
		if body := tailFile(f, maxLines); strings.TrimSpace(body) != "" {
			out[filepath.Base(f)] = body
		}
		return out
	}
	dir := strings.TrimSpace(os.Getenv("BOOKWYRM_LOG_DIR"))
	if dir == "" {
		dir = "./logs"
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".log") && !strings.HasSuffix(strings.ToLower(name), ".txt") {
			continue
		}
		path := filepath.Join(dir, name)
		if body := tailFile(path, maxLines); strings.TrimSpace(body) != "" {
			out[name] = body
		}
	}
	return out
}

func tailFile(path string, maxLines int) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	lines := make([]string, 0, maxLines)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > maxLines {
			lines = lines[1:]
		}
	}
	return strings.Join(lines, "\n")
}

func redactSensitiveAny(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			if sensitiveKeyPattern.MatchString(k) {
				out[k] = "<redacted>"
				continue
			}
			out[k] = redactSensitiveAny(val)
		}
		return out
	case []any:
		out := make([]any, 0, len(x))
		for _, item := range x {
			out = append(out, redactSensitiveAny(item))
		}
		return out
	case string:
		return redactSensitiveText(x)
	default:
		return x
	}
}

func redactSensitiveText(in string) string {
	trimmed := strings.TrimSpace(in)
	if trimmed == "" {
		return in
	}
	if sensitiveKeyPattern.MatchString(trimmed) {
		return "<redacted>"
	}
	out := in
	secretLinePattern := regexp.MustCompile(`(?im)^([^=\n]*?(password|secret|token|api[_-]?key|dsn|connection)[^=\n]*)=(.*)$`)
	out = secretLinePattern.ReplaceAllString(out, "$1=<redacted>")
	return out
}

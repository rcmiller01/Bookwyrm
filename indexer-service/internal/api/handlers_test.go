package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"indexer-service/internal/indexer"
	"indexer-service/internal/mcp"
)

func testService() *indexer.Service {
	svc := indexer.NewService()
	svc.Register("prowlarr", indexer.NewMockAdapter("prowlarr-primary", "prowlarr", []string{"availability"}, true, 0))
	svc.Register("non_prowlarr", indexer.NewMockAdapter("archive-primary", "non_prowlarr", []string{"availability"}, true, 0))
	return svc
}

func testHandlers() *Handlers {
	svc := testService()
	store := indexer.NewStore()
	orchestrator := indexer.NewOrchestrator(store, "last_resort")
	orchestrator.RegisterBackend(
		indexer.NewMockSearchBackend("prowlarr:mock", "prowlarr-mock", "prowlarr"),
		indexer.BackendRecord{
			ID:               "prowlarr:mock",
			Name:             "prowlarr-mock",
			BackendType:      indexer.BackendTypeProwlarr,
			Enabled:          true,
			Tier:             indexer.TierPrimary,
			ReliabilityScore: 0.85,
			Priority:         100,
		},
	)
	orchestrator.Start(context.Background(), 1)
	reg := mcp.NewRegistry(store)
	runtime := mcp.NewRuntime()
	return NewHandlers(svc, store, orchestrator, reg, runtime)
}

func TestSearchConcurrentGroups(t *testing.T) {
	h := testHandlers()
	r := NewRouter(h)

	payload := map[string]any{
		"metadata": map[string]any{
			"work_id": "work-1",
			"title":   "Dune",
		},
		"backend_groups": []string{"prowlarr", "non_prowlarr"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/indexer/search", bytes.NewReader(body))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	var parsed map[string]any
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	candidates, ok := parsed["candidates"].([]any)
	if !ok || len(candidates) < 2 {
		t.Fatalf("expected merged candidates from both groups, got %v", parsed["candidates"])
	}
}

func TestProvidersEndpoint(t *testing.T) {
	h := testHandlers()
	r := NewRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/indexer/providers", nil)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
}

func TestStatsEndpoint(t *testing.T) {
	h := testHandlers()
	r := NewRouter(h)

	// Seed one search/candidate/grab to validate non-zero counters.
	body := bytes.NewBufferString(`{"title":"Dune","author":"Frank Herbert","limit":2}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/indexer/search/work/work-stats", body)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200 enqueue, got %d", res.Code)
	}
	var enqueue map[string]any
	_ = json.NewDecoder(res.Body).Decode(&enqueue)
	reqID := int64(enqueue["search_request_id"].(float64))
	waitForStatus(t, r, reqID, "succeeded", 3*time.Second)

	candidatesReq := httptest.NewRequest(http.MethodGet, "/v1/indexer/candidates/"+itoa(reqID)+"?limit=1", nil)
	candidatesRes := httptest.NewRecorder()
	r.ServeHTTP(candidatesRes, candidatesReq)
	if candidatesRes.Code != http.StatusOK {
		t.Fatalf("expected 200 candidates, got %d", candidatesRes.Code)
	}
	var cPayload map[string]any
	_ = json.NewDecoder(candidatesRes.Body).Decode(&cPayload)
	items, _ := cPayload["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("expected at least one candidate")
	}
	first, _ := items[0].(map[string]any)
	candidateID := int64(first["id"].(float64))

	grabReq := httptest.NewRequest(http.MethodPost, "/v1/indexer/grab/"+itoa(candidateID), nil)
	grabRes := httptest.NewRecorder()
	r.ServeHTTP(grabRes, grabReq)
	if grabRes.Code != http.StatusOK {
		t.Fatalf("expected 200 grab, got %d", grabRes.Code)
	}

	statsReq := httptest.NewRequest(http.MethodGet, "/v1/indexer/stats", nil)
	statsRes := httptest.NewRecorder()
	r.ServeHTTP(statsRes, statsReq)
	if statsRes.Code != http.StatusOK {
		t.Fatalf("expected 200 stats, got %d", statsRes.Code)
	}
	var statsPayload map[string]any
	if err := json.NewDecoder(statsRes.Body).Decode(&statsPayload); err != nil {
		t.Fatalf("decode stats failed: %v", err)
	}
	stats, _ := statsPayload["stats"].(map[string]any)
	if stats["searches_executed"].(float64) < 1 {
		t.Fatalf("expected searches_executed >= 1, got %v", stats["searches_executed"])
	}
	if stats["candidates_evaluated"].(float64) < 1 {
		t.Fatalf("expected candidates_evaluated >= 1, got %v", stats["candidates_evaluated"])
	}
	if stats["grabs_performed"].(float64) < 1 {
		t.Fatalf("expected grabs_performed >= 1, got %v", stats["grabs_performed"])
	}
}

func TestSearchNoAdapters(t *testing.T) {
	svc := indexer.NewService()
	store := indexer.NewStore()
	orchestrator := indexer.NewOrchestrator(store, "last_resort")
	reg := mcp.NewRegistry(store)
	runtime := mcp.NewRuntime()
	h := NewHandlers(svc, store, orchestrator, reg, runtime)
	r := NewRouter(h)

	payload := map[string]any{
		"metadata": map[string]any{"work_id": "w1", "title": "X"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/indexer/search", bytes.NewReader(body))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.Code)
	}
}

func TestIndexerAsyncSearchFlow(t *testing.T) {
	h := testHandlers()
	r := NewRouter(h)

	body := bytes.NewBufferString(`{"title":"Dune","author":"Frank Herbert","limit":5}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/indexer/search/work/work-1", body)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200 enqueue, got %d", res.Code)
	}
	var enqueue map[string]any
	_ = json.NewDecoder(res.Body).Decode(&enqueue)
	idFloat, ok := enqueue["search_request_id"].(float64)
	if !ok || idFloat <= 0 {
		t.Fatalf("missing search request id")
	}
	reqID := int64(idFloat)
	waitForStatus(t, r, reqID, "succeeded", 3*time.Second)

	candidatesReq := httptest.NewRequest(http.MethodGet, "/v1/indexer/candidates/"+itoa(reqID)+"?limit=10", nil)
	candidatesRes := httptest.NewRecorder()
	r.ServeHTTP(candidatesRes, candidatesReq)
	if candidatesRes.Code != http.StatusOK {
		t.Fatalf("expected 200 candidates, got %d", candidatesRes.Code)
	}
	var cPayload map[string]any
	if err := json.NewDecoder(candidatesRes.Body).Decode(&cPayload); err != nil {
		t.Fatalf("decode candidates failed: %v", err)
	}
	items, _ := cPayload["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("expected at least one candidate")
	}

	first, _ := items[0].(map[string]any)
	candidateID, _ := first["id"].(float64)
	if candidateID <= 0 {
		t.Fatalf("expected candidate id in first item")
	}

	grabReq := httptest.NewRequest(http.MethodPost, "/v1/indexer/grab/"+itoa(int64(candidateID)), nil)
	grabRes := httptest.NewRecorder()
	r.ServeHTTP(grabRes, grabReq)
	if grabRes.Code != http.StatusOK {
		t.Fatalf("expected 200 grab, got %d", grabRes.Code)
	}
	var grabPayload map[string]any
	if err := json.NewDecoder(grabRes.Body).Decode(&grabPayload); err != nil {
		t.Fatalf("decode grab response failed: %v", err)
	}
	grab, _ := grabPayload["grab"].(map[string]any)
	grabID, _ := grab["id"].(float64)
	if grabID <= 0 {
		t.Fatalf("expected grab id > 0")
	}
}

func TestBackendPriorityPersists(t *testing.T) {
	h := testHandlers()
	r := NewRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/v1/indexer/backends/prowlarr:mock/priority", bytes.NewBufferString(`{"priority":42}`))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", res.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/indexer/backends", nil)
	listRes := httptest.NewRecorder()
	r.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listRes.Code)
	}
	var payload map[string]any
	if err := json.NewDecoder(listRes.Body).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	backends, _ := payload["backends"].([]any)
	for _, item := range backends {
		b, _ := item.(map[string]any)
		if b["id"] == "prowlarr:mock" {
			if b["priority"] != float64(42) {
				t.Fatalf("expected priority 42, got %v", b["priority"])
			}
			return
		}
	}
	t.Fatalf("expected prowlarr:mock backend in list")
}

func TestMCPEnvMappingDoesNotExposeSecretValues(t *testing.T) {
	h := testHandlers()
	r := NewRouter(h)
	_ = os.Setenv("MCP_TEST_SECRET", "super-secret-value")
	t.Cleanup(func() { _ = os.Unsetenv("MCP_TEST_SECRET") })

	createReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/servers", bytes.NewBufferString(`{
		"id":"srv1","name":"Server 1","source":"local","source_ref":"/tmp/srv1","enabled":true,
		"base_url":"http://127.0.0.1:65535","env_schema":{"Authorization":"MCP_TEST_SECRET"}
	}`))
	createRes := httptest.NewRecorder()
	r.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createRes.Code)
	}

	envReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/servers/srv1/env", bytes.NewBufferString(`{"Authorization":"MCP_TEST_SECRET"}`))
	envRes := httptest.NewRecorder()
	r.ServeHTTP(envRes, envReq)
	if envRes.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", envRes.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/mcp/servers", nil)
	listRes := httptest.NewRecorder()
	r.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listRes.Code)
	}
	body := listRes.Body.String()
	if strings.Contains(body, "super-secret-value") {
		t.Fatalf("response must not expose secret env var value")
	}
}

func TestGetCandidateAndGrabByIDEndpoints(t *testing.T) {
	h := testHandlers()
	r := NewRouter(h)

	body := bytes.NewBufferString(`{"title":"Dune","author":"Frank Herbert","limit":5}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/indexer/search/work/work-2", body)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200 enqueue, got %d", res.Code)
	}
	var enqueue map[string]any
	_ = json.NewDecoder(res.Body).Decode(&enqueue)
	reqID := int64(enqueue["search_request_id"].(float64))
	waitForStatus(t, r, reqID, "succeeded", 3*time.Second)

	candidatesReq := httptest.NewRequest(http.MethodGet, "/v1/indexer/candidates/"+itoa(reqID)+"?limit=1", nil)
	candidatesRes := httptest.NewRecorder()
	r.ServeHTTP(candidatesRes, candidatesReq)
	if candidatesRes.Code != http.StatusOK {
		t.Fatalf("expected 200 candidates, got %d", candidatesRes.Code)
	}
	var listPayload map[string]any
	_ = json.NewDecoder(candidatesRes.Body).Decode(&listPayload)
	items, _ := listPayload["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("expected at least one candidate")
	}
	first, _ := items[0].(map[string]any)
	candidateID := int64(first["id"].(float64))

	getCandidateReq := httptest.NewRequest(http.MethodGet, "/v1/indexer/candidates/id/"+itoa(candidateID), nil)
	getCandidateRes := httptest.NewRecorder()
	r.ServeHTTP(getCandidateRes, getCandidateReq)
	if getCandidateRes.Code != http.StatusOK {
		t.Fatalf("expected 200 candidate by id, got %d", getCandidateRes.Code)
	}

	grabReq := httptest.NewRequest(http.MethodPost, "/v1/indexer/grab/"+itoa(candidateID), nil)
	grabRes := httptest.NewRecorder()
	r.ServeHTTP(grabRes, grabReq)
	if grabRes.Code != http.StatusOK {
		t.Fatalf("expected 200 grab, got %d", grabRes.Code)
	}
	var grabPayload map[string]any
	_ = json.NewDecoder(grabRes.Body).Decode(&grabPayload)
	grab, _ := grabPayload["grab"].(map[string]any)
	grabID := int64(grab["id"].(float64))

	getGrabReq := httptest.NewRequest(http.MethodGet, "/v1/indexer/grabs/"+itoa(grabID), nil)
	getGrabRes := httptest.NewRecorder()
	r.ServeHTTP(getGrabRes, getGrabReq)
	if getGrabRes.Code != http.StatusOK {
		t.Fatalf("expected 200 grab by id, got %d", getGrabRes.Code)
	}
}

func TestWantedWorkEndpoints(t *testing.T) {
	h := testHandlers()
	r := NewRouter(h)

	putReq := httptest.NewRequest(http.MethodPut, "/v1/indexer/wanted/works/work-123", bytes.NewBufferString(`{"enabled":true,"priority":12,"cadence_minutes":45,"profile_id":"default-ebook","formats":["epub"],"languages":["en"]}`))
	putRes := httptest.NewRecorder()
	r.ServeHTTP(putRes, putReq)
	if putRes.Code != http.StatusOK {
		t.Fatalf("expected 200 set wanted work, got %d", putRes.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/indexer/wanted/works", nil)
	listRes := httptest.NewRecorder()
	r.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected 200 list wanted works, got %d", listRes.Code)
	}
	var payload map[string]any
	if err := json.NewDecoder(listRes.Body).Decode(&payload); err != nil {
		t.Fatalf("decode wanted works failed: %v", err)
	}
	items, _ := payload["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 wanted work item, got %d", len(items))
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/v1/indexer/wanted/works/work-123", nil)
	delRes := httptest.NewRecorder()
	r.ServeHTTP(delRes, delReq)
	if delRes.Code != http.StatusNoContent {
		t.Fatalf("expected 204 delete wanted work, got %d", delRes.Code)
	}
}

func TestWantedAuthorEndpoints(t *testing.T) {
	h := testHandlers()
	r := NewRouter(h)

	putReq := httptest.NewRequest(http.MethodPut, "/v1/indexer/wanted/authors/author-123", bytes.NewBufferString(`{"enabled":true,"priority":22,"cadence_minutes":30,"profile_id":"default-ebook"}`))
	putRes := httptest.NewRecorder()
	r.ServeHTTP(putRes, putReq)
	if putRes.Code != http.StatusOK {
		t.Fatalf("expected 200 set wanted author, got %d", putRes.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/indexer/wanted/authors", nil)
	listRes := httptest.NewRecorder()
	r.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected 200 list wanted authors, got %d", listRes.Code)
	}
	var payload map[string]any
	if err := json.NewDecoder(listRes.Body).Decode(&payload); err != nil {
		t.Fatalf("decode wanted authors failed: %v", err)
	}
	items, _ := payload["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 wanted author item, got %d", len(items))
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/v1/indexer/wanted/authors/author-123", nil)
	delRes := httptest.NewRecorder()
	r.ServeHTTP(delRes, delReq)
	if delRes.Code != http.StatusNoContent {
		t.Fatalf("expected 204 delete wanted author, got %d", delRes.Code)
	}
}

func TestProfilesEndpoints(t *testing.T) {
	h := testHandlers()
	r := NewRouter(h)

	createReq := httptest.NewRequest(http.MethodPost, "/v1/indexer/profiles", bytes.NewBufferString(`{
		"id":"audio-main",
		"name":"Audio Main",
		"cutoff_quality":"m4b",
		"qualities":[{"quality":"m4b","rank":1},{"quality":"mp3","rank":2}]
	}`))
	createRes := httptest.NewRecorder()
	r.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusOK {
		t.Fatalf("expected 200 create profile, got %d", createRes.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/indexer/profiles", nil)
	listRes := httptest.NewRecorder()
	r.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected 200 list profiles, got %d", listRes.Code)
	}
	var payload map[string]any
	if err := json.NewDecoder(listRes.Body).Decode(&payload); err != nil {
		t.Fatalf("decode profiles failed: %v", err)
	}
	items, _ := payload["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("expected profiles list")
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/indexer/profiles/audio-main", nil)
	deleteRes := httptest.NewRecorder()
	r.ServeHTTP(deleteRes, deleteReq)
	if deleteRes.Code != http.StatusNoContent {
		t.Fatalf("expected 204 delete profile, got %d", deleteRes.Code)
	}
}

func TestBulkSearchEndpoint(t *testing.T) {
	h := testHandlers()
	r := NewRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/v1/indexer/search/bulk", bytes.NewBufferString(`{
		"items":[
			{"entity_type":"work","entity_id":"work-1","title":"Dune"},
			{"entity_type":"work","entity_id":"work-2","title":"Hyperion"}
		]
	}`))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200 bulk search, got %d", res.Code)
	}
	var payload map[string]any
	_ = json.NewDecoder(res.Body).Decode(&payload)
	items, _ := payload["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("expected 2 bulk search requests, got %d", len(items))
	}
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	buf := make([]byte, 0, 20)
	for v > 0 {
		buf = append([]byte{byte('0' + (v % 10))}, buf...)
		v /= 10
	}
	return string(buf)
}

func waitForStatus(t *testing.T, router http.Handler, requestID int64, wanted string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		statusReq := httptest.NewRequest(http.MethodGet, "/v1/indexer/search/"+itoa(requestID), nil)
		statusRes := httptest.NewRecorder()
		router.ServeHTTP(statusRes, statusReq)
		if statusRes.Code == http.StatusOK {
			var payload map[string]any
			if err := json.NewDecoder(statusRes.Body).Decode(&payload); err == nil {
				reqRec, _ := payload["request"].(map[string]any)
				if reqRec["status"] == wanted {
					return
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for request %d to reach status %s", requestID, wanted)
}

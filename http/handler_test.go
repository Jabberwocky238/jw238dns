package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"jabberwocky238/jw238dns/storage"
	"jabberwocky238/jw238dns/types"

	"github.com/gin-gonic/gin"
)

func setupTestRouter(t *testing.T) (*gin.Engine, *storage.MemoryStorage) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	store := storage.NewMemoryStorage()
	_ = store.Create(context.Background(), &types.DNSRecord{
		Name:  "example.com.",
		Type:  types.RecordTypeA,
		TTL:   300,
		Value: []string{"192.168.1.1"},
	})

	srv := NewServer(ServerConfig{Listen: ":0", AuthToken: "test-token"}, store)
	return srv.Engine(), store
}

func doRequest(router *gin.Engine, method, path string, body any, token string) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = &bytes.Buffer{}
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func parseResponse(t *testing.T, w *httptest.ResponseRecorder) Response {
	t.Helper()
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v, body: %s", err, w.Body.String())
	}
	return resp
}

// --- Health & Status ---

func TestHealthEndpoint(t *testing.T) {
	router, _ := setupTestRouter(t)
	w := doRequest(router, http.MethodGet, "/health", nil, "")

	if w.Code != 200 {
		t.Fatalf("GET /health status = %d, want 200", w.Code)
	}
	resp := parseResponse(t, w)
	if resp.Code != 0 {
		t.Errorf("response code = %d, want 0", resp.Code)
	}
}

func TestStatusEndpoint(t *testing.T) {
	router, _ := setupTestRouter(t)
	w := doRequest(router, http.MethodGet, "/status", nil, "")

	if w.Code != 200 {
		t.Fatalf("GET /status status = %d, want 200", w.Code)
	}
	resp := parseResponse(t, w)
	if resp.Code != 0 {
		t.Errorf("response code = %d, want 0", resp.Code)
	}
}

// --- Auth Middleware ---

func TestAuthMiddleware_NoToken(t *testing.T) {
	router, _ := setupTestRouter(t)
	w := doRequest(router, http.MethodGet, "/dns/list", nil, "")

	if w.Code != 401 {
		t.Errorf("GET /dns/list without token status = %d, want 401", w.Code)
	}
}

func TestAuthMiddleware_WrongToken(t *testing.T) {
	router, _ := setupTestRouter(t)
	w := doRequest(router, http.MethodGet, "/dns/list", nil, "wrong-token")

	if w.Code != 401 {
		t.Errorf("GET /dns/list with wrong token status = %d, want 401", w.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	router, _ := setupTestRouter(t)
	w := doRequest(router, http.MethodGet, "/dns/list", nil, "test-token")

	if w.Code != 200 {
		t.Errorf("GET /dns/list with valid token status = %d, want 200", w.Code)
	}
}

// --- DNS Add ---

func TestAddRecord(t *testing.T) {
	tests := []struct {
		name       string
		body       AddRecordRequest
		wantStatus int
		wantCode   int
	}{
		{
			name: "add new record",
			body: AddRecordRequest{
				Domain: "new.example.com.",
				Type:   types.RecordTypeA,
				Value:  []string{"10.0.0.1"},
				TTL:    600,
			},
			wantStatus: 200,
			wantCode:   0,
		},
		{
			name: "add duplicate record",
			body: AddRecordRequest{
				Domain: "example.com.",
				Type:   types.RecordTypeA,
				Value:  []string{"10.0.0.2"},
			},
			wantStatus: 409,
			wantCode:   409,
		},
		{
			name: "add record with default TTL",
			body: AddRecordRequest{
				Domain: "ttl.example.com.",
				Type:   types.RecordTypeTXT,
				Value:  []string{"hello"},
			},
			wantStatus: 200,
			wantCode:   0,
		},
		{
			name: "add record with invalid type",
			body: AddRecordRequest{
				Domain: "bad.example.com.",
				Type:   "INVALID",
				Value:  []string{"1.2.3.4"},
			},
			wantStatus: 400,
			wantCode:   400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, _ := setupTestRouter(t)
			w := doRequest(router, http.MethodPost, "/dns/add", tt.body, "test-token")

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", w.Code, tt.wantStatus, w.Body.String())
			}
			resp := parseResponse(t, w)
			if resp.Code != tt.wantCode {
				t.Errorf("response code = %d, want %d", resp.Code, tt.wantCode)
			}
		})
	}
}

// --- DNS Delete ---

func TestDeleteRecord(t *testing.T) {
	tests := []struct {
		name       string
		body       DeleteRecordRequest
		wantStatus int
	}{
		{
			name:       "delete existing record",
			body:       DeleteRecordRequest{Domain: "example.com.", Type: types.RecordTypeA},
			wantStatus: 200,
		},
		{
			name:       "delete non-existent record",
			body:       DeleteRecordRequest{Domain: "notfound.com.", Type: types.RecordTypeA},
			wantStatus: 404,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, _ := setupTestRouter(t)
			w := doRequest(router, http.MethodPost, "/dns/delete", tt.body, "test-token")

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

// --- DNS Update ---

func TestUpdateRecord(t *testing.T) {
	tests := []struct {
		name       string
		body       UpdateRecordRequest
		wantStatus int
	}{
		{
			name: "update existing record",
			body: UpdateRecordRequest{
				Domain: "example.com.",
				Type:   types.RecordTypeA,
				Value:  []string{"10.0.0.1"},
				TTL:    600,
			},
			wantStatus: 200,
		},
		{
			name: "update non-existent record",
			body: UpdateRecordRequest{
				Domain: "notfound.com.",
				Type:   types.RecordTypeA,
				Value:  []string{"10.0.0.1"},
			},
			wantStatus: 404,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, _ := setupTestRouter(t)
			w := doRequest(router, http.MethodPost, "/dns/update", tt.body, "test-token")

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

// --- DNS List ---

func TestListRecords(t *testing.T) {
	router, _ := setupTestRouter(t)
	w := doRequest(router, http.MethodGet, "/dns/list", nil, "test-token")

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	resp := parseResponse(t, w)
	data, ok := resp.Data.([]any)
	if !ok {
		t.Fatalf("data is not a slice: %T", resp.Data)
	}
	if len(data) != 1 {
		t.Errorf("list returned %d records, want 1", len(data))
	}
}

func TestListRecords_Filter(t *testing.T) {
	router, store := setupTestRouter(t)

	// Add a second record.
	_ = store.Create(context.Background(), &types.DNSRecord{
		Name: "other.com.", Type: types.RecordTypeTXT, TTL: 60, Value: []string{"txt"},
	})

	w := doRequest(router, http.MethodGet, "/dns/list?domain=example.com.", nil, "test-token")
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.([]any)
	if len(data) != 1 {
		t.Errorf("filtered list returned %d records, want 1", len(data))
	}
}

// --- DNS Get ---

func TestGetRecord(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "get existing record",
			query:      "/dns/get?domain=example.com.&type=A",
			wantStatus: 200,
		},
		{
			name:       "get non-existent record",
			query:      "/dns/get?domain=notfound.com.&type=A",
			wantStatus: 404,
		},
		{
			name:       "get missing params",
			query:      "/dns/get?domain=example.com.",
			wantStatus: 400,
		},
		{
			name:       "get invalid type",
			query:      "/dns/get?domain=example.com.&type=INVALID",
			wantStatus: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, _ := setupTestRouter(t)
			w := doRequest(router, http.MethodGet, tt.query, nil, "test-token")

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

// --- Invalid JSON ---

func TestAddRecord_InvalidJSON(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/dns/add", bytes.NewBufferString("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

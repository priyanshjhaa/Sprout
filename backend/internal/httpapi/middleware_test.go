package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLoggerRecordsSafeSummary(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	request := httptest.NewRequest(http.MethodGet, "/health/live?token=sensitive-value", nil)
	request.Header.Set("Authorization", "Bearer sensitive-value")
	response := httptest.NewRecorder()

	NewRouter(logger, AlwaysReady).ServeHTTP(response, request)

	logLine := logs.String()
	if !strings.Contains(logLine, `"msg":"request completed"`) {
		t.Fatalf("logs = %q, want request summary", logLine)
	}
	if !strings.Contains(logLine, `"path":"/health/live"`) {
		t.Fatalf("logs = %q, want safe path", logLine)
	}
	if strings.Contains(logLine, "sensitive-value") {
		t.Fatalf("logs expose request secret: %s", logLine)
	}
	if !strings.Contains(logLine, response.Header().Get(requestIDHeader)) {
		t.Fatalf("logs = %q, want response request ID", logLine)
	}
}

func TestRecoveryReturnsSafeError(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	panickingHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("sensitive panic detail")
	})
	handler := requestIDMiddleware(requestLogger(logger)(recoverPanic(logger)(panickingHandler)))
	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if strings.Contains(response.Body.String(), "sensitive panic detail") || strings.Contains(logs.String(), "sensitive panic detail") {
		t.Fatal("panic detail was exposed")
	}

	var body errorEnvelope
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "internal_error" {
		t.Fatalf("error code = %q, want internal_error", body.Error.Code)
	}
	if body.Error.RequestID == "" || body.Error.RequestID != response.Header().Get(requestIDHeader) {
		t.Fatalf("error request ID = %q, header = %q", body.Error.RequestID, response.Header().Get(requestIDHeader))
	}
	if !strings.Contains(logs.String(), body.Error.RequestID) {
		t.Fatalf("logs = %q, want request ID", logs.String())
	}
}

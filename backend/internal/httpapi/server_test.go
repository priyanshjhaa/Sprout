package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type blockingListener struct {
	closed chan struct{}
	once   sync.Once
}

func newBlockingListener() *blockingListener {
	return &blockingListener{closed: make(chan struct{})}
}

func (listener *blockingListener) Accept() (net.Conn, error) {
	<-listener.closed
	return nil, net.ErrClosed
}

func (listener *blockingListener) Close() error {
	listener.once.Do(func() { close(listener.closed) })
	return nil
}

func (listener *blockingListener) Addr() net.Addr {
	return testAddress("127.0.0.1:0")
}

type testAddress string

func (address testAddress) Network() string { return "tcp" }
func (address testAddress) String() string  { return string(address) }

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestLiveness(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	response := httptest.NewRecorder()

	NewRouter(discardLogger(), AlwaysReady).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}

	var body healthResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	expected := healthResponse{Status: "ok", Service: "sprout-api"}
	if body != expected {
		t.Fatalf("response = %#v, want %#v", body, expected)
	}

	if requestID := response.Header().Get(requestIDHeader); requestID == "" {
		t.Fatal("X-Request-ID is empty")
	}
}

func TestLivenessRejectsUnsupportedMethod(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/health/live", nil)
	response := httptest.NewRecorder()

	NewRouter(discardLogger(), AlwaysReady).ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestNewServerSetsTimeouts(t *testing.T) {
	t.Parallel()

	server := NewServer("127.0.0.1:8080", NewRouter(discardLogger(), AlwaysReady))

	if server.ReadHeaderTimeout != readHeaderTimeout {
		t.Fatalf("ReadHeaderTimeout = %s, want %s", server.ReadHeaderTimeout, readHeaderTimeout)
	}
	if server.ReadTimeout != readTimeout {
		t.Fatalf("ReadTimeout = %s, want %s", server.ReadTimeout, readTimeout)
	}
	if server.WriteTimeout != writeTimeout {
		t.Fatalf("WriteTimeout = %s, want %s", server.WriteTimeout, writeTimeout)
	}
	if server.IdleTimeout != idleTimeout {
		t.Fatalf("IdleTimeout = %s, want %s", server.IdleTimeout, idleTimeout)
	}
}

func TestServeStopsAfterCancellation(t *testing.T) {
	t.Parallel()

	listener := newBlockingListener()
	t.Cleanup(func() { _ = listener.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	server := NewServer(listener.Addr().String(), NewRouter(discardLogger(), AlwaysReady))
	serveDone := make(chan error, 1)

	go func() {
		serveDone <- Serve(ctx, listener, server)
	}()

	cancel()

	select {
	case err := <-serveDone:
		if err != nil {
			t.Fatalf("Serve() error = %v, want nil", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve() did not stop after context cancellation")
	}
}

func TestReadinessReportsReady(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	response := httptest.NewRecorder()

	NewRouter(discardLogger(), AlwaysReady).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body healthResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	expected := healthResponse{Status: "ready", Service: "sprout-api"}
	if body != expected {
		t.Fatalf("response = %#v, want %#v", body, expected)
	}
}

func TestReadinessHidesDependencyError(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	response := httptest.NewRecorder()
	readiness := func(context.Context) error {
		return errors.New("database password=sensitive-value")
	}

	NewRouter(discardLogger(), readiness).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if bytes.Contains(response.Body.Bytes(), []byte("sensitive-value")) {
		t.Fatalf("response exposes dependency error: %s", response.Body.String())
	}

	var body errorEnvelope
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "service_not_ready" {
		t.Fatalf("error code = %q, want service_not_ready", body.Error.Code)
	}
	if body.Error.RequestID == "" || body.Error.RequestID != response.Header().Get(requestIDHeader) {
		t.Fatalf("error request ID = %q, header = %q", body.Error.RequestID, response.Header().Get(requestIDHeader))
	}
}

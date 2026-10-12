package runtime

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func endpointOf(server *httptest.Server) string { return strings.TrimPrefix(server.URL, "http://") }

func TestWaitHealthyRetriesUntilOK(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	if err := WaitHealthy(context.Background(), endpointOf(server), 10*time.Second); err != nil || calls.Load() != 3 {
		t.Fatalf("err=%v calls=%d", err, calls.Load())
	}
}

func TestWaitHealthyDoesNotFollowRedirects(t *testing.T) {
	var followed atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { followed.Store(true) }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer server.Close()
	if err := WaitHealthy(context.Background(), endpointOf(server), time.Second); !errors.Is(err, ErrUnhealthy) {
		t.Fatalf("err = %v, want ErrUnhealthy", err)
	}
	if followed.Load() {
		t.Fatal("redirect was followed")
	}
}

func TestWaitHealthyTimesOutAndHonoursCancellation(t *testing.T) {
	if err := WaitHealthy(context.Background(), "127.0.0.1:1", 700*time.Millisecond); !errors.Is(err, ErrUnhealthy) {
		t.Fatalf("err = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := WaitHealthy(ctx, "127.0.0.1:1", time.Minute); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want canceled", err)
	}
}

package httpapi

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/priyanshjhaa/Sprout/backend/internal/deployment"
)

type progressStub struct {
	mu       sync.Mutex
	job      deployment.Job
	err      error
	updates  chan struct{}
	released chan struct{}
}

func newProgressStub() *progressStub {
	return &progressStub{job: deployment.Job{ID: "job-id", Status: "queued", Simulated: true}, updates: make(chan struct{}, 1), released: make(chan struct{}, 1)}
}
func (s *progressStub) Get(_ context.Context, scope deployment.Scope, _ string) (deployment.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if scope.UserID != handlerTestUserID || scope.WorkspaceSlug != "acme" || scope.ApplicationID != "app-id" {
		return deployment.Job{}, errors.New("incorrect scope")
	}
	return s.job, s.err
}
func (s *progressStub) Subscribe(ctx context.Context, scope deployment.Scope, id string) (<-chan struct{}, func(), error) {
	if _, err := s.Get(ctx, scope, id); err != nil {
		return nil, nil, err
	}
	return s.updates, func() { s.released <- struct{}{} }, nil
}
func (s *progressStub) change(status string, err error) {
	s.mu.Lock()
	s.job.Status = status
	s.err = err
	s.mu.Unlock()
	s.updates <- struct{}{}
}
func streamServer(t *testing.T, s *progressStub, shutdown context.Context, expiry int64, lifetime time.Duration) *httptest.Server {
	t.Helper()
	router := NewRouter(discardLogger(), AlwaysReady, "")
	router.Route("/api/v1", func(api chi.Router) {
		api.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				claims := &clerk.SessionClaims{}
				claims.Expiry = &expiry
				ctx := clerk.ContextWithSessionClaims(r.Context(), claims)
				ctx = context.WithValue(ctx, identityContextKey{}, handlerTestUserID)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		h := &simulationStream{service: s, logger: discardLogger(), shutdown: shutdown, heartbeat: 20 * time.Millisecond, lifetime: lifetime, writeTimeout: time.Second}
		api.Get("/workspaces/{workspaceSlug}/applications/{applicationId}/deployment-simulations/{deploymentId}/events", h.serve)
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server
}
func streamGet(t *testing.T, server *httptest.Server, cursor string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("GET", server.URL+simulationPath+"/job-id/events", nil)
	req.Header.Set("Last-Event-ID", cursor)
	client := &http.Client{Timeout: 3 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}
func progressFrame(t *testing.T, r *bufio.Reader, event string) string {
	t.Helper()
	for {
		var frame strings.Builder
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			frame.WriteString(line)
			if line == "\n" {
				break
			}
		}
		if strings.Contains(frame.String(), "event: "+event+"\n") {
			return frame.String()
		}
	}
}
func assertReleased(t *testing.T, s *progressStub) {
	t.Helper()
	select {
	case <-s.released:
	case <-time.After(time.Second):
		t.Fatal("subscription not released")
	}
}
func TestProgressStreamOrderedSnapshotsAndReconnect(t *testing.T) {
	s := newProgressStub()
	server := streamServer(t, s, context.Background(), time.Now().Unix()+60, time.Second)
	res := streamGet(t, server, "")
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("bad response: %v", res)
	}
	reader := bufio.NewReader(res.Body)
	first := progressFrame(t, reader, "progress")
	if !strings.Contains(first, `"status":"queued"`) {
		t.Fatal(first)
	}
	s.change("building", nil)
	second := progressFrame(t, reader, "progress")
	if !strings.Contains(second, `"status":"building"`) {
		t.Fatal(second)
	}
	s.change("succeeded", nil)
	last := progressFrame(t, reader, "progress")
	progressFrame(t, reader, "complete")
	assertReleased(t, s)
	cursor := strings.TrimPrefix(strings.Split(last, "\n")[0], "id: ")
	res = streamGet(t, server, cursor)
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "event: progress") || !strings.Contains(string(body), "event: complete") {
		t.Fatalf("bad reconnect: %s", body)
	}
	assertReleased(t, s)
}
func TestProgressStreamReauthorizationAndCleanup(t *testing.T) {
	for _, reason := range []string{"revoked", "disconnect", "shutdown", "lifetime"} {
		t.Run(reason, func(t *testing.T) {
			s := newProgressStub()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server := streamServer(t, s, ctx, time.Now().Unix()+60, 150*time.Millisecond)
			res := streamGet(t, server, "")
			reader := bufio.NewReader(res.Body)
			progressFrame(t, reader, "progress")
			switch reason {
			case "revoked":
				s.change("building", errors.New("secret database detail"))
				frame := progressFrame(t, reader, "unavailable")
				if strings.Contains(frame, "secret") {
					t.Fatal(frame)
				}
			case "disconnect":
				res.Body.Close()
			case "shutdown":
				cancel()
			}
			assertReleased(t, s)
		})
	}
}
func TestProgressStreamRejectsInvalidRequests(t *testing.T) {
	for _, tc := range []struct {
		name, cursor string
		expiry       int64
		err          error
		status       int
	}{
		{"expired", "", 0, nil, 401},
		{"bad cursor", "bad", time.Now().Unix() + 60, nil, 400},
		{"hidden job", "", time.Now().Unix() + 60, deployment.ErrNotFound, 404},
		{"capacity", "", time.Now().Unix() + 60, deployment.ErrFull, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newProgressStub()
			s.err = tc.err
			server := streamServer(t, s, context.Background(), tc.expiry, time.Second)
			res := streamGet(t, server, tc.cursor)
			if res.StatusCode != tc.status {
				t.Fatalf("status=%d want %d", res.StatusCode, tc.status)
			}
		})
	}
}

type brokenStreamWriter struct {
	*httptest.ResponseRecorder
	deadlines int
}

func (w *brokenStreamWriter) SetWriteDeadline(time.Time) error { w.deadlines++; return nil }
func (w *brokenStreamWriter) Write([]byte) (int, error)        { return 0, io.ErrClosedPipe }

func TestProgressStreamWriteFailureReleasesSubscription(t *testing.T) {
	s := newProgressStub()
	h := &simulationStream{service: s, logger: discardLogger(), shutdown: context.Background(), heartbeat: time.Second, lifetime: time.Second, writeTimeout: time.Second}
	r := httptest.NewRequest("GET", simulationPath+"/job-id/events", nil)
	params := chi.NewRouteContext()
	params.URLParams.Add("workspaceSlug", "acme")
	params.URLParams.Add("applicationId", "app-id")
	params.URLParams.Add("deploymentId", "job-id")
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, params)
	ctx = context.WithValue(ctx, identityContextKey{}, handlerTestUserID)
	expiry := time.Now().Unix() + 60
	claims := &clerk.SessionClaims{}
	claims.Expiry = &expiry
	ctx = clerk.ContextWithSessionClaims(ctx, claims)
	w := &brokenStreamWriter{ResponseRecorder: httptest.NewRecorder()}
	h.serve(w, r.WithContext(ctx))
	assertReleased(t, s)
	if w.deadlines < 2 {
		t.Fatal("write deadline not refreshed")
	}
}

package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/priyanshjhaa/Sprout/backend/internal/deployment"
)

type SimulationProgress interface {
	Get(context.Context, deployment.Scope, string) (deployment.Job, error)
	Subscribe(context.Context, deployment.Scope, string) (<-chan struct{}, func(), error)
}
type simulationStream struct {
	service                           SimulationProgress
	logger                            *slog.Logger
	shutdown                          context.Context
	heartbeat, lifetime, writeTimeout time.Duration
}

var progressCursor = regexp.MustCompile(`^[a-f0-9]{64}$`)

func RegisterDeploymentStreamRoutes(router chi.Router, service SimulationProgress, logger *slog.Logger, shutdown context.Context) {
	h := &simulationStream{service: service, logger: logger, shutdown: shutdown, heartbeat: 10 * time.Second, lifetime: 25 * time.Second, writeTimeout: 5 * time.Second}
	router.Get("/workspaces/{workspaceSlug}/applications/{applicationId}/deployment-simulations/{deploymentId}/events", h.serve)
}
func (h *simulationStream) serve(w http.ResponseWriter, r *http.Request) {
	fail := func(err error) { (&simulationHandler{logger: h.logger}).respond(w, r, nil, err, 0) }
	cursor := r.Header.Get("Last-Event-ID")
	if cursor != "" && !progressCursor.MatchString(cursor) {
		fail(deployment.ErrInvalid)
		return
	}
	claims, ok := clerk.SessionClaimsFromContext(r.Context())
	if !ok || claims == nil || claims.Expiry == nil || *claims.Expiry <= time.Now().Unix() {
		writeAPIError(w, 401, "authentication_required", "A current session with an expiry is required.", requestIDFromContext(r.Context()))
		return
	}
	deadline := time.Now().Add(h.lifetime)
	if expiry := time.Unix(*claims.Expiry, 0); expiry.Before(deadline) {
		deadline = expiry
	}
	ctx, cancel := context.WithDeadline(r.Context(), deadline)
	defer cancel()
	scope, id := simulationScope(r), chi.URLParam(r, "deploymentId")
	lookup, stop := context.WithTimeout(ctx, 5*time.Second)
	updates, release, err := h.service.Subscribe(lookup, scope, id)
	stop()
	if err != nil {
		if errors.Is(err, deployment.ErrFull) {
			w.Header().Set("Retry-After", "3")
			writeAPIError(w, 503, "stream_capacity_reached", "Too many progress streams are open.", requestIDFromContext(r.Context()))
			return
		}
		fail(err)
		return
	}
	defer release()
	// Subscribe first, then read, so an update cannot fall between initial snapshot
	// and registration. Snapshot reads repeat current membership authorization.
	read := func() (deployment.Job, error) {
		lookup, stop := context.WithTimeout(ctx, 5*time.Second)
		defer stop()
		return h.service.Get(lookup, scope, id)
	}
	job, err := read()
	if err != nil {
		fail(err)
		return
	}
	controller := http.NewResponseController(w)
	if err = controller.SetWriteDeadline(time.Now().Add(h.writeTimeout)); err != nil {
		fail(deployment.ErrUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	// Per-frame deadlines protect slow clients without disabling HTTP timeouts.
	write := func(frame string) error {
		if err := controller.SetWriteDeadline(time.Now().Add(h.writeTimeout)); err != nil {
			return err
		}
		if _, err := fmt.Fprint(w, frame); err != nil {
			return err
		}
		return controller.Flush()
	}
	snapshot := func(job deployment.Job) (bool, error) {
		if ctx.Err() != nil || h.shutdown.Err() != nil {
			return true, ctx.Err()
		}
		payload, err := json.Marshal(job)
		if err != nil {
			return true, err
		}
		sum := sha256.Sum256(payload)
		next := hex.EncodeToString(sum[:])
		if next != cursor {
			if err = write(fmt.Sprintf("id: %s\nevent: progress\ndata: %s\n\n", next, payload)); err != nil {
				return true, err
			}
			cursor = next
		}
		terminal := job.Status == "succeeded" || job.Status == "failed" || job.Status == "cancelled"
		if terminal {
			return true, write("event: complete\ndata: {}\n\n")
		}
		return false, nil
	}
	if err = write("retry: 1000\n\n"); err != nil {
		return
	}
	if done, err := snapshot(job); done || err != nil {
		return
	}
	heartbeat := time.NewTicker(h.heartbeat)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return // reconnect authenticates again with a fresh token
		case <-h.shutdown.Done():
			return
		case _, open := <-updates:
			if !open {
				return
			}
		case <-heartbeat.C:
		}
		job, err = read()
		if err != nil {
			// Never send stale progress after membership removal or dependency failure.
			_ = write("event: unavailable\ndata: {\"code\":\"progress_unavailable\"}\n\n")
			return
		}
		if done, err := snapshot(job); done || err != nil {
			return
		}
		if err = write(": heartbeat\n\n"); err != nil {
			return
		}
	}
}

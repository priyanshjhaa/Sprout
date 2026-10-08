package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/priyanshjhaa/Sprout/backend/internal/deployment"
	"github.com/priyanshjhaa/Sprout/backend/internal/source"
)

type intakeStub struct {
	checkErr  error
	submitted []byte
	scope     deployment.Scope
}

func (s *intakeStub) Check(_ context.Context, scope deployment.Scope) error {
	s.scope = scope
	return s.checkErr
}
func (s *intakeStub) Submit(_ context.Context, scope deployment.Scope, archive []byte) (deployment.Job, error) {
	s.scope, s.submitted = scope, archive
	return deployment.Job{ID: "job-id", ApplicationID: "app-id", Status: "queued", Stages: []deployment.Stage{}}, nil
}

// readTracker records whether the handler consumed the request body.
type readTracker struct {
	io.Reader
	read bool
}

func (r *readTracker) Read(p []byte) (int, error) {
	r.read = true
	return r.Reader.Read(p)
}

func buildRouter(intake BuildIntake) *chi.Mux {
	router := NewRouter(discardLogger(), AlwaysReady, "")
	router.Route("/api/v1", func(api chi.Router) {
		api.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityContextKey{}, handlerTestUserID)))
			})
		})
		RegisterDeploymentRoutes(api, simulationStub{}, intake, discardLogger())
	})
	return router
}

const buildPath = "/api/v1/workspaces/acme/applications/app-id/deployments"

func TestBuildUploadContract(t *testing.T) {
	upload := func(router http.Handler, contentType string, body io.Reader) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, buildPath, body)
		request.Header.Set("Content-Type", contentType)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}

	accepted := &intakeStub{}
	response := upload(buildRouter(accepted), "application/x-tar", strings.NewReader("tar bytes"))
	if response.Code != http.StatusCreated || response.Header().Get("Location") != buildPath+"/job-id" {
		t.Fatalf("upload: %d %s", response.Code, response.Header().Get("Location"))
	}
	if string(accepted.submitted) != "tar bytes" || accepted.scope.Simulated || accepted.scope.UserID != handlerTestUserID {
		t.Fatalf("submitted %q with scope %+v", accepted.submitted, accepted.scope)
	}

	if response := upload(buildRouter(&intakeStub{}), "application/json", strings.NewReader("{}")); response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("wrong media type: %d", response.Code)
	}

	// A refused request is answered before the upload is read.
	body := &readTracker{Reader: strings.NewReader("tar bytes")}
	response = upload(buildRouter(&intakeStub{checkErr: deployment.ErrForbidden}), "application/x-tar", body)
	if response.Code != http.StatusForbidden || body.read {
		t.Fatalf("refused upload: %d, body read %v", response.Code, body.read)
	}

	tooLarge := &intakeStub{}
	response = upload(buildRouter(tooLarge), "application/x-tar", bytes.NewReader(make([]byte, source.MaxArchiveBytes+1)))
	if response.Code != http.StatusRequestEntityTooLarge || !strings.Contains(response.Body.String(), "source_too_large") || tooLarge.submitted != nil {
		t.Fatalf("oversized upload: %d %s", response.Code, response.Body.String())
	}
}

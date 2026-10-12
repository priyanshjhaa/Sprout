package runtime

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestProxy(t *testing.T, route Route, lookupErr error) (*Proxy, *scriptedDriver) {
	t.Helper()
	driver := newScripted(healthyServer(t))
	s := newSupervisor(t, driver, staticArtifacts{}, 3)
	lookup := func(_ context.Context, workspace, app string) (Route, error) {
		if lookupErr != nil || workspace != "acme" || app != "shop" {
			if lookupErr == nil {
				return Route{}, ErrNoApp
			}
			return Route{}, lookupErr
		}
		return route, nil
	}
	p, err := NewProxy("localhost", lookup, s, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return p, driver
}

func get(p *Proxy, host string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "http://"+host+"/", nil)
	request.Host = host
	for k, v := range headers {
		request.Header.Set(k, v)
	}
	recorder := httptest.NewRecorder()
	p.ServeHTTP(recorder, request)
	return recorder
}

func TestHostParsingIsStrict(t *testing.T) {
	p, _ := newTestProxy(t, testRoute, nil)
	for host, want := range map[string]bool{
		"shop.acme.localhost":      true,
		"SHOP.Acme.localhost:8081": true,
		"shop.acme.localhost.":     true,
		"acme.localhost":           false, // one label
		"a.shop.acme.localhost":    false, // three labels
		"shop.acme.example.com":    false, // wrong domain
		"shop.acme.evil.localhost": false,
		"-shop.acme.localhost":     false,
		"sh_op.acme.localhost":     false,
		"localhost":                false,
		"":                         false,
		strings.Repeat("a", 64) + ".acme.localhost": false,
	} {
		if _, _, ok := p.parseHost(host); ok != want {
			t.Errorf("parseHost(%q) = %v, want %v", host, ok, want)
		}
	}
}

func TestRoutingStatuses(t *testing.T) {
	for _, tc := range []struct {
		name  string
		route Route
		host  string
		want  int
	}{
		{"live app", testRoute, "shop.acme.localhost", http.StatusOK},
		{"unknown app", testRoute, "nope.acme.localhost", http.StatusNotFound},
		{"foreign host", testRoute, "shop.acme.example.com", http.StatusNotFound},
		{"not deployed", Route{Release: testRelease, Lifecycle: "active"}, "shop.acme.localhost", http.StatusNotFound},
		{"paused", Route{Release: testRelease, Lifecycle: "paused", Live: true}, "shop.acme.localhost", http.StatusServiceUnavailable},
		{"archived", Route{Release: testRelease, Lifecycle: "archived", Live: true}, "shop.acme.localhost", http.StatusGone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, driver := newTestProxy(t, tc.route, nil)
			if got := get(p, tc.host, nil).Code; got != tc.want {
				t.Fatalf("status = %d, want %d", got, tc.want)
			}
			if tc.want != http.StatusOK && driver.starts.Load() != 0 {
				t.Fatal("a refused request woke the application")
			}
		})
	}
}

func TestForwardingStripsSpoofableHeaders(t *testing.T) {
	p, _ := newTestProxy(t, testRoute, nil)
	body := get(p, "shop.acme.localhost:8081", map[string]string{"X-Sprout-Admin": "1", "X-Forwarded-Host": "evil.example"}).Body.String()
	if !strings.Contains(body, "host=shop.acme.localhost:8081") {
		t.Errorf("original Host not preserved: %s", body)
	}
	if !strings.Contains(body, "fwd=shop.acme.localhost:8081") {
		t.Errorf("client-supplied X-Forwarded-Host was trusted: %s", body)
	}
	if !strings.Contains(body, "sprout=\n") && !strings.HasSuffix(body, "sprout=") {
		t.Errorf("X-Sprout-* header reached the application: %s", body)
	}
}

func TestLookupFailureAndBadGateway(t *testing.T) {
	p, _ := newTestProxy(t, testRoute, context.DeadlineExceeded)
	if got := get(p, "shop.acme.localhost", nil).Code; got != http.StatusServiceUnavailable {
		t.Fatalf("lookup failure status = %d", got)
	}
	// An app that cannot start yields 502 and does not leak error details.
	driver := newScripted("127.0.0.1:1")
	driver.startErr = ErrStart
	s := newSupervisor(t, driver, staticArtifacts{}, 2)
	bad, _ := NewProxy("localhost", func(context.Context, string, string) (Route, error) { return testRoute, nil }, s, slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := get(bad, "shop.acme.localhost", nil)
	if recorder.Code != http.StatusBadGateway || strings.Contains(recorder.Body.String(), "runtime_") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

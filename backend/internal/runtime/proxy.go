package runtime

import (
	"context"
	"errors"
	"html"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	maxRequestBody = 10 << 20
	// Server timeouts for the public listener. Applications are untrusted and
	// visitors anonymous, so nothing may hold a connection open indefinitely.
	ProxyReadHeaderTimeout = 5 * time.Second
	ProxyReadTimeout       = 30 * time.Second
	ProxyWriteTimeout      = 60 * time.Second
	ProxyIdleTimeout       = 60 * time.Second
)

var slugLabel = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Proxy serves live applications on their public hostnames. A request for
// {application}.{workspace}.{domain} is routed to that application's live
// release, which the Supervisor starts if it is asleep. Everything the
// application sends back is untrusted; everything it receives passes through
// the standard reverse-proxy header hygiene.
type Proxy struct {
	domain     string
	lookup     Lookup
	supervisor *Supervisor
	sites      *siteCache
	logger     *slog.Logger
}

func NewProxy(domain string, lookup Lookup, supervisor *Supervisor, artifacts ArtifactLoader, logger *slog.Logger) (*Proxy, error) {
	domain = strings.ToLower(strings.Trim(domain, "."))
	if domain == "" || lookup == nil || supervisor == nil || artifacts == nil || logger == nil {
		return nil, ErrInvalid
	}
	return &Proxy{domain: domain, lookup: lookup, supervisor: supervisor, sites: newSiteCache(artifacts), logger: logger}, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	application, workspace, ok := p.parseHost(r.Host)
	if !ok {
		notice(w, http.StatusNotFound, "Not found", "There is no application at this address.")
		return
	}
	route, err := p.lookup(r.Context(), workspace, application)
	switch {
	case errors.Is(err, ErrNoApp):
		notice(w, http.StatusNotFound, "Not found", "There is no application at this address.")
		return
	case err != nil:
		p.logger.ErrorContext(r.Context(), "route lookup failed")
		notice(w, http.StatusServiceUnavailable, "Unavailable", "Sprout could not look up this application. Try again shortly.")
		return
	}
	switch {
	case route.Lifecycle == "archived":
		notice(w, http.StatusGone, "Archived", "This application has been archived.")
		return
	case route.Lifecycle == "paused":
		notice(w, http.StatusServiceUnavailable, "Paused", "This application is paused by its owners.")
		return
	case !route.Live:
		notice(w, http.StatusNotFound, "Not deployed", "This application has not been deployed yet.")
		return
	}
	if route.Runtime == RuntimeStatic {
		p.serveStatic(w, r, route)
		return
	}
	endpoint, err := p.supervisor.Ensure(r.Context(), route)
	switch {
	case errors.Is(err, ErrCapacity):
		w.Header().Set("Retry-After", "10")
		notice(w, http.StatusServiceUnavailable, "Busy", "Sprout is running as many applications as it can. Try again shortly.")
		return
	case r.Context().Err() != nil:
		return // the visitor left; the start continues for the next one
	case err != nil:
		p.logger.ErrorContext(r.Context(), "application could not start", "deployment_id", route.Release.DeploymentID)
		notice(w, http.StatusBadGateway, "Application unavailable", "This application could not be started.")
		return
	}
	p.forward(w, r, route, endpoint)
}

// parseHost accepts exactly {application}.{workspace}.{domain}[:port] with
// strict slug labels; anything else is not ours to route.
func (p *Proxy) parseHost(hostport string) (application, workspace string, ok bool) {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	prefix, found := strings.CutSuffix(host, "."+p.domain)
	if !found {
		return "", "", false
	}
	labels := strings.Split(prefix, ".")
	if len(labels) != 2 {
		return "", "", false
	}
	for _, label := range labels {
		if len(label) > 63 || !slugLabel.MatchString(label) {
			return "", "", false
		}
	}
	return labels[0], labels[1], true
}

func (p *Proxy) forward(w http.ResponseWriter, r *http.Request, route Route, endpoint string) {
	target := &url.URL{Scheme: "http", Host: endpoint}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(out *httputil.ProxyRequest) {
			out.SetURL(target)
			out.Out.Host = out.In.Host
			out.SetXForwarded()
			for name := range out.Out.Header {
				if strings.HasPrefix(strings.ToLower(name), "x-sprout-") {
					out.Out.Header.Del(name)
				}
			}
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if errors.Is(err, context.Canceled) {
				return
			}
			// The instance stopped answering: forget it so the next request restarts it.
			p.supervisor.Failed(route.Release)
			notice(w, http.StatusBadGateway, "Application unavailable", "This application did not respond.")
		},
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	proxy.ServeHTTP(w, r)
}

// NewServer returns the public listener's server with conservative timeouts.
func (p *Proxy) NewServer(address string) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           p,
		ReadHeaderTimeout: ProxyReadHeaderTimeout,
		ReadTimeout:       ProxyReadTimeout,
		WriteTimeout:      ProxyWriteTimeout,
		IdleTimeout:       ProxyIdleTimeout,
	}
}

func notice(w http.ResponseWriter, status int, title, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write([]byte("<!doctype html><meta charset=utf-8><title>" + html.EscapeString(title) + " · Sprout</title>" +
		"<body style=\"font:16px system-ui;max-width:32rem;margin:20vh auto;padding:0 1rem\"><h1>" + html.EscapeString(title) +
		"</h1><p>" + html.EscapeString(message) + "</p>"))
}

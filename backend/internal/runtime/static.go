package runtime

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/staticsite"
)

// maxCachedSites bounds memory: a site is at most the source limit (8 MiB), so
// the cache never holds much more than 64 MiB of file data.
const maxCachedSites = 8

// siteCache keeps recently served static sites in memory. A release is
// immutable, so a cached site never goes stale: entries are keyed by
// deployment ID and only ever evicted, never invalidated.
type siteCache struct {
	artifacts ArtifactLoader
	mu        sync.Mutex
	entries   map[string]*siteEntry
	order     []string // least recently used first
}

type siteEntry struct {
	ready chan struct{}
	site  *staticsite.Site
	err   error
}

func newSiteCache(artifacts ArtifactLoader) *siteCache {
	return &siteCache{artifacts: artifacts, entries: map[string]*siteEntry{}}
}

// get returns the release's site, loading it from the artifact store at most
// once at a time. The load's own context is independent of any one visitor.
func (c *siteCache) get(ctx context.Context, route Route) (*staticsite.Site, error) {
	key := route.Release.DeploymentID
	c.mu.Lock()
	entry := c.entries[key]
	if entry == nil {
		entry = &siteEntry{ready: make(chan struct{})}
		c.entries[key] = entry
		go c.load(route, entry)
	}
	c.touch(key)
	c.mu.Unlock()
	select {
	case <-entry.ready:
		return entry.site, entry.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *siteCache) load(route Route, entry *siteEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	archive, err := c.artifacts.Load(ctx, route.Artifact)
	if err == nil {
		entry.site, err = staticsite.Load(archive)
	}
	entry.err = err
	close(entry.ready)
	if err != nil {
		// Do not cache failures: a transient storage error should heal.
		c.mu.Lock()
		if c.entries[route.Release.DeploymentID] == entry {
			delete(c.entries, route.Release.DeploymentID)
			c.drop(route.Release.DeploymentID)
		}
		c.mu.Unlock()
	}
}

func (c *siteCache) touch(key string) {
	c.drop(key)
	c.order = append(c.order, key)
	for len(c.order) > maxCachedSites {
		delete(c.entries, c.order[0])
		c.order = c.order[1:]
	}
}

func (c *siteCache) drop(key string) {
	for i, existing := range c.order {
		if existing == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}

// serveStatic answers a request for a static release straight from its stored
// artifact: no container, no volume, nothing executes. The site is untrusted
// content, so the rules are narrow: GET and HEAD only, exact canonical paths,
// content types from Sprout's own table, and nosniff so a browser cannot
// reinterpret a file as something riskier than Sprout declared.
func (p *Proxy) serveStatic(w http.ResponseWriter, r *http.Request, route Route) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		notice(w, http.StatusMethodNotAllowed, "Method not allowed", "This site only answers GET and HEAD requests.")
		return
	}
	site, err := p.sites.get(r.Context(), route)
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		p.logger.ErrorContext(r.Context(), "static site unavailable", "deployment_id", route.Release.DeploymentID)
		notice(w, http.StatusBadGateway, "Site unavailable", "This site could not be loaded.")
		return
	}
	file, found := site.Lookup(r.URL.Path)
	status := http.StatusOK
	if !found {
		// A site may bring its own 404 page; otherwise Sprout's plain one.
		file, found = site.Lookup("/404.html")
		if !found {
			notice(w, http.StatusNotFound, "Not found", "There is nothing at this address.")
			return
		}
		status = http.StatusNotFound
	}
	header := w.Header()
	header.Set("Content-Type", file.ContentType)
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Cache-Control", "no-cache") // revalidate with the ETag; a redeploy shows up at once
	if status == http.StatusNotFound {
		header.Set("Content-Length", strconv.Itoa(len(file.Data)))
		w.WriteHeader(status)
		if r.Method == http.MethodGet {
			_, _ = w.Write(file.Data)
		}
		return
	}
	header.Set("ETag", file.ETag)
	// ServeContent handles HEAD, conditional requests and byte ranges.
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(file.Data))
}

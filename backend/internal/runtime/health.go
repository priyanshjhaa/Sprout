package runtime

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

var ErrUnhealthy = errors.New("runtime_unhealthy")

const (
	healthInterval = 500 * time.Millisecond
	healthBodyMax  = 4 << 10
)

// WaitHealthy polls http://endpoint/health until it answers 200, the context
// ends, or timeout elapses. The application is untrusted, so the client never
// follows redirects (they could point at other local services) and reads at
// most a few KiB of any response.
func WaitHealthy(ctx context.Context, endpoint string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client := &http.Client{
		Timeout:       2 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			if parent := ctx.Err(); errors.Is(parent, context.Canceled) {
				return parent
			}
			return ErrUnhealthy
		case <-timer.C:
		}
		if probe(ctx, client, endpoint) {
			return nil
		}
		timer.Reset(healthInterval)
	}
}

func probe(ctx context.Context, client *http.Client, endpoint string) bool {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+endpoint+"/health", nil)
	if err != nil {
		return false
	}
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, healthBodyMax))
	return response.StatusCode == http.StatusOK
}

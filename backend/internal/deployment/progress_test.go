package deployment

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestWorkerPublishesPersistedCompletion(t *testing.T) {
	repo := newMemoryRepository()
	entered, proceed := make(chan struct{}, 1), make(chan struct{})
	m := managerForTest(t, repo, func(ctx context.Context, stage string) error {
		if stage == "source" {
			entered <- struct{}{}
			select {
			case <-proceed:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}, 1, 1, time.Second)
	job := submit(t, m, "app")
	waitSignal(t, entered)
	updates, release, err := m.Subscribe(context.Background(), Scope{}, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	close(proceed)
	for {
		waitSignal(t, updates)
		snapshot, err := m.Get(context.Background(), Scope{}, job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Status == "succeeded" {
			break
		}
	}
}

func TestProgressBrokerBoundsAndCleanup(t *testing.T) {
	b := newProgressBroker()
	var releases []func()
	for i := 0; i < 128; i++ {
		ch, release, err := b.subscribe("job", fmt.Sprint(i/4))
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
		b.notify("job")
		b.notify("job")
		if len(ch) != 1 {
			t.Fatal("notifications must coalesce")
		}
	}
	if _, _, err := b.subscribe("job", "extra"); !errors.Is(err, ErrFull) {
		t.Fatal("global cap missing")
	}
	releases[0]()
	if _, _, err := b.subscribe("job", "1"); !errors.Is(err, ErrFull) {
		t.Fatal("user cap missing")
	}
	for _, release := range releases {
		release()
		release()
	}
	if b.count != 0 || len(b.users) != 0 || len(b.subscribers) != 0 {
		t.Fatal("subscription leaked")
	}
}

func TestProgressBrokerConcurrentRelease(t *testing.T) {
	b := newProgressBroker()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, release, err := b.subscribe("job", fmt.Sprint(i))
				if err != nil {
					t.Error(err)
					return
				}
				b.notify("job")
				release()
			}
		}(i)
	}
	wg.Wait()
	if b.count != 0 {
		t.Fatal("subscription leaked")
	}
}

package deployment

import (
	"context"
	"sync"
)

// Notifications carry no data: subscribers re-read an authorized DB snapshot.
// One pending notification is enough; slow clients never block a worker.
type progressBroker struct {
	mu          sync.Mutex
	subscribers map[string]map[chan struct{}]string
	users       map[string]int
	count       int
}

func newProgressBroker() *progressBroker {
	return &progressBroker{subscribers: map[string]map[chan struct{}]string{}, users: map[string]int{}}
}
func (b *progressBroker) subscribe(id, user string) (<-chan struct{}, func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.count >= 128 || b.users[user] >= 4 {
		return nil, nil, ErrFull
	}
	updates := make(chan struct{}, 1)
	if b.subscribers[id] == nil {
		b.subscribers[id] = map[chan struct{}]string{}
	}
	b.subscribers[id][updates] = user
	b.users[user]++
	b.count++
	release := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if _, ok := b.subscribers[id][updates]; !ok {
			return
		}
		delete(b.subscribers[id], updates)
		if len(b.subscribers[id]) == 0 {
			delete(b.subscribers, id)
		}
		b.users[user]--
		if b.users[user] == 0 {
			delete(b.users, user)
		}
		b.count--
		close(updates)
	}
	return updates, release, nil
}
func (b *progressBroker) notify(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subscribers[id] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
func (m *Manager) Subscribe(ctx context.Context, scope Scope, id string) (<-chan struct{}, func(), error) {
	if _, err := m.repository.Get(ctx, scope, id); err != nil {
		return nil, nil, err
	}
	return m.progress.subscribe(id, scope.UserID)
}

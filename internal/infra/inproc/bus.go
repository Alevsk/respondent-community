package inproc

import (
	"context"
	"strings"
	"sync"
)

// Bus is a simple in-process publish/subscribe broker.
// Subject matching follows NATS token semantics: a subscriber whose filter ends
// in the ">" token matches any subject sharing the preceding token prefix, and
// "*" matches exactly one token. Subjects without wildcards match exactly.
type Bus struct {
	mu         sync.RWMutex
	exact      map[string][]chan []byte
	wildcard   []wildcardSub
	bufferSize int
}

type wildcardSub struct {
	filter string
	ch     chan []byte
}

// NewBus creates a new Bus with the given per-subscriber channel buffer size.
func NewBus(bufferSize int) *Bus {
	return &Bus{
		exact:      make(map[string][]chan []byte),
		bufferSize: bufferSize,
	}
}

// isWildcard reports whether a filter uses NATS token wildcards.
func isWildcard(filter string) bool {
	return strings.HasSuffix(filter, ">") || strings.Contains(filter, "*")
}

// SubjectMatches reports whether a concrete subject matches a subscription
// filter using NATS token rules: "*" matches exactly one token; a trailing ">"
// matches one or more remaining tokens.
func SubjectMatches(filter, subject string) bool {
	if filter == subject {
		return true
	}
	ft := strings.Split(filter, ".")
	st := strings.Split(subject, ".")
	for i, tok := range ft {
		if tok == ">" {
			return i == len(ft)-1 && i < len(st)
		}
		if i >= len(st) {
			return false
		}
		if tok == "*" {
			continue
		}
		if tok != st[i] {
			return false
		}
	}
	return len(ft) == len(st)
}

// subscribe registers a new channel for the given filter and returns it.
func (b *Bus) subscribe(filter string) chan []byte {
	ch := make(chan []byte, b.bufferSize)
	b.mu.Lock()
	if isWildcard(filter) {
		b.wildcard = append(b.wildcard, wildcardSub{filter: filter, ch: ch})
	} else {
		b.exact[filter] = append(b.exact[filter], ch)
	}
	b.mu.Unlock()
	return ch
}

// unsubscribe removes a specific channel registered under filter.
func (b *Bus) unsubscribe(filter string, ch chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if isWildcard(filter) {
		for i, ws := range b.wildcard {
			if ws.ch == ch {
				b.wildcard = append(b.wildcard[:i], b.wildcard[i+1:]...)
				break
			}
		}
		return
	}
	subs := b.exact[filter]
	for i, sub := range subs {
		if sub == ch {
			b.exact[filter] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(b.exact[filter]) == 0 {
		delete(b.exact, filter)
	}
}

// publish sends data to all subscribers whose filter matches subject.
// Blocks if a subscriber's buffer is full (backpressure), unless ctx is canceled.
func (b *Bus) publish(ctx context.Context, subject string, data []byte) error {
	b.mu.RLock()
	subs := make([]chan []byte, len(b.exact[subject]))
	copy(subs, b.exact[subject])
	for _, ws := range b.wildcard {
		if SubjectMatches(ws.filter, subject) {
			subs = append(subs, ws.ch)
		}
	}
	b.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- data:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Package events is the SSE broker for user-facing live updates
// (dossier 04): worker_joined | worker_left | metrics_tick | miner_status |
// payout_detected | block_found. Proxy-friendly (nginx proxy_buffering off
// on /v1/events), auto-reconnecting via the browser's EventSource.
package events

import (
	"encoding/json"
	"sync"
)

// Event is one SSE frame.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// Broker fans events out to per-user subscribers. Max 5 concurrent
// connections per user (dossier 04 rate limits).
type Broker struct {
	mu   sync.Mutex
	subs map[string]map[chan Event]struct{} // owner -> channels
}

func New() *Broker { return &Broker{subs: map[string]map[chan Event]struct{}{}} }

const maxConnsPerUser = 5

// Subscribe registers a buffered channel for owner; the returned func
// unsubscribes. ok=false when the per-user connection cap is reached.
func (b *Broker) Subscribe(owner string) (ch chan Event, unsubscribe func(), ok bool) {
	ch = make(chan Event, 32)
	b.mu.Lock()
	defer b.mu.Unlock()
	set := b.subs[owner]
	if set == nil {
		set = map[chan Event]struct{}{}
		b.subs[owner] = set
	}
	if len(set) >= maxConnsPerUser {
		return nil, func() {}, false
	}
	set[ch] = struct{}{}
	return ch, func() {
		b.mu.Lock()
		delete(b.subs[owner], ch)
		if len(b.subs[owner]) == 0 {
			delete(b.subs, owner)
		}
		b.mu.Unlock()
		close(ch)
	}, true
}

// Publish delivers to all of owner's subscribers (non-blocking; slow
// consumers drop frames — the next metrics_tick repaints anyway).
func (b *Broker) Publish(owner string, e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[owner] {
		select {
		case ch <- e:
		default:
		}
	}
}

// PublishAll delivers to every subscriber (used by the metrics tick loop,
// which iterates owners itself).
func (b *Broker) PublishAll(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, set := range b.subs {
		for ch := range set {
			select {
			case ch <- e:
			default:
			}
		}
	}
}

// Marshal renders the SSE wire format for one event.
func Marshal(e Event) ([]byte, error) {
	payload, err := json.Marshal(e.Data)
	if err != nil {
		return nil, err
	}
	out := "event: " + e.Type + "\ndata: " + string(payload) + "\n\n"
	return []byte(out), nil
}

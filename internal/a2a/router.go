package a2a

import (
	"fmt"
	"sync"
	"time"
)

type Message struct {
	ID        string
	From      string
	To        string
	Type      string
	Payload   map[string]any
	Timestamp time.Time
}

type Router struct {
	socketPath string

	mu          sync.RWMutex
	running     bool
	subscribers map[string]map[chan Message]struct{}
}

func NewRouter(socketPath string) *Router {
	return &Router{
		socketPath:  socketPath,
		subscribers: make(map[string]map[chan Message]struct{}),
	}
}

func (r *Router) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.socketPath == "" {
		return fmt.Errorf("a2a socket path required")
	}
	r.running = true
	return nil
}

func (r *Router) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = false
	for _, set := range r.subscribers {
		for ch := range set {
			close(ch)
		}
	}
	r.subscribers = make(map[string]map[chan Message]struct{})
	return nil
}

func (r *Router) Running() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

func (r *Router) SocketPath() string {
	return r.socketPath
}

func (r *Router) Subscribe(target string, buffer int) (<-chan Message, func(), error) {
	if target == "" {
		return nil, nil, fmt.Errorf("subscription target required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.running {
		return nil, nil, fmt.Errorf("a2a router not running")
	}
	if buffer <= 0 {
		buffer = 1
	}
	ch := make(chan Message, buffer)
	if _, ok := r.subscribers[target]; !ok {
		r.subscribers[target] = make(map[chan Message]struct{})
	}
	r.subscribers[target][ch] = struct{}{}
	unsubscribe := func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		set, ok := r.subscribers[target]
		if !ok {
			return
		}
		if _, exists := set[ch]; !exists {
			return
		}
		delete(set, ch)
		close(ch)
		if len(set) == 0 {
			delete(r.subscribers, target)
		}
	}
	return ch, unsubscribe, nil
}

func (r *Router) Publish(msg Message) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.running {
		return fmt.Errorf("a2a router not running")
	}
	if msg.To == "" {
		return fmt.Errorf("a2a target required")
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	dispatch := func(target string) {
		set := r.subscribers[target]
		for ch := range set {
			select {
			case ch <- msg:
			default:
			}
		}
	}
	dispatch(msg.To)
	if msg.To != "broadcast" {
		dispatch("broadcast")
	}
	return nil
}

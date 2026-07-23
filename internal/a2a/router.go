package a2a

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

type Message struct {
	ID        string         `json:"id"`
	From      string         `json:"from"`
	To        string         `json:"to"`
	Type      string         `json:"type"`
	Payload   map[string]any `json:"payload"`
	Timestamp time.Time      `json:"timestamp"`
}

type Router struct {
	socketPath string

	mu          sync.RWMutex
	running     bool
	subscribers map[string]map[chan Message]struct{}
	listener    net.Listener
	conns       map[net.Conn]struct{}
	wg          sync.WaitGroup
}

func NewRouter(socketPath string) *Router {
	return &Router{
		socketPath:  socketPath,
		subscribers: make(map[string]map[chan Message]struct{}),
		conns:       make(map[net.Conn]struct{}),
	}
}

func (r *Router) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.socketPath == "" {
		return fmt.Errorf("a2a socket path required")
	}
	if r.running {
		return nil
	}

	listener, err := listenSocket(r.socketPath)
	if err != nil {
		return err
	}

	r.listener = listener
	r.running = true
	r.wg.Add(1)
	go r.acceptLoop(listener)
	return nil
}

func (r *Router) Stop() error {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return nil
	}
	r.running = false
	listener := r.listener
	r.listener = nil
	conns := make([]net.Conn, 0, len(r.conns))
	for conn := range r.conns {
		conns = append(conns, conn)
	}
	r.conns = make(map[net.Conn]struct{})
	for _, set := range r.subscribers {
		for ch := range set {
			close(ch)
		}
	}
	r.subscribers = make(map[string]map[chan Message]struct{})
	r.mu.Unlock()

	if listener != nil {
		_ = listener.Close()
	}
	for _, conn := range conns {
		_ = conn.Close()
	}
	r.wg.Wait()

	if err := os.Remove(r.socketPath); err != nil && !os.IsNotExist(err) {
		return err
	}
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

func (r *Router) acceptLoop(listener net.Listener) {
	defer r.wg.Done()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			r.mu.RLock()
			running := r.running
			r.mu.RUnlock()
			if !running {
				return
			}
			continue
		}
		r.mu.Lock()
		if !r.running {
			r.mu.Unlock()
			_ = conn.Close()
			return
		}
		r.conns[conn] = struct{}{}
		r.wg.Add(1)
		r.mu.Unlock()
		go r.handleConn(conn)
	}
}

func (r *Router) handleConn(conn net.Conn) {
	defer r.wg.Done()
	defer func() {
		_ = conn.Close()
		r.mu.Lock()
		delete(r.conns, conn)
		r.mu.Unlock()
	}()

	decoder := json.NewDecoder(conn)
	for {
		var msg Message
		if err := decoder.Decode(&msg); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return
			}
			return
		}
		_ = r.Publish(msg)
	}
}

package adr

import (
	"context"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watch starts watching a directory for file changes and returns a channel
// that receives a value on each change (debounced by 200ms). The caller
// should call cancel to stop watching and release resources. Returns an
// error if the watcher cannot be created.
func Watch(dir string) (<-chan struct{}, context.CancelFunc, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil, err
	}
	if err := w.Add(dir); err != nil {
		w.Close()
		return nil, nil, err
	}

	ch := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer w.Close()
		defer close(ch)

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		var debounce *time.Timer
		emit := func() {
			select {
			case ch <- struct{}{}:
			default:
			}
		}

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				emit()

			case _, ok := <-w.Events:
				if !ok {
					return
				}
				if debounce != nil {
					debounce.Stop()
				}
				debounce = time.AfterFunc(200*time.Millisecond, emit)

			case <-w.Errors:
				// ignore errors, fallback ticker ensures we don't miss changes
			}
		}
	}()

	return ch, cancel, nil
}

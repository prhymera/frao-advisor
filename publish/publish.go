// Package publish provides a best-effort HTTP client that MCP processes use
// to publish advice events to the standalone frao-advisor dashboard.
//
// Publishing is deliberately non-blocking and never fails the caller: if the
// dashboard is down or unreachable, events are logged and dropped. The MCP's
// main task — returning advice — must never depend on the dashboard.
package publish

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/prhymera/frao-advisor/db"
)

var (
	// retryDelay is the pause between publish attempts.
	retryDelay = 500 * time.Millisecond
	// requestTimeout bounds each HTTP call.
	requestTimeout = 3 * time.Second
	// maxAttempts is the number of POST attempts per event.
	maxAttempts = 2
)

// Publisher posts db.Events to a dashboard service. A nil *Publisher is a
// no-op, so callers can pass nil to disable publishing.
type Publisher struct {
	url    string
	client *http.Client
	wg     sync.WaitGroup
}

// New returns a disabled (nil) publisher when url is empty.
func New(url string) *Publisher {
	if url == "" {
		return nil
	}
	return &Publisher{
		url:    url,
		client: &http.Client{Timeout: requestTimeout},
	}
}

// Publish fire-and-forget POSTs ev to the dashboard. It never blocks or errors
// the caller; all failures are logged. Safe to call from a single goroutine.
func (p *Publisher) Publish(ev db.Event) {
	if p == nil {
		return
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.send(ev)
	}()
}

func (p *Publisher) send(ev db.Event) {
	body, err := json.Marshal(ev)
	if err != nil {
		log.Printf("publish: marshal: %v", err)
		return
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		resp, err := p.client.Post(p.url+"/api/events", "application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("publish: %v (attempt %d)", err, attempt+1)
		} else {
			io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return
			}
			log.Printf("publish: dashboard returned %d (attempt %d)", resp.StatusCode, attempt+1)
		}
		if attempt == 0 {
			time.Sleep(retryDelay)
		}
	}
}

// Flush waits (bounded) for in-flight publishes. Nil-safe.
func (p *Publisher) Flush() {
	if p == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		log.Printf("publish: flush timed out")
	}
}

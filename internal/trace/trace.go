// Package trace carries a request-scoped, user-safe execution trace through
// the detector pipeline. Events deliberately exclude credentials, listing
// text, and image URLs so the same trace can be displayed in the extension and
// optionally mirrored to the daemon terminal.
package trace

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Event is one verified stage of an analysis request.
type Event struct {
	Timestamp time.Time `json:"timestamp"`
	Step      string    `json:"step"`
	Message   string    `json:"message"`
}

// Sink receives trace events. Implementations must be safe for concurrent use
// because detectors run concurrently.
type Sink interface {
	Add(Event)
}

type contextKey struct{}

// WithSink attaches a trace sink to an analysis context.
func WithSink(ctx context.Context, sink Sink) context.Context {
	return context.WithValue(ctx, contextKey{}, sink)
}

// Log records a safe, formatted event when a trace sink is present.
func Log(ctx context.Context, step, format string, args ...any) {
	sink, _ := ctx.Value(contextKey{}).(Sink)
	if sink == nil {
		return
	}
	sink.Add(Event{Timestamp: time.Now().UTC(), Step: step, Message: fmt.Sprintf(format, args...)})
}

// Recorder retains the chronological events for one request and can mirror
// each event to a caller-provided output function.
type Recorder struct {
	mu     sync.Mutex
	events []Event
	emit   func(Event)
}

// NewRecorder creates a request-local trace recorder.
func NewRecorder(emit func(Event)) *Recorder {
	return &Recorder{events: make([]Event, 0, 32), emit: emit}
}

// Add satisfies Sink.
func (r *Recorder) Add(event Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	if r.emit != nil {
		r.emit(event)
	}
}

// Events returns a snapshot suitable for JSON serialization.
func (r *Recorder) Events() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Event(nil), r.events...)
}

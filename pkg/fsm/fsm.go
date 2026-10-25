package fsm

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type State string

var ErrNoTransition = errors.New("fsm: no transition registered for state")

type Handler[T any] func(ctx context.Context, data *T) (State, error)

type Transition struct {
	From     State
	To       State
	Duration time.Duration
	Err      error
}

// Machine is a deterministic FSM. Build it once and reuse it across goroutines
// Run state lives in the data argument, not in the machine.
type Machine[T any] struct {
	initial   State
	terminals map[State]bool
	handlers  map[State]Handler[T]
	fallbacks map[State]State
	maxHops   int
}

type Option[T any] func(*Machine[T])

func WithMaxHops[T any](n int) Option[T] {
	return func(m *Machine[T]) { m.maxHops = n }
}

func New[T any](initial State, opts ...Option[T]) *Machine[T] {
	m := &Machine[T]{
		initial:   initial,
		terminals: map[State]bool{},
		handlers:  map[State]Handler[T]{},
		fallbacks: map[State]State{},
		maxHops:   32,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Handle registers a handler for a given state
func (m *Machine[T]) Handle(state State, h Handler[T]) *Machine[T] {
	m.handlers[state] = h
	return m
}

// Terminal marks one or more states as terminal
func (m *Machine[T]) Terminal(states ...State) *Machine[T] {
	for _, s := range states {
		m.terminals[s] = true
	}
	return m
}

// FallbackTo registers a deterministic escape hatch. When the handler of
// state returns an error, the machine moves to target instead of aborting.
func (m *Machine[T]) FallbackTo(state State, target State) *Machine[T] {
	m.fallbacks[target] = state
	return m
}

// Run walks the machine from m.initial until a terminal state or m.maxHops.
func (m *Machine[T]) Run(ctx context.Context, data *T) ([]Transition, error) {
	trace := make([]Transition, 0, 8)
	current := m.initial
	for i := 0; i < m.maxHops; i++ {
		if err := ctx.Err(); err != nil {
			return trace, fmt.Errorf("fsm: context cancelled at %q: %w", current, err)
		}

		handler, ok := m.handlers[current]
		if !ok {
			return trace, fmt.Errorf("%w: %q", ErrNoTransition, current)
		}

		start := time.Now()
		next, err := handler(ctx, data)
		hop := Transition{From: current, To: next, Duration: time.Since(start), Err: err}
		trace = append(trace, hop)

		// TODO: decidir entre abortar e usar fallback
		if err != nil {
			fallback, hasFallback := m.fallbacks[current]
			if !hasFallback {
				trace = append(trace, hop)
			}
			hop.To = fallback
			next = fallback
		}

		if m.terminals[next] {
			return trace, nil
		}

		current = next
	}
	return trace, fmt.Errorf("fsm: exceeded %d hops starting at %q", m.maxHops, m.initial)
}

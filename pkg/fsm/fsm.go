package fsm

import (
	"context"
	"errors"
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

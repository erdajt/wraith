package plugin

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// InvocationBinding freezes one registered callback within its creator's lifetime.
type InvocationBinding struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Method   string `json:"method"`
	Owner    string `json:"owner"`
	Lease    string `json:"lease"`
	Callback string `json:"callback"`
	Revision uint64 `json:"revision"`
}

// InvocationRequest starts a detached provider or tool operation with an optional deadline.
type InvocationRequest struct {
	Binding InvocationBinding
	Data    json.RawMessage
	Timeout time.Duration
}

// InvocationStart asks the host to dispatch a callback with a fresh workspace snapshot.
type InvocationStart struct {
	ID      string
	Binding InvocationBinding
	Data    json.RawMessage
}

// InvocationEvent contains one ordered stream delta or the final result.
type InvocationEvent struct {
	Sequence uint64
	Data     json.RawMessage
	Final    bool
	Error    string
}

// InvocationEffect publishes a reply only after the host accepts preceding operations.
type InvocationEffect struct {
	ID    string
	Kind  string
	Data  json.RawMessage
	Error string
}

// Invocation is a single-consumer handle; close releases queued and terminal payloads.
type Invocation struct {
	ID      string
	manager *invocationManager
}

type invocationRecord struct {
	context  context.Context
	binding  InvocationBinding
	data     json.RawMessage
	cancel   context.CancelFunc
	stop     func() bool
	wake     chan struct{}
	queue    []InvocationEvent
	bytes    int
	sequence uint64
	terminal *InvocationEvent
	consumed bool
	started  bool
	children map[string]bool
}

type invocationManager struct {
	mu             sync.Mutex
	prefix         string
	serial         uint64
	closed         bool
	records        map[string]*invocationRecord
	starts         chan InvocationStart
	cancelChildren func([]string)
}

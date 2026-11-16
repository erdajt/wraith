package runner

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"
)

type Job struct {
	Record Record
	Err    error
}

type jobState struct {
	Job
	cancel  context.CancelFunc
	capture capture
}

type Manager struct {
	mu      sync.Mutex
	jobs    map[string]*jobState
	history History
	maxJobs int
	closed  bool
	workers sync.WaitGroup
	Updates chan struct{}
}

func NewManager(history History, maxJobs int) *Manager {
	return &Manager{history: history, maxJobs: max(1, maxJobs), jobs: map[string]*jobState{}, Updates: make(chan struct{}, 1)}
}

func (m *Manager) Start(ctx context.Context, record Record, timeout time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("command manager is closed")
	}
	active := 0
	for _, job := range m.jobs {
		if job.cancel != nil {
			active++
		}
	}
	if active >= m.maxJobs || len(m.jobs) >= 128 {
		return errors.New("command job limit reached")
	}
	if _, exists := m.jobs[record.ID]; exists {
		return errors.New("command id already exists")
	}
	if strings.TrimSpace(record.Command) == "" || strings.ContainsRune(record.Command, 0) {
		return errors.New("command is empty or contains a null byte")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if timeout <= 0 {
		return errors.New("command timeout must be positive")
	}
	record.Environment = slices.Clone(record.Environment)
	record, live, err := m.history.prepare(record)
	if err != nil {
		return err
	}
	child, cancel := context.WithTimeout(ctx, timeout)
	job := &jobState{Job: Job{Record: record}, cancel: cancel, capture: capture{limit: record.CaptureLimit, tailLimit: record.TailLimit}}
	m.jobs[record.ID] = job
	m.workers.Add(1)
	go func() {
		defer m.workers.Done()
		defer cancel()
		result, err := m.history.execute(child, record, live, func(output string) {
			m.mu.Lock()
			_, _ = job.capture.Write([]byte(output))
			job.Record.Output = job.capture.text.String()
			job.Record.Tail = job.capture.tail
			job.Record.Observed = job.capture.observed
			job.Record.TailOffset = job.capture.observed - int64(len(job.capture.tail))
			job.Record.TailLine = job.capture.tailLine + 1
			job.Record.Truncated = job.capture.truncated
			m.mu.Unlock()
			m.notify()
		}, func() {
			m.mu.Lock()
			job.Record.Status = "running"
			m.mu.Unlock()
			m.notify()
		})
		m.mu.Lock()
		job.Record, job.Err, job.cancel = result, err, nil
		m.mu.Unlock()
		m.notify()
	}()
	return nil
}

func (m *Manager) Snapshot() []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		copy := job.Job
		copy.Record.Environment = nil
		out = append(out, copy)
	}
	return out
}

func (m *Manager) Cancel(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[id]
	if job == nil || job.cancel == nil {
		return false
	}
	job.cancel()
	return true
}

func (m *Manager) CancelTurn(session, generation string) {
	if generation == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, job := range m.jobs {
		if job.Record.Session == session && job.Record.Generation == generation && job.cancel != nil {
			job.cancel()
		}
	}
}

func (m *Manager) Forget(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if job := m.jobs[id]; job != nil && job.cancel == nil && job.Err == nil {
		delete(m.jobs, id)
	}
}

func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	for _, job := range m.jobs {
		if job.cancel != nil {
			job.cancel()
		}
	}
	m.mu.Unlock()
	m.workers.Wait()
}

func (m *Manager) notify() {
	select {
	case m.Updates <- struct{}{}:
	default:
	}
}

func (m *Manager) SetLimit(limit int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxJobs = max(1, limit)
}

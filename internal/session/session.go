package session

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"
)

type Message struct {
	Evidence    *Evidence `json:"evidence,omitempty"`
	Role        string    `json:"role"`
	Content     string    `json:"content"`
	Thinking    string    `json:"thinking,omitempty"`
	Edited      bool      `json:"edited,omitempty"`
	Interrupted bool      `json:"interrupted,omitempty"`
}

type Usage struct {
	Prompt          int     `json:"prompt"`
	Output          int     `json:"output"`
	InputEstimate   int     `json:"input_estimate"`
	TokensPerSecond float64 `json:"tokens_per_second"`
}

type Session struct {
	RunReferences []string  `json:"run_references,omitempty"`
	DeliveredRuns []string  `json:"delivered_runs,omitempty"`
	Provider      string    `json:"provider,omitempty"`
	Directory     string    `json:"directory,omitempty"`
	SharedRuns    []string  `json:"shared_runs,omitempty"`
	Version       int       `json:"version"`
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Model         string    `json:"model"`
	Created       time.Time `json:"created"`
	Updated       time.Time `json:"updated"`
	Messages      []Message `json:"messages"`
	Summary       string    `json:"summary,omitempty"`
	Facts         []string  `json:"facts,omitempty"`
	Pins          []string  `json:"pins,omitempty"`
	Archives      []string  `json:"archives,omitempty"`
	Declines      int       `json:"declines"`
	Usage         Usage     `json:"usage"`
}

// ID returns a random 24-character hexadecimal identifier.
func ID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// New creates an empty session for model with a new id and timestamps.
func New(model string) Session {
	now := time.Now().UTC()
	dir, _ := os.Getwd()
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}

	return Session{Directory: dir, Version: 1, ID: ID(), Title: "untitled", Model: model, Created: now, Updated: now}
}

// Clone copies the session and its mutable conversation state.
func (s Session) Clone() Session {
	s.RunReferences = append([]string(nil), s.RunReferences...)
	s.DeliveredRuns = append([]string(nil), s.DeliveredRuns...)
	s.SharedRuns = append([]string(nil), s.SharedRuns...)
	s.Messages = append([]Message(nil), s.Messages...)
	for i := range s.Messages {
		s.Messages[i].Evidence = s.Messages[i].Evidence.Clone()
	}

	s.Facts = append([]string(nil), s.Facts...)
	s.Pins = append([]string(nil), s.Pins...)
	s.Archives = append([]string(nil), s.Archives...)
	return s
}

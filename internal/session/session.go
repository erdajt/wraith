package session

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

type Message struct {
	Role        string `json:"role"`
	Content     string `json:"content"`
	Thinking    string `json:"thinking,omitempty"`
	Edited      bool   `json:"edited,omitempty"`
	Interrupted bool   `json:"interrupted,omitempty"`
}

type Usage struct {
	Prompt          int     `json:"prompt"`
	Output          int     `json:"output"`
	InputEstimate   int     `json:"input_estimate"`
	TokensPerSecond float64 `json:"tokens_per_second"`
}

type Session struct {
	SharedRuns []string  `json:"shared_runs,omitempty"`
	Version    int       `json:"version"`
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Model      string    `json:"model"`
	Created    time.Time `json:"created"`
	Updated    time.Time `json:"updated"`
	Messages   []Message `json:"messages"`
	Summary    string    `json:"summary,omitempty"`
	Facts      []string  `json:"facts,omitempty"`
	Pins       []string  `json:"pins,omitempty"`
	Archives   []string  `json:"archives,omitempty"`
	Declines   int       `json:"declines"`
	Usage      Usage     `json:"usage"`
}

func ID() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }

func New(model string) Session {
	now := time.Now().UTC()
	return Session{Version: 1, ID: ID(), Title: "untitled", Model: model, Created: now, Updated: now}
}

func (s Session) Clone() Session {
	s.SharedRuns = append([]string(nil), s.SharedRuns...)
	s.Messages = append([]Message(nil), s.Messages...)
	s.Facts = append([]string(nil), s.Facts...)
	s.Pins = append([]string(nil), s.Pins...)
	s.Archives = append([]string(nil), s.Archives...)
	return s
}

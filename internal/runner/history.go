package runner

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"syscall"
	"time"

	"github.com/erdajt/wraith/internal/fsutil"
)

const OutputLimit = 8 << 20
const MaxOutputLimit = 256 << 20

var validID = regexp.MustCompile(`^[a-f0-9]{24}$`)

type Record struct {
	CaptureLimit   int      `json:"capture_limit,omitempty"`
	TailLimit      int      `json:"tail_limit,omitempty"`
	AgentRequested bool     `json:"agent_requested,omitempty"`
	Agent          bool     `json:"agent,omitempty"`
	Captured       int      `json:"captured,omitempty"`
	Generation     string   `json:"generation,omitempty"`
	Background     bool     `json:"background,omitempty"`
	ShareOutput    bool     `json:"share_output,omitempty"`
	OutputHidden   bool     `json:"output_hidden,omitempty"`
	Environment    []string `json:"-"`
	Tail           string   `json:"tail,omitempty"`
	Observed       int64    `json:"observed,omitempty"`
	TailOffset     int64    `json:"tail_offset,omitempty"`
	TailLine       int64    `json:"tail_line,omitempty"`

	ID        string    `json:"id"`
	Session   string    `json:"session"`
	Command   string    `json:"command"`
	Directory string    `json:"directory"`
	Shell     string    `json:"shell"`
	Source    string    `json:"source"`
	Started   time.Time `json:"started"`
	Finished  time.Time `json:"finished"`
	Status    string    `json:"status"`
	ExitCode  int       `json:"exit_code"`
	Output    string    `json:"output"`
	Truncated bool      `json:"truncated"`
	Error     string    `json:"error,omitempty"`
}

type History struct{ Root string }

// Save atomically saves a compressed command record after checking its id.
func (h History) Save(r Record) error {
	if !validID.MatchString(r.ID) {
		return errors.New("invalid command id")
	}

	var source os.FileInfo
	err := fsutil.Atomic(filepath.Join(h.Root, r.ID+".json.gz"), func(w io.Writer) error {
		z := gzip.NewWriter(w)
		if err := json.NewEncoder(z).Encode(r); err != nil {
			z.Close()
			return err
		}
		if err := z.Close(); err != nil {
			return err
		}
		if file, ok := w.(*os.File); ok {
			source, _ = file.Stat()
		}
		return nil
	})
	if err == nil && source != nil {
		h.saveMetadata(r, sourceSignature(source))
	}
	return err
}

// Load loads a bounded command record and validates its id and capture limits.
func (h History) Load(id string) (Record, error) {
	return h.LoadBounded(context.Background(), id, MaxOutputLimit*6+8<<20)
}

// LoadBounded reads a command with a caller-selected decoded source limit.
func (h History) LoadBounded(ctx context.Context, id string, limit int64) (Record, error) {
	var r Record
	if limit < 1 || limit > MaxOutputLimit*6+8<<20 {
		return r, errors.New("invalid command history source limit")
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	if !validID.MatchString(id) {
		return r, errors.New("invalid command id")
	}

	f, err := os.OpenFile(filepath.Join(h.Root, id+".json.gz"), os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return r, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return r, err
	}
	if !info.Mode().IsRegular() {
		return r, errors.New("command history is not a regular file")
	}

	z, err := gzip.NewReader(contextReader{ctx, f})
	if err != nil {
		return r, err
	}
	defer z.Close()

	b, err := io.ReadAll(io.LimitReader(contextReader{ctx, z}, limit+1))
	if err != nil {
		return r, err
	}
	if int64(len(b)) > limit {
		return r, ErrHistoryLimit
	}

	if err = json.Unmarshal(b, &r); err != nil {
		return r, err
	}
	if r.ID != id {
		return r, errors.New("command id does not match file")
	}
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	return r, r.validateLimits()
}

// List lists command metadata newest first, omitting captured output.
func (h History) List() ([]Record, error) {
	paths, err := filepath.Glob(filepath.Join(h.Root, "*.json.gz"))
	if err != nil {
		return nil, err
	}

	var out []Record
	for _, path := range paths {
		name := filepath.Base(path)
		r, err := h.Load(name[:len(name)-len(".json.gz")])
		if err != nil {
			return nil, err
		}
		r.Output = ""
		r.Tail = ""
		out = append(out, r)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Started.After(out[j].Started) })
	return out, nil
}

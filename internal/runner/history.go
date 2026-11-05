package runner

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/erdajt/wraith/internal/fsutil"
)

const OutputLimit = 8 << 20

var validID = regexp.MustCompile(`^[a-f0-9]{24}$`)

type Record struct {
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

func (h History) Save(r Record) error {
	if !validID.MatchString(r.ID) {
		return errors.New("invalid command id")
	}
	return fsutil.Atomic(filepath.Join(h.Root, r.ID+".json.gz"), func(w io.Writer) error {
		z := gzip.NewWriter(w)
		if err := json.NewEncoder(z).Encode(r); err != nil {
			z.Close()
			return err
		}
		return z.Close()
	})
}

func (h History) Load(id string) (Record, error) {
	var r Record
	if !validID.MatchString(id) {
		return r, errors.New("invalid command id")
	}
	f, err := os.Open(filepath.Join(h.Root, id+".json.gz"))
	if err != nil {
		return r, err
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		return r, err
	}
	defer z.Close()
	b, err := io.ReadAll(io.LimitReader(z, OutputLimit*6+1<<20+1))
	if err != nil {
		return r, err
	}
	if len(b) > OutputLimit*6+1<<20 {
		return r, errors.New("command history exceeds decompression limit")
	}
	if err = json.Unmarshal(b, &r); err != nil {
		return r, err
	}
	if r.ID != id {
		return r, errors.New("command id does not match file")
	}
	return r, nil
}

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
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Started.After(out[j].Started) })
	return out, nil
}

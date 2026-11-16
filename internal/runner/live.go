package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/erdajt/wraith/internal/fsutil"
	"golang.org/x/sys/unix"
)

type liveRun struct {
	directory    string
	lock, prefix *os.File
}

func (h History) openLive(record Record) (*liveRun, error) {
	if !validID.MatchString(record.ID) {
		return nil, errors.New("invalid command id")
	}
	parent := filepath.Join(h.Root, "live")
	if err := os.MkdirAll(parent, 0700); err != nil {
		return nil, err
	}
	directory, err := os.MkdirTemp(parent, ".starting-")
	if err != nil {
		return nil, err
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(directory)
		}
	}()
	lock, err := os.OpenFile(filepath.Join(directory, "owner.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	live := &liveRun{directory: directory, lock: lock}
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		lock.Close()
		return nil, errors.New("command is owned by another process")
	}
	live.prefix, err = os.OpenFile(filepath.Join(directory, "prefix"), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		live.close()
		return nil, err
	}
	if err = live.checkpoint(record, &capture{}); err != nil {
		live.close()
		return nil, err
	}
	destination := filepath.Join(parent, record.ID)
	if err = unix.Renameat2(unix.AT_FDCWD, directory, unix.AT_FDCWD, destination, unix.RENAME_NOREPLACE); err != nil {
		live.close()
		return nil, err
	}
	published = true
	live.directory = destination
	parentFile, err := os.Open(parent)
	if err == nil {
		err = parentFile.Sync()
		parentFile.Close()
	}
	if err != nil {
		live.close()
		return nil, err
	}
	return live, nil
}

func (l *liveRun) checkpoint(record Record, output *capture) error {
	output.mu.Lock()
	defer output.mu.Unlock()
	record.Output = ""
	record.Tail, record.Observed = output.tail, output.observed
	record.Captured = output.text.Len()
	record.TailOffset, record.TailLine = output.observed-int64(len(output.tail)), output.tailLine+1
	record.Truncated = output.truncated
	if err := l.prefix.Sync(); err != nil {
		return err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return fsutil.Bytes(filepath.Join(l.directory, "record.json"), data)
}

func (l *liveRun) close() {
	if l.prefix != nil {
		l.prefix.Close()
	}
	if l.lock != nil {
		_ = unix.Flock(int(l.lock.Fd()), unix.LOCK_UN)
		l.lock.Close()
	}
}

func (h History) Recover() ([]Record, error) {
	paths, err := filepath.Glob(filepath.Join(h.Root, "live", "*"))
	if err != nil {
		return nil, err
	}
	var recovered []Record
	for _, directory := range paths {
		id := filepath.Base(directory)
		if !validID.MatchString(id) {
			continue
		}
		record, err := h.recoverRun(directory, id)
		if err != nil {
			return recovered, err
		}
		if record != nil {
			recovered = append(recovered, *record)
		}
	}
	return recovered, nil
}

func (h History) recoverRun(directory, id string) (*Record, error) {
	lock, err := os.OpenFile(filepath.Join(directory, "owner.lock"), os.O_RDWR, 0600)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, nil
		}
		return nil, err
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	if saved, err := h.Load(id); err == nil && !saved.Finished.IsZero() {
		return nil, os.RemoveAll(directory)
	}
	data, err := fsutil.ReadLimit(filepath.Join(directory, "record.json"), 8<<20)
	if errors.Is(err, os.ErrNotExist) {
		return nil, os.RemoveAll(directory)
	}
	if err != nil {
		return nil, err
	}
	var record Record
	if err = json.Unmarshal(data, &record); err != nil || record.ID != id || record.validateLimits() != nil {
		return nil, fmt.Errorf("invalid recovery record for command %s", id)
	}
	limit := record.CaptureLimit
	if limit == 0 {
		limit = OutputLimit
	}
	prefix, err := fsutil.ReadLimit(filepath.Join(directory, "prefix"), int64(limit))
	if err != nil {
		return nil, err
	}
	if len(prefix) < record.Captured {
		return nil, fmt.Errorf("incomplete recovery prefix for command %s", id)
	}
	record.Output = string(prefix[:record.Captured])
	if record.Finished.IsZero() {
		record.Status, record.ExitCode, record.Finished = "interrupted", -1, time.Now().UTC()
		record.Error = "process ended without a final result; retained output may be incomplete"
	} else if record.Status != "completed" && record.Status != "failed" && record.Status != "cancelled" && record.Status != "timed_out" {
		return nil, fmt.Errorf("invalid terminal recovery state for command %s", id)
	}
	if err = h.Save(record); err != nil {
		return nil, err
	}
	if err = os.RemoveAll(directory); err != nil {
		return &record, err
	}
	return &record, nil
}

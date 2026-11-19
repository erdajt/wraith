package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/erdajt/wraith/internal/fsutil"
)

const MetadataSourceLimit = 16 << 20

var ErrHistoryLimit = errors.New("command history exceeds query source limit")

type historySignature struct {
	Size, Modified int64
	Device, Inode  uint64
}

type historyMetadata struct {
	Version int
	Source  historySignature
	Record  Record
}

// sourceSignature binds cached metadata to one atomic payload replacement.
func sourceSignature(info os.FileInfo) historySignature {
	signature := historySignature{Size: info.Size(), Modified: info.ModTime().UnixNano()}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		signature.Device, signature.Inode = uint64(stat.Dev), stat.Ino
	}
	return signature
}

// saveMetadata caches small records without making committed payload saves depend on cache writes.
func (h History) saveMetadata(record Record, signature historySignature) {
	record.Output, record.Tail, record.Environment = "", "", nil
	cached := historyMetadata{Version: 1, Source: signature, Record: record}
	data, err := json.Marshal(cached)
	if err != nil || len(data) > 256<<10 {
		return
	}
	_ = fsutil.Atomic(filepath.Join(h.Root, "metadata", record.ID+".json"), func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

// Metadata reads validated cache entries or rebuilds them within the legacy source budget.
func (h History) Metadata(ctx context.Context, id string) (Record, error) {
	if !validID.MatchString(id) {
		return Record{}, errors.New("invalid command id")
	}
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	path := filepath.Join(h.Root, id+".json.gz")
	info, err := os.Stat(path)
	if err != nil {
		return Record{}, err
	}
	signature := sourceSignature(info)
	f, err := os.Open(filepath.Join(h.Root, "metadata", id+".json"))
	if err == nil {
		data, readErr := io.ReadAll(io.LimitReader(f, 256<<10+1))
		_ = f.Close()
		var cached historyMetadata
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if readErr == nil && len(data) <= 256<<10 && decoder.Decode(&cached) == nil && decoder.Decode(new(any)) == io.EOF && cached.Version == 1 && cached.Source == signature && cached.Record.ID == id && cached.Record.Output == "" && cached.Record.Tail == "" && cached.Record.validateLimits() == nil {
			current, err := os.Stat(path)
			if err == nil && sourceSignature(current) == signature {
				return cached.Record, ctx.Err()
			}
		}
	}

	record, err := h.LoadBounded(ctx, id, MetadataSourceLimit)
	if err != nil {
		return Record{}, err
	}
	current, err := os.Stat(path)
	if err != nil {
		return Record{}, err
	}
	if sourceSignature(current) != signature {
		return Record{}, errors.New("command history changed while reading metadata")
	}
	h.saveMetadata(record, signature)
	record.Output, record.Tail, record.Environment = "", "", nil
	return record, nil
}

type contextReader struct {
	context context.Context
	reader  io.Reader
}

// Read checks cancellation between bounded compressed input reads.
func (r contextReader) Read(p []byte) (int, error) {
	if err := r.context.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

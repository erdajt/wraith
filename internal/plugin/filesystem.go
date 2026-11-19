package plugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/sys/unix"
)

type fileOperation struct {
	kind, path, destination string
	data                    []byte
	offset                  int64
	limit, mode             int
	overwrite, recursive    bool
	parents, follow         bool
}

// run performs one copied filesystem request with cancellation checks before irreversible changes.
func (op fileOperation) run(ctx context.Context) (result map[string]any, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("fs.%s %q: %w", op.kind, op.path, err)
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch op.kind {
	case "read":
		file, err := os.OpenFile(op.path, os.O_RDONLY|unix.O_NONBLOCK, 0)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, errors.New("read requires a regular file")
		}
		if _, err := file.Seek(op.offset, io.SeekStart); err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(file, int64(op.limit)+1))
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		truncated := len(data) > op.limit
		data = data[:min(len(data), op.limit)]
		return map[string]any{"path": op.path, "data": data, "offset": op.offset, "next_offset": op.offset + int64(len(data)), "truncated": truncated}, nil
	case "write":
		return op.writeAtomic(ctx)
	case "stat":
		info, err := os.Lstat(op.path)
		if op.follow {
			info, err = os.Stat(op.path)
		}
		if err != nil {
			return nil, err
		}
		return map[string]any{"path": op.path, "size": info.Size(), "mode": int(info.Mode().Perm()), "type": fileKind(info.Mode()), "modified_ms": info.ModTime().UnixMilli()}, nil
	case "list":
		directory, err := os.OpenFile(op.path, os.O_RDONLY|unix.O_NONBLOCK, 0)
		if err != nil {
			return nil, err
		}
		defer directory.Close()
		for remaining := op.offset; remaining > 0; {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			entries, err := directory.ReadDir(int(min(remaining, 1024)))
			remaining -= int64(len(entries))
			if err != nil && !errors.Is(err, io.EOF) {
				return nil, err
			}
			if errors.Is(err, io.EOF) {
				break
			}
		}
		entries, err := directory.ReadDir(op.limit + 1)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		truncated := len(entries) > op.limit
		entries = entries[:min(len(entries), op.limit)]
		slices.SortFunc(entries, func(a, b os.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
		items := make([]any, 0, len(entries))
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			mode := entry.Type()
			if mode == 0 {
				info, err := entry.Info()
				if err != nil {
					return nil, err
				}
				mode = info.Mode()
			}
			items = append(items, map[string]any{"name": entry.Name(), "type": fileKind(mode)})
		}
		return map[string]any{"path": op.path, "entries": items, "next_offset": op.offset + int64(len(entries)), "truncated": truncated}, ctx.Err()
	case "mkdir":
		if op.parents {
			err = os.MkdirAll(op.path, os.FileMode(op.mode))
		} else {
			err = os.Mkdir(op.path, os.FileMode(op.mode))
		}
	case "remove":
		if op.recursive {
			err = os.RemoveAll(op.path)
		} else {
			err = os.Remove(op.path)
		}
	case "rename":
		if op.overwrite {
			err = os.Rename(op.path, op.destination)
		} else {
			err = unix.Renameat2(unix.AT_FDCWD, op.path, unix.AT_FDCWD, op.destination, unix.RENAME_NOREPLACE)
		}
	default:
		return nil, errors.New("unknown filesystem operation")
	}
	return map[string]any{"path": op.path}, err
}

// writeAtomic keeps the destination intact until a complete replacement is ready.
func (op fileOperation) writeAtomic(ctx context.Context) (map[string]any, error) {
	file, err := os.CreateTemp(filepath.Dir(op.path), ".wraith-write-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	for offset := 0; offset < len(op.data); {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		written, err := file.Write(op.data[offset:min(offset+64<<10, len(op.data))])
		if err != nil {
			return nil, err
		}
		if written == 0 {
			return nil, io.ErrShortWrite
		}
		offset += written
	}
	if err := file.Chmod(os.FileMode(op.mode)); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if op.overwrite {
		err = os.Rename(file.Name(), op.path)
	} else {
		err = os.Link(file.Name(), op.path)
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": op.path, "bytes": len(op.data)}, nil
}

// fileKind shares file type labels between metadata and directory listings.
func fileKind(mode os.FileMode) string {
	switch {
	case mode&os.ModeSymlink != 0:
		return "symlink"
	case mode.IsDir():
		return "directory"
	case mode.IsRegular():
		return "file"
	default:
		return "other"
	}
}

package plugin

import (
	"errors"
	"slices"
	"sync"
)

type processOutput struct {
	mu       sync.Mutex
	data     []byte
	observed int64
	limit    int
	stream   string
	protocol bool
	sink     *resourceSink
	ready    <-chan struct{}
}

// Write retains a bounded byte tail and separates lossless protocol data from capture notifications.
func (o *processOutput) Write(data []byte) (int, error) {
	if o.ready != nil {
		select {
		case <-o.ready:
		case <-o.sink.ctx.Done():
			return 0, o.sink.ctx.Err()
		}
	}
	written := 0
	for len(data) > 0 {
		chunk := data[:min(len(data), 32<<10)]
		o.mu.Lock()
		offset := o.observed
		o.observed += int64(len(chunk))
		if len(chunk) >= o.limit {
			o.data = append(o.data[:0], chunk[len(chunk)-o.limit:]...)
		} else {
			drop := max(0, len(o.data)+len(chunk)-o.limit)
			o.data = append(o.data[drop:], chunk...)
		}
		earliest := o.observed - int64(len(o.data))
		o.mu.Unlock()

		event := map[string]any{"stream": o.stream, "offset": offset, "next_offset": offset + int64(len(chunk)), "earliest_offset": earliest}
		if o.protocol {
			event["data"] = chunk
		}
		if err := o.sink.Emit(o.stream, event, o.protocol); err != nil {
			return written, err
		}
		written += len(chunk)
		data = data[len(chunk):]
	}
	return written, nil
}

// query copies a retained byte window and reports any prefix already evicted from capture.
func (o *processOutput) query(offset int64, limit int) (map[string]any, error) {
	if offset < 0 || limit < 1 || limit > 1<<20 {
		return nil, errors.New("process output requires a nonnegative offset and limit between 1 and 1048576")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	earliest := o.observed - int64(len(o.data))
	from := min(max(offset, earliest), o.observed)
	count := min(int64(limit), o.observed-from)
	data := slices.Clone(o.data[from-earliest : from-earliest+count])
	return map[string]any{
		"stream": o.stream, "data": data, "offset": from, "next_offset": from + count,
		"earliest_offset": earliest, "observed_bytes": o.observed, "truncated": offset < earliest,
		"more": from+count < o.observed,
	}, nil
}

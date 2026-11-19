package runner

import (
	"context"
	"fmt"
	"io"
	"time"
)

// executeTask journals cooperative extension output and finalizes the ordinary job record.
func (h History) executeTask(ctx context.Context, record Record, live *liveRun, emit func(string), started func(), task func(context.Context, io.Writer) error) (Record, error) {
	defer live.close()
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	output := &capture{emit: emit, sink: live.prefix, limit: record.CaptureLimit, tailLimit: record.TailLimit}
	writer := &journalWriter{output: output, live: live, record: record}
	stop, checkpointed := make(chan struct{}), make(chan error, 1)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				checkpointed <- nil
				return
			case <-ticker.C:
				if err := live.checkpoint(record, output); err != nil {
					cancel()
					checkpointed <- err
					return
				}
			}
		}
	}()
	if started != nil {
		started()
	}
	err := task(child, writer)
	close(stop)
	if checkpointErr := <-checkpointed; checkpointErr != nil {
		err = fmt.Errorf("checkpoint output: %w", checkpointErr)
	}
	exitCode := 0
	if err != nil {
		exitCode = 1
	}
	return h.finish(ctx, record, live, output, exitCode, err)
}

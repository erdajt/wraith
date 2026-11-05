package terminal

import (
	"strings"
	"testing"
	"time"
)

func TestPTYRoundTrip(t *testing.T) {
	p, err := Start([]string{"/bin/sh", "-c", "printf ready; read value; printf 'received:%s' \"$value\""}, "", 30, 10)
	if err != nil {
		t.Skipf("pty unavailable in this environment: %v", err)
	}
	defer p.Close()
	deadline := time.After(3 * time.Second)
	var output strings.Builder
	sent := false
	for {
		select {
		case e, ok := <-p.Events:
			if !ok {
				if !strings.Contains(output.String(), "received:hello") {
					t.Fatalf("missing shell response: %q", output.String())
				}
				return
			}
			output.Write(e.Data)
			if !sent && strings.Contains(output.String(), "ready") {
				sent = true
				if err = p.Write([]byte("hello\n")); err != nil {
					t.Fatal(err)
				}
			}
		case <-deadline:
			t.Fatal("pty timed out")
		}
	}
}

func TestPTYDrainsOutputAfterShellExit(t *testing.T) {
	p, err := Start([]string{"/bin/sh", "-c", "i=0; while [ $i -lt 2000 ]; do printf x; i=$((i+1)); done; printf finished"}, "", 40, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	deadline := time.After(3 * time.Second)
	var output strings.Builder
	for {
		select {
		case e, ok := <-p.Events:
			if !ok {
				if output.String() != strings.Repeat("x", 2000)+"finished" {
					t.Fatalf("lost trailing output: received %d bytes", output.Len())
				}
				return
			}
			output.Write(e.Data)
		case <-deadline:
			t.Fatal("shell output did not close")
		}
	}
}

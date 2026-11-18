package terminal

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

type PasteEvent struct{ ID, Status string }

type pasteBridge struct {
	root        string
	wake, reply *os.File
	children    []*os.File
	events      chan PasteEvent
	done        chan struct{}
	once        sync.Once
}

// preparePaste connects zsh's line editor without changing persistent shell configuration.
func preparePaste(argv, environment []string) (*pasteBridge, []string, error) {
	if filepath.Base(argv[0]) != "zsh" || len(argv) > 2 || len(argv) == 2 && argv[1] != "-i" {
		return nil, environment, nil
	}
	root, err := os.MkdirTemp("", "wraith-shell-*")
	if err != nil {
		return nil, environment, err
	}
	p := &pasteBridge{root: root, events: make(chan PasteEvent, 16), done: make(chan struct{})}
	read, write, err := os.Pipe()
	if err != nil {
		p.close()
		return nil, environment, err
	}
	p.wake, p.children = write, []*os.File{read}
	read, write, err = os.Pipe()
	if err != nil {
		p.close()
		return nil, environment, err
	}
	p.reply, p.children = read, append(p.children, write)

	home, original, configured := "", "", false
	for _, entry := range environment {
		name, value, _ := strings.Cut(entry, "=")
		if name == "HOME" {
			home = value
		}
		if name == "ZDOTDIR" {
			original, configured = value, true
		}
	}
	if !configured {
		original = home
	}
	script := "unset ZDOTDIR\n"
	if configured {
		script = "export ZDOTDIR=" + shellQuote(original) + "\n"
	}
	script += "[[ -r " + shellQuote(filepath.Join(original, ".zshenv")) + " ]] && source " + shellQuote(filepath.Join(original, ".zshenv")) + "\n"
	script += "typeset -g _wraith_paste_root=" + shellQuote(root) + "\n" + zshPaste
	if err = os.WriteFile(filepath.Join(root, ".zshenv"), []byte(script), 0600); err != nil {
		p.close()
		return nil, environment, err
	}
	return p, append(environment, "ZDOTDIR="+root), nil
}

// shellQuote quotes a generated startup path without interpreting its contents.
func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }

// read forwards insertion acknowledgements from the dedicated shell pipe.
func (p *pasteBridge) read() {
	defer close(p.events)
	scanner := bufio.NewScanner(p.reply)
	scanner.Buffer(make([]byte, 128), 256)
	for scanner.Scan() {
		id, status, ok := strings.Cut(scanner.Text(), " ")
		if !ok {
			continue
		}
		p.remove(id)
		select {
		case p.events <- PasteEvent{ID: id, Status: status}:
		case <-p.done:
			return
		}
	}
}

// close releases bridge descriptors and temporary command files once.
func (p *pasteBridge) close() {
	p.once.Do(func() {
		close(p.done)
		for _, f := range append(p.children, p.wake, p.reply) {
			if f != nil {
				_ = f.Close()
			}
		}
		_ = os.RemoveAll(p.root)
	})
}

// QueuePaste asks an integrated shell to insert text only into its empty edit buffer.
func (p *PTY) QueuePaste(command string) (string, error) {
	if p.paste == nil {
		return "", errors.New("shell has no automatic insertion integration; use manual staging")
	}
	if command == "" || len(command) > 4<<20 {
		return "", errors.New("saved command is empty or exceeds 4 mib")
	}
	for _, char := range command {
		if unicode.IsControl(char) && char != '\n' && char != '\t' {
			return "", errors.New("saved command contains terminal controls")
		}
	}
	id := fmt.Sprintf("%x", rand.Text())
	path := filepath.Join(p.paste.root, id)
	if err := os.WriteFile(path, []byte(command), 0600); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if n, err := p.paste.wake.Write([]byte(id + "\n")); err != nil || n != len(id)+1 {
		_ = os.Remove(path)
		return "", errors.New("shell insertion notification failed; command retained for manual recovery")
	}
	return id, nil
}

// CancelPaste removes a queued payload; an insertion already acknowledged remains in the shell buffer.
func (p *PTY) CancelPaste(id string) {
	if p.paste != nil {
		p.paste.remove(id)
	}
}

// remove discards a payload only when its id has the generated format.
func (p *pasteBridge) remove(id string) {
	if _, err := hex.DecodeString(id); err == nil && len(id) == 52 {
		_ = os.Remove(filepath.Join(p.root, id))
	}
}

const zshPaste = `
function _wraith_paste_widget() {
  emulate -L zsh
  local request payload
  if ! IFS= read -r -u 3 request; then
    zle -F 3
    return
  fi
  [[ -n $request && $request != *[^a-zA-Z0-9]* ]] || return
  if [[ ! -r $_wraith_paste_root/$request ]]; then
    print -r -- "$request cancelled" >&4
    return
  fi
  if [[ -n $BUFFER || -n $PREBUFFER ]]; then
    print -r -- "$request occupied" >&4
    return
  fi
  payload=$(<"$_wraith_paste_root/$request")
  BUFFER=$payload
  CURSOR=$#BUFFER
  print -r -- "$request inserted" >&4
  zle -R
}
function _wraith_paste_install() {
  [[ -o zle ]] || return
  zle -N _wraith_paste_widget
  zle -F -w 3 _wraith_paste_widget
  precmd_functions=(${precmd_functions:#_wraith_paste_install})
  print -r -- 'ready integrated' >&4
}
precmd_functions+=(_wraith_paste_install)
`

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/fsutil"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/store"
	"github.com/erdajt/wraith/internal/ui"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "wraith:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("wraith", flag.ContinueOnError)
	flags.SetOutput(out)
	showVersion := flags.Bool("version", false, "print version")
	demo := flags.Bool("demo", false, "open a demo without model requests or saved changes")
	snapshot := flags.Bool("snapshot", false, "render one demo frame to stdout")
	resume := flags.Bool("resume", false, "choose a saved session")
	id := flags.String("session", "", "resume a session id")
	model := flags.String("model", "", "model for this chat")
	theme := flags.String("theme", "", "theme override")
	configDir := flags.String("config-dir", "", "override the configuration directory")
	dataDir := flags.String("data-dir", "", "override the data directory")
	cacheDir := flags.String("cache-dir", "", "override the cache directory")
	width := flags.Int("width", 110, "snapshot width")
	height := flags.Int("height", 34, "snapshot height")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *resume && *id != "" {
		return errors.New("--resume cannot be combined with --session")
	}
	if *showVersion {
		fmt.Fprintln(out, "wraith", version)
		return nil
	}
	p, err := config.Locations()
	if err != nil {
		return err
	}
	for _, v := range []struct {
		override string
		target   *string
	}{{*configDir, &p.Config}, {*dataDir, &p.Data}, {*cacheDir, &p.Cache}} {
		if v.override != "" {
			abs, e := filepath.Abs(v.override)
			if e != nil {
				return e
			}
			*v.target = abs
		}
	}
	c := config.Default()
	if !*demo && !*snapshot {
		c, err = config.LoadDirectory(p.Config)
		if err != nil {
			return err
		}
	}
	if *model != "" {
		c.Model = *model
	}
	if *theme != "" {
		c.Theme = *theme
	}
	if err = c.Validate(); err != nil {
		return err
	}
	s := store.Store{Root: p.Data}
	if flags.NArg() > 0 {
		switch flags.Arg(0) {
		case "doctor":
			return doctor(c, p, out)
		case "config":
			fmt.Fprintln(out, config.File(p.Config))
			return nil
		case "models":
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			models, e := ollama.New(c.Host).Models(ctx)
			if e != nil {
				return e
			}
			for _, m := range models {
				fmt.Fprintf(out, "%-65s %.1f gb\n", m.Name, float64(m.Size)/1e9)
			}
			return nil
		case "sessions":
			list, e := s.List()
			if e != nil {
				return e
			}
			for _, v := range list {
				fmt.Fprintf(out, "%s  %s  %s\n", v.ID, v.Updated.Local().Format("2006-01-02 15:04"), v.Title)
			}
			return nil
		case "export":
			if flags.NArg() != 2 {
				return errors.New("usage: wraith export session-id")
			}
			v, e := s.Load(flags.Arg(1))
			if e != nil {
				return e
			}
			_, e = io.WriteString(out, ui.Export(v))
			return e
		default:
		}
	}
	chat := session.New(c.Model)
	if !*demo && !*snapshot {
		unlock, e := s.Lock()
		if e != nil {
			return e
		}
		defer unlock()
		if *id != "" {
			chat, e = s.Load(*id)
			if e != nil {
				return e
			}
		}
		if *model != "" {
			chat.Model = *model
		}
		if !*resume {
			if err = s.Save(chat); err != nil {
				return err
			}
		}
		memory := filepath.Join(p.Config, "instructions.md")
		if _, e = os.Stat(memory); errors.Is(e, os.ErrNotExist) {
			if e = fsutil.Bytes(memory, []byte("")); e != nil {
				return e
			}
		}
	}
	a, err := ui.New(c, p, chat, *demo || *snapshot)
	if err != nil {
		return err
	}
	defer a.Close()
	a.Prompt.Set(strings.Join(flags.Args(), " "))
	if *resume {
		a.OpenSessions()
	}
	if *snapshot {
		_, _ = a.Update(tea.WindowSizeMsg{Width: max(50, *width), Height: max(18, *height)})
		_, err = fmt.Fprintln(out, a.View())
		return err
	}
	options := []tea.ProgramOption{tea.WithAltScreen(), tea.WithOutput(out)}
	if c.Appearance.Mouse {
		options = append(options, tea.WithMouseCellMotion())
	}
	_, err = tea.NewProgram(a, options...).Run()
	return err
}

func doctor(c config.Config, p config.Paths, out io.Writer) error {
	fmt.Fprintf(out, "wraith %s\nconfig  %s\ndata    %s\ncache   %s\nollama  %s\nmodel   %s\n", version, p.Config, p.Data, p.Cache, c.Host, c.Model)
	for _, entry := range []struct {
		name string
		argv []string
	}{{"editor", c.Editor}, {"shell", c.Shell}} {
		path, err := exec.LookPath(entry.argv[0])
		if err != nil {
			fmt.Fprintf(out, "%s  missing: %s\n", entry.name, entry.argv[0])
		} else {
			fmt.Fprintf(out, "%s  %s\n", entry.name, path)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := ollama.New(c.Host)
	models, err := client.Models(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "installed models  %d\n", len(models))
	info, err := client.Show(ctx, c.Model)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "capabilities  %s\nthinking      %v\n", strings.Join(info.Capabilities, ", "), info.Thinking.Values)
	if _, err = info.Think(c.Thinking); err != nil {
		return err
	}
	fmt.Fprintln(out, "configuration and model connection are ready")
	fmt.Fprintln(out, "after generating, use ollama ps to check gpu placement")
	return nil
}

// Package clip copies secrets to the system clipboard using wl-copy (Wayland),
// xclip or xsel (X11).
//
// The helper runs in the foreground as a child process that owns the
// selection. The secret goes through its stdin, never through argv or the
// environment. The clipboard holds our value exactly as long as that process
// runs: it exits when another program takes the selection (or, in paste-once
// mode, after the first paste). Clearing means ending that process, so there
// is no need to read the clipboard back and no risk of wiping something the
// user copied later.
package clip

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
)

// ErrNoTool is returned when no supported clipboard helper is installed.
var ErrNoTool = errors.New("no clipboard tool found (install wl-clipboard, xclip or xsel)")

type tool struct {
	name string
	args func(pasteOnce bool) []string
	// pasteOnce reports whether the tool supports serving a single paste.
	pasteOnce bool
}

var (
	// --sensitive sets x-kde-passwordManagerHint so KDE's Klipper and other
	// clipboard managers that honour it keep it out of their history.
	wlCopy = tool{"wl-copy", func(once bool) []string {
		a := []string{"--foreground", "--sensitive", "--type", "text/plain;charset=utf-8"}
		if once {
			a = append(a, "--paste-once")
		}
		return a
	}, true}
	xclip = tool{"xclip", func(once bool) []string {
		a := []string{"-selection", "clipboard", "-in", "-quiet"}
		if once {
			a = append(a, "-loops", "1")
		}
		return a
	}, true}
	xsel = tool{"xsel", func(bool) []string {
		return []string{"--clipboard", "--input", "--nodetach"}
	}, false}
)

// Clipboard owns at most one running helper process.
type Clipboard struct {
	tool tool
	path string

	mu   sync.Mutex
	proc *os.Process
	done chan struct{}
}

// Detect picks a helper for the current session.
func Detect() (*Clipboard, error) {
	var candidates []tool
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		candidates = append(candidates, wlCopy)
	}
	if os.Getenv("DISPLAY") != "" {
		candidates = append(candidates, xclip, xsel)
	}
	for _, t := range candidates {
		if p, err := exec.LookPath(t.name); err == nil {
			return &Clipboard{tool: t, path: p}, nil
		}
	}
	return nil, ErrNoTool
}

// Tool returns the name of the helper in use.
func (c *Clipboard) Tool() string { return c.tool.name }

// SupportsPasteOnce reports whether the helper can serve exactly one paste.
func (c *Clipboard) SupportsPasteOnce() bool { return c.tool.pasteOnce }

// Copy puts secret on the clipboard, replacing anything we copied earlier.
// The caller may wipe secret as soon as Copy returns.
func (c *Clipboard) Copy(secret []byte, pasteOnce bool) error {
	c.Clear()

	cmd := exec.Command(c.path, c.tool.args(pasteOnce && c.tool.pasteOnce)...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("clipboard stdin: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting %s: %w", c.tool.name, err)
	}
	// Write synchronously so the secret slice is no longer referenced once
	// Copy returns.
	_, werr := stdin.Write(secret)
	cerr := stdin.Close()
	if werr != nil || cerr != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("writing to %s: %w", c.tool.name, errors.Join(werr, cerr))
	}

	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	c.mu.Lock()
	c.proc, c.done = cmd.Process, done
	c.mu.Unlock()
	return nil
}

// Active reports whether the clipboard still holds our secret.
func (c *Clipboard) Active() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.done == nil {
		return false
	}
	select {
	case <-c.done:
		return false
	default:
		return true
	}
}

// Clear removes our secret from the clipboard if it is still there. It does
// nothing if another program has taken over the clipboard in the meantime.
func (c *Clipboard) Clear() {
	c.mu.Lock()
	proc, done := c.proc, c.done
	c.proc, c.done = nil, nil
	c.mu.Unlock()
	if proc == nil {
		return
	}
	select {
	case <-done:
		return
	default:
	}
	_ = proc.Kill()
	<-done
}

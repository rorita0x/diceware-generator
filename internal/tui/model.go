// Package tui implements the Bubble Tea interface.
package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/awnumar/memguard"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"diceware/internal/clip"
	"diceware/internal/gen"
	"diceware/internal/harden"
	"diceware/internal/history"
	"diceware/internal/wordlist"
)

// ClipboardTimeout is how long a copied passphrase stays on the clipboard.
const ClipboardTimeout = 30 * time.Second

type screen int

const (
	screenMain screen = iota
	screenSettings
	screenHistory
	screenSave
	screenGuide
)

// separator presets cycled with ←/→; the last slot is the custom value.
var sepPresets = []string{" ", "-", "_", ".", ",", ""}

type settingRow int

const (
	rowWords settingRow = iota
	rowSep
	rowCaps
	rowDigits
	rowDigitMin
	rowDigitMax
	rowWordlist
	rowPasteOnce
	rowCount
)

type clipExpiredMsg struct{ gen int }

// Model is the Bubble Tea model.
type Model struct {
	lists   []*wordlist.List
	listIdx int
	cfg     gen.Config

	hist   history.History
	reveal bool

	clip      *clip.Clipboard
	clipErr   error
	clipGen   int
	pasteOnce bool

	hardening harden.Status

	screen    screen
	returnTo  screen
	cursor    int // settings row or history row
	customSep string
	sepInput  textinput.Model
	editSep   bool

	saveInput  textinput.Model
	saveTarget *memguard.Enclave

	guide viewport.Model
	help  help.Model

	status    string
	statusErr bool

	width, height int
}

// New builds the initial model and generates the first passphrase.
func New(lists []*wordlist.List, cb *clip.Clipboard, clipErr error, hs harden.Status) Model {
	si := textinput.New()
	si.Prompt = "separator: "
	si.CharLimit = gen.MaxSepLength

	sv := textinput.New()
	sv.Prompt = "path: "
	sv.Placeholder = "~/passphrase.txt"
	sv.CharLimit = 4096

	m := Model{
		lists:     lists,
		cfg:       gen.Default(),
		clip:      cb,
		clipErr:   clipErr,
		hardening: hs,
		sepInput:  si,
		saveInput: sv,
		guide:     viewport.New(80, 20),
		help:      help.New(),
		width:     80,
		height:    24,
	}
	m.generate()
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

func (m *Model) list() *wordlist.List { return m.lists[m.listIdx] }

func (m *Model) entropy() float64 { return gen.Entropy(m.cfg, m.list().Len()) }

func (m *Model) setStatus(s string, isErr bool) { m.status, m.statusErr = s, isErr }

// generate creates a new passphrase and adds it to the history.
func (m *Model) generate() {
	e, err := gen.Generate(m.cfg, m.list().Words)
	if err != nil {
		m.setStatus("generate: "+err.Error(), true)
		return
	}
	m.hist.Add(history.Entry{Secret: e, Bits: m.entropy(), Created: time.Now(), Wordlist: m.list().Name})
	m.setStatus("", false)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.Width = msg.Width - 4
		m.guide.Width = max(20, msg.Width-4)
		m.guide.Height = max(5, msg.Height-6)
		m.refreshGuide()
		return m, nil

	case clipExpiredMsg:
		if msg.gen == m.clipGen && m.clip != nil && m.clip.Active() {
			m.clip.Clear()
			m.setStatus("clipboard cleared", false)
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.screen {
		case screenSettings:
			return m.updateSettings(msg)
		case screenHistory:
			return m.updateHistory(msg)
		case screenSave:
			return m.updateSave(msg)
		case screenGuide:
			return m.updateGuide(msg)
		default:
			return m.updateMain(msg)
		}
	}
	return m, nil
}

func (m Model) updateMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	latest, ok := m.hist.Latest()
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, keys.Reroll):
		m.generate()
	case key.Matches(msg, keys.Reveal):
		m.reveal = !m.reveal
	case key.Matches(msg, keys.Copy):
		if ok {
			return m, m.copy(latest.Secret)
		}
	case key.Matches(msg, keys.Save):
		if ok {
			m.openSave(latest.Secret)
			return m, textinput.Blink
		}
	case key.Matches(msg, keys.History):
		m.screen, m.cursor = screenHistory, 0
		m.reveal = false
	case key.Matches(msg, keys.Settings):
		m.screen, m.cursor = screenSettings, 0
	case key.Matches(msg, keys.Guide):
		m.screen = screenGuide
		m.refreshGuide()
		m.guide.GotoTop()
	}
	return m, nil
}

func (m Model) updateHistory(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := m.hist.Len()
	switch {
	case key.Matches(msg, keys.Back):
		m.screen, m.reveal = screenMain, false
	case key.Matches(msg, keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, keys.Down):
		if m.cursor < n-1 {
			m.cursor++
		}
	case key.Matches(msg, keys.Reveal):
		m.reveal = !m.reveal
	case key.Matches(msg, keys.Copy):
		if m.cursor < n {
			return m, m.copy(m.hist.At(m.cursor).Secret)
		}
	case key.Matches(msg, keys.Save):
		if m.cursor < n {
			m.openSave(m.hist.At(m.cursor).Secret)
			return m, textinput.Blink
		}
	case key.Matches(msg, keys.Wipe):
		m.hist.Clear()
		m.cursor, m.reveal = 0, false
		if m.clip != nil {
			m.clip.Clear()
		}
		m.setStatus("history wiped", false)
	}
	return m, nil
}

func (m Model) updateGuide(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, keys.Back) || key.Matches(msg, keys.Guide) {
		m.screen = screenMain
		return m, nil
	}
	var cmd tea.Cmd
	m.guide, cmd = m.guide.Update(msg)
	return m, cmd
}

func (m Model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.editSep {
		switch msg.String() {
		case "enter":
			v := m.sepInput.Value()
			c := m.cfg
			c.Sep = v
			if err := c.Validate(); err != nil {
				m.setStatus(err.Error(), true)
				return m, nil
			}
			m.customSep, m.cfg.Sep = v, v
			m.editSep = false
			m.sepInput.Blur()
			m.setStatus("", false)
			return m, nil
		case "esc":
			m.editSep = false
			m.sepInput.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.sepInput, cmd = m.sepInput.Update(msg)
		return m, cmd
	}

	switch {
	case key.Matches(msg, keys.Back), key.Matches(msg, keys.Settings):
		m.screen = screenMain
	case key.Matches(msg, keys.Reroll) && msg.String() == "r":
		m.generate()
		m.setStatus("re-rolled with new settings", false)
	case key.Matches(msg, keys.Up):
		m.cursor = (m.cursor + int(rowCount) - 1) % int(rowCount)
	case key.Matches(msg, keys.Down):
		m.cursor = (m.cursor + 1) % int(rowCount)
	case key.Matches(msg, keys.Left):
		m.adjust(-1)
	case key.Matches(msg, keys.Right):
		m.adjust(+1)
	case key.Matches(msg, keys.Enter):
		if settingRow(m.cursor) == rowSep {
			m.editSep = true
			m.sepInput.SetValue(m.cfg.Sep)
			m.sepInput.CursorEnd()
			return m, m.sepInput.Focus()
		}
	}
	return m, nil
}

func cycle(v, delta, n int) int { return ((v+delta)%n + n) % n }

func clamp(v, lo, hi int) int { return min(max(v, lo), hi) }

// adjust changes the selected setting by delta. It never re-rolls on its own.
func (m *Model) adjust(delta int) {
	switch settingRow(m.cursor) {
	case rowWords:
		m.cfg.Words = clamp(m.cfg.Words+delta, gen.MinWords, gen.MaxWords)
	case rowSep:
		opts := sepPresets
		if m.customSep != "" {
			opts = append(append([]string(nil), sepPresets...), m.customSep)
		}
		i := 0
		for j, s := range opts {
			if s == m.cfg.Sep {
				i = j
			}
		}
		m.cfg.Sep = opts[cycle(i, delta, len(opts))]
	case rowCaps:
		m.cfg.Caps = gen.Caps(cycle(int(m.cfg.Caps), delta, 4))
	case rowDigits:
		m.cfg.DigitPos = gen.DigitPos(cycle(int(m.cfg.DigitPos), delta, 4))
	case rowDigitMin:
		m.cfg.DigitMin = clamp(m.cfg.DigitMin+delta, gen.MinDigitLen, m.cfg.DigitMax)
	case rowDigitMax:
		m.cfg.DigitMax = clamp(m.cfg.DigitMax+delta, m.cfg.DigitMin, gen.MaxDigitLen)
	case rowWordlist:
		m.listIdx = cycle(m.listIdx, delta, len(m.lists))
	case rowPasteOnce:
		m.pasteOnce = !m.pasteOnce
	}
}

// copy puts the secret on the clipboard and schedules clearing it.
func (m *Model) copy(e *memguard.Enclave) tea.Cmd {
	if m.clip == nil {
		m.setStatus("clipboard unavailable: "+m.clipErr.Error(), true)
		return nil
	}
	buf, err := e.Open()
	if err != nil {
		m.setStatus("decrypt: "+err.Error(), true)
		return nil
	}
	err = m.clip.Copy(buf.Bytes(), m.pasteOnce)
	buf.Destroy()
	if err != nil {
		m.setStatus(err.Error(), true)
		return nil
	}
	m.clipGen++
	g := m.clipGen
	what := "copied"
	if m.pasteOnce && m.clip.SupportsPasteOnce() {
		what = "copied for one paste"
	}
	m.setStatus(fmt.Sprintf("%s via %s, clears in %s", what, m.clip.Tool(), ClipboardTimeout), false)
	return tea.Tick(ClipboardTimeout, func(time.Time) tea.Msg { return clipExpiredMsg{g} })
}

func (m *Model) openSave(e *memguard.Enclave) {
	m.returnTo = m.screen
	m.screen = screenSave
	m.saveTarget = e
	m.saveInput.SetValue("")
	m.saveInput.Focus()
}

func (m Model) updateSave(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.saveInput.Blur()
		m.screen, m.saveTarget = m.returnTo, nil
		m.setStatus("save cancelled", false)
		return m, nil
	case "enter":
		path := strings.TrimSpace(m.saveInput.Value())
		if path == "" {
			path = m.saveInput.Placeholder
		}
		written, err := saveSecret(path, m.saveTarget)
		if err != nil {
			m.setStatus(err.Error(), true)
			return m, nil
		}
		m.saveInput.Blur()
		m.screen, m.saveTarget = m.returnTo, nil
		m.setStatus("saved to "+written+" (mode 0600)", false)
		return m, nil
	}
	var cmd tea.Cmd
	m.saveInput, cmd = m.saveInput.Update(msg)
	return m, cmd
}

// saveSecret writes the secret plus a newline to a new file with mode 0600.
// It refuses to overwrite existing files.
func saveSecret(path string, e *memguard.Enclave) (string, error) {
	if e == nil {
		return "", errors.New("nothing to save")
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving ~: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	buf, err := e.Open()
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	defer buf.Destroy()

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("save: %w", err)
	}
	_, werr := f.Write(buf.Bytes())
	if werr == nil {
		_, werr = f.Write([]byte{'\n'})
	}
	if werr == nil {
		werr = f.Sync()
	}
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("save: %w", werr)
	}
	return path, nil
}

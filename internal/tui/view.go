package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/awnumar/memguard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"diceware/internal/gen"
	"diceware/internal/guide"
	"diceware/internal/history"
)

// mask is shown instead of a hidden passphrase. Fixed width, so it does not
// leak the length.
var mask = strings.Repeat("•", 24)

// plaintext decrypts e for display. The returned string is a Go string the
// runtime can't wipe, so it is only built while the user asked to reveal.
func plaintext(e *memguard.Enclave) string {
	buf, err := e.Open()
	if err != nil {
		return "<decryption failed>"
	}
	s := string(buf.Bytes())
	buf.Destroy()
	return s
}

// View implements tea.Model.
func (m Model) View() string {
	var body string
	switch m.screen {
	case screenSettings:
		body = m.viewSettings()
	case screenHistory:
		body = m.viewHistory()
	case screenSave:
		body = m.viewSave()
	case screenGuide:
		body = m.viewGuide()
	default:
		body = m.viewMain()
	}
	return appStyle.Render(body)
}

func (m Model) header(sub string) string {
	t := titleStyle.Render("⚄ diceware")
	if sub != "" {
		t += mutedStyle.Render("  ›  " + sub)
	}
	return t
}

func (m Model) statusLine() string {
	switch {
	case m.status == "":
		return ""
	case m.statusErr:
		return errorStyle.Render("✗ " + m.status)
	default:
		return okStyle.Render("✓ " + m.status)
	}
}

func (m Model) entropyBar(bits float64) string {
	const width = 32
	filled := clamp(int(math.Round(bits/128*width)), 0, width)
	c := ratingColor(bits)
	bar := lipgloss.NewStyle().Foreground(c).Render(strings.Repeat("█", filled)) +
		mutedStyle.Render(strings.Repeat("░", width-filled))
	return fmt.Sprintf("%s  %s %s", bar,
		lipgloss.NewStyle().Bold(true).Foreground(c).Render(fmt.Sprintf("%.1f bits", bits)),
		mutedStyle.Render("· "+guide.Rating(bits)))
}

func (m Model) summary() string {
	sep := fmt.Sprintf("%q", m.cfg.Sep)
	digits := m.cfg.DigitPos.String()
	if m.cfg.DigitPos != gen.DigitsOff {
		digits += fmt.Sprintf(" %d–%d", m.cfg.DigitMin, m.cfg.DigitMax)
	}
	return mutedStyle.Render(fmt.Sprintf("%s · %d words · sep %s · caps %s · digits %s",
		m.list().Name, m.cfg.Words, sep, m.cfg.Caps, digits))
}

func (m Model) footer() string {
	clipInfo := "clipboard: none"
	if m.clip != nil {
		clipInfo = "clipboard: " + m.clip.Tool()
		if m.clip.Active() {
			clipInfo += " (holding passphrase)"
		}
	}
	return mutedStyle.Render("🔒 memguard enclaves · " + m.hardening.String() + " · " + clipInfo)
}

func (m Model) viewMain() string {
	var pass string
	latest, ok := m.hist.Latest()
	switch {
	case !ok:
		pass = mutedStyle.Render("no passphrase, press r")
	case m.reveal:
		pass = plaintext(latest.Secret)
	default:
		pass = mask + mutedStyle.Render("  (v to reveal)")
	}

	bits := m.entropy()
	if ok {
		bits = latest.Bits
	}
	help := m.help.ShortHelpView([]key.Binding{keys.Reroll, keys.Reveal, keys.Copy, keys.Save}) + "\n" +
		m.help.ShortHelpView([]key.Binding{keys.History, keys.Settings, keys.Guide, keys.Quit})
	return lipgloss.JoinVertical(lipgloss.Left,
		m.header(""),
		"",
		passStyle.Render(pass),
		"",
		m.entropyBar(bits),
		m.summary(),
		"",
		m.statusLine(),
		"",
		help,
		"",
		m.footer(),
	)
}

func (m Model) viewSettings() string {
	n := m.list().Len()
	sepLabel := map[string]string{" ": "space", "": "none"}[m.cfg.Sep]
	if sepLabel == "" {
		sepLabel = fmt.Sprintf("%q", m.cfg.Sep)
	}
	onOff := map[bool]string{true: "on", false: "off"}

	digitNote := ""
	if m.cfg.DigitPos != gen.DigitsOff {
		b := gen.Entropy(m.cfg, n) - gen.Entropy(gen.Config{Words: m.cfg.Words, Caps: m.cfg.Caps}, n)
		digitNote = fmt.Sprintf("+%.1f bits", b)
	}
	capsNote := "+0 bits (attacker knows the rule)"
	if m.cfg.Caps == gen.CapsRandom {
		capsNote = fmt.Sprintf("+%d bits (1 per word)", m.cfg.Words)
	}
	sepNote := "enter: custom"
	if m.cfg.Sep == "" {
		sepNote = "harder to type and read; enter: custom"
	}
	pasteNote := "clipboard managers may use up the single paste"
	if m.clip != nil && !m.clip.SupportsPasteOnce() {
		pasteNote = m.clip.Tool() + " doesn't support this"
	}

	rows := []struct{ name, value, note string }{
		{"Words", fmt.Sprint(m.cfg.Words), fmt.Sprintf("%.2f bits each", m.list().BitsPerWord())},
		{"Separator", sepLabel, sepNote},
		{"Capitalization", m.cfg.Caps.String(), capsNote},
		{"Digits", m.cfg.DigitPos.String(), digitNote},
		{"Digit length min", fmt.Sprint(m.cfg.DigitMin), "length is random in [min, max]"},
		{"Digit length max", fmt.Sprint(m.cfg.DigitMax), ""},
		{"Wordlist", m.list().Name, fmt.Sprintf("%d words", n)},
		{"Copy: paste once", onOff[m.pasteOnce], pasteNote},
	}

	var b strings.Builder
	for i, r := range rows {
		cursor, name := "  ", r.name
		value := "‹ " + r.value + " ›"
		if i == m.cursor {
			cursor = selStyle.Render("▸ ")
			name, value = selStyle.Render(name), selStyle.Render(value)
		}
		dim := (settingRow(i) == rowDigitMin || settingRow(i) == rowDigitMax) && m.cfg.DigitPos == gen.DigitsOff
		line := fmt.Sprintf("%s%-*s %-28s %s", cursor, 18, name, value, mutedStyle.Render(r.note))
		if dim {
			line = mutedStyle.Render(fmt.Sprintf("%s%-*s %-28s", "  ", 18, r.name, "‹ "+r.value+" ›"))
			if i == m.cursor {
				line = selStyle.Render("▸ ") + line[len("  "):]
			}
		}
		b.WriteString(line + "\n")
	}

	sep := ""
	if m.editSep {
		sep = "\n" + m.sepInput.View() + mutedStyle.Render("  (enter to apply, esc to cancel)") + "\n"
	}

	help := m.help.ShortHelpView([]key.Binding{keys.Up, keys.Down, keys.Left, keys.Right,
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "re-roll")), keys.Back})
	return lipgloss.JoinVertical(lipgloss.Left,
		m.header("options"),
		"",
		b.String()+sep,
		m.entropyBar(m.entropy()),
		mutedStyle.Render("Settings apply to the next roll."),
		"",
		m.statusLine(),
		"",
		help,
	)
}

func (m Model) viewHistory() string {
	var b strings.Builder
	if m.hist.Len() == 0 {
		b.WriteString(mutedStyle.Render("empty. Nothing is kept beyond this session.") + "\n")
	}
	// Keep the cursor visible on small terminals.
	visible := max(3, m.height-12)
	start := clamp(m.cursor-visible/2, 0, max(0, m.hist.Len()-visible))
	end := min(m.hist.Len(), start+visible)
	for i := start; i < end; i++ {
		e := m.hist.At(i)
		b.WriteString(m.historyLine(i, e) + "\n")
	}
	help := m.help.ShortHelpView([]key.Binding{keys.Up, keys.Down, keys.Reveal, keys.Copy, keys.Save, keys.Wipe, keys.Back})
	return lipgloss.JoinVertical(lipgloss.Left,
		m.header(fmt.Sprintf("history (%d, memory only)", m.hist.Len())),
		"",
		b.String(),
		m.statusLine(),
		"",
		help,
	)
}

func (m Model) historyLine(i int, e history.Entry) string {
	pass := mask
	if i == m.cursor && m.reveal {
		pass = plaintext(e.Secret)
	}
	meta := mutedStyle.Render(fmt.Sprintf("%s  %5.1f bits  %-14s", e.Created.Format("15:04:05"), e.Bits, e.Wordlist))
	if i == m.cursor {
		return selStyle.Render("▸ ") + meta + "  " + selStyle.Render(pass)
	}
	return "  " + meta + "  " + pass
}

func (m Model) viewSave() string {
	warn := lipgloss.NewStyle().Foreground(colWeak).Width(max(40, m.width-6)).Render(
		"⚠ Writing the passphrase to disk defeats the memory-only design: it can survive in backups, " +
			"filesystem journals and SSD remapped blocks even after you delete it. Prefer copying it " +
			"directly into your password manager.")
	return lipgloss.JoinVertical(lipgloss.Left,
		m.header("save to file"),
		"",
		warn,
		"",
		m.saveInput.View(),
		mutedStyle.Render("New file only (never overwrites), mode 0600. Enter to save, esc to cancel."),
		"",
		m.statusLine(),
	)
}

func (m Model) viewGuide() string {
	pct := fmt.Sprintf("%3.0f%%", m.guide.ScrollPercent()*100)
	help := m.help.ShortHelpView([]key.Binding{
		key.NewBinding(key.WithKeys("up"), key.WithHelp("↑↓/pgup/pgdn", "scroll")), keys.Back})
	return lipgloss.JoinVertical(lipgloss.Left,
		m.header("threat model & entropy guide")+mutedStyle.Render("  "+pct),
		m.guide.View(),
		help,
	)
}

// refreshGuide re-renders the guide for the current width and entropy.
func (m *Model) refreshGuide() {
	bits := m.entropy()
	if e, ok := m.hist.Latest(); ok {
		bits = e.Bits
	}
	m.guide.SetContent(guideContent(m.guide.Width, bits))
}

func guideContent(width int, current float64) string {
	para := lipgloss.NewStyle().Width(max(30, width-2))
	h2 := headerStyle.Foreground(colAccent).MarginTop(1)
	var b strings.Builder
	w := func(s string) { b.WriteString(s + "\n") }

	w(para.Render(fmt.Sprintf(
		"Your current setting yields %.1f bits. Every bit doubles the attacker's work. "+
			"All numbers assume the attacker knows your word list and settings: diceware's "+
			"strength comes only from the random choice, never from secrecy of the method.", current)))

	w(h2.Render("Which threat model are you defending against?"))
	threats := table.New().Border(lipgloss.RoundedBorder()).BorderStyle(mutedStyle).
		Headers("Threat model", "Example", "Target", "EFF/DE", "You").
		StyleFunc(func(row, col int) lipgloss.Style {
			s := lipgloss.NewStyle().Padding(0, 1)
			if row == table.HeaderRow {
				return s.Bold(true)
			}
			return s
		}).Wrap(true)
	if width < 100 {
		threats.Width(width - 2)
	}
	for _, t := range guide.Threats {
		ok := errorStyle.Render("✗")
		if current >= t.MinBits {
			ok = okStyle.Render("✓")
		}
		threats.Row(t.Name, t.Example, fmt.Sprintf("≥ %.0f bits", t.MinBits), t.Words, ok)
	}
	w(threats.Render())

	w(h2.Render("SHA-256 vs. Argon2id"))
	w(para.Render(
		"SHA-256 is built to be fast: a single gaming GPU tries tens of billions of guesses per second. " +
			"Argon2id is a password hashing function that is slow on purpose and memory-hard: every guess " +
			"has to fill and re-read 64 MiB of RAM, so GPUs and ASICs are limited by memory bandwidth instead " +
			"of compute. That makes each guess roughly 25 million times more expensive, worth about 24 bits " +
			"(≈ 2 extra words) for free. You usually don't get to choose the hash a service uses, so pick " +
			"entropy for the worst case you expect."))

	for _, h := range []guide.Hash{guide.SHA256, guide.Argon2id} {
		w(h2.Render("Average time to crack: " + h.String()))
		w(crackTable(h, current, width))
	}

	w(h2.Render("Attacker tiers (order-of-magnitude estimates)"))
	tiers := table.New().Border(lipgloss.RoundedBorder()).BorderStyle(mutedStyle).
		Headers("Tier", "Hardware", "SHA-256 /s", "Argon2id /s").
		StyleFunc(func(row, col int) lipgloss.Style { return lipgloss.NewStyle().Padding(0, 1) })
	for _, t := range guide.Tiers {
		tiers.Row(t.Name, t.Hardware, guide.FormatRate(t.SHA256), guide.FormatRate(t.Argon2id))
	}
	w(tiers.Render())

	w(h2.Render("Notes"))
	w(para.Render(
		"• Times are averages: on average half the key space is searched (2^(bits−1) guesses). " +
			"An attacker can get lucky early.\n" +
			"• Hardware gets faster. Add ~1 bit for every two years the secret must stay safe.\n" +
			"• Online attacks (login forms with rate limits) are millions of times slower; 4 words are plenty there.\n" +
			"• One more word (+12.9 bits) beats any amount of capitalization or digit tricks.\n" +
			"• Never reuse a passphrase. The best entropy doesn't help once it leaks somewhere else."))
	return b.String()
}

func crackTable(h guide.Hash, current float64, width int) string {
	headers := []string{"Entropy"}
	for _, t := range guide.Tiers {
		headers = append(headers, t.Name)
	}
	rows := guide.Rows(current)
	hl := -1
	t := table.New().Border(lipgloss.RoundedBorder()).BorderStyle(mutedStyle).Headers(headers...)
	for i, bits := range rows {
		label := fmt.Sprintf("%.0f bits", bits)
		if math.Abs(bits-current) < 0.05 {
			label = fmt.Sprintf("▸ %.1f you", bits)
			hl = i
		}
		r := []string{label}
		for _, tier := range guide.Tiers {
			r = append(r, guide.FormatDuration(guide.CrackSeconds(bits, tier.Rate(h))))
		}
		t.Row(r...)
	}
	t.StyleFunc(func(row, col int) lipgloss.Style {
		s := lipgloss.NewStyle().Padding(0, 1)
		switch {
		case row == table.HeaderRow:
			return s.Bold(true)
		case row == hl:
			return s.Bold(true).Foreground(colAccent)
		case col == 0:
			return s.Foreground(colMuted)
		}
		return s
	})
	if width < 90 {
		t.Width(width - 2)
	}
	return t.Render()
}

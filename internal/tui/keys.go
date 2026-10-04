package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Reroll, Reveal, Copy, Save, History, Settings, Guide, Quit key.Binding
	Up, Down, Left, Right, Enter, Back, Wipe                   key.Binding
}

var keys = keyMap{
	Reroll:   key.NewBinding(key.WithKeys("r", " "), key.WithHelp("r/space", "re-roll")),
	Reveal:   key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "reveal/hide")),
	Copy:     key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy")),
	Save:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "save to file")),
	History:  key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "history")),
	Settings: key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "options")),
	Guide:    key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "threat model")),
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),

	Up:    key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:  key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Left:  key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "less")),
	Right: key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "more")),
	Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
	Back:  key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc", "back")),
	Wipe:  key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "wipe history")),
}

// Command diceware is a terminal diceware passphrase generator that keeps
// its secrets in encrypted, locked memory and never writes them to disk
// unless asked to.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/awnumar/memguard"
	tea "github.com/charmbracelet/bubbletea"

	"diceware/internal/clip"
	"diceware/internal/harden"
	"diceware/internal/tui"
	"diceware/internal/wordlist"
)

func main() {
	// Purge wipes all memguard buffers and destroys the enclave key. It is
	// skipped by os.Exit, so run() returns instead of exiting directly.
	err := run()
	memguard.Purge()
	if err != nil {
		fmt.Fprintln(os.Stderr, "diceware:", err)
		os.Exit(1)
	}
}

func run() error {
	custom := flag.String("wordlist", "", "path to an additional word list (one word per line, or \"index word\")")
	flag.Parse()

	hs := harden.Apply()

	lists, err := wordlist.Embedded()
	if err != nil {
		return err
	}
	if *custom != "" {
		l, err := wordlist.Load(*custom)
		if err != nil {
			return err
		}
		lists = append(lists, l)
	}

	cb, clipErr := clip.Detect()

	p := tea.NewProgram(tui.New(lists, cb, clipErr, hs), tea.WithAltScreen())

	// Bubble Tea handles SIGINT and SIGTERM itself. SIGHUP (terminal closed)
	// ends the program the same way, so the cleanup below still runs.
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	go func() {
		if _, ok := <-hup; ok {
			p.Kill()
		}
	}()

	_, err = p.Run()
	signal.Stop(hup)
	close(hup)

	if cb != nil {
		cb.Clear()
	}
	if err != nil && !errors.Is(err, tea.ErrProgramKilled) && !errors.Is(err, tea.ErrInterrupted) {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

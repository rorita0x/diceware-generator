// Package history keeps generated passphrases for the current session only.
// Entries live in encrypted memguard Enclaves and are never written to disk.
package history

import (
	"time"

	"github.com/awnumar/memguard"
)

// MaxEntries limits how many passphrases are kept. Older ones are dropped.
const MaxEntries = 50

// Entry is one generated passphrase plus non-secret metadata.
type Entry struct {
	Secret   *memguard.Enclave
	Bits     float64
	Created  time.Time
	Wordlist string
}

// History is a newest-first list of entries.
type History struct {
	entries []Entry
}

// Add puts e at the front and drops the oldest entry beyond MaxEntries.
func (h *History) Add(e Entry) {
	h.entries = append([]Entry{e}, h.entries...)
	if len(h.entries) > MaxEntries {
		h.entries[MaxEntries] = Entry{}
		h.entries = h.entries[:MaxEntries]
	}
}

// Len returns the number of entries.
func (h *History) Len() int { return len(h.entries) }

// At returns the i-th newest entry.
func (h *History) At(i int) Entry { return h.entries[i] }

// Latest returns the newest entry, if any.
func (h *History) Latest() (Entry, bool) {
	if len(h.entries) == 0 {
		return Entry{}, false
	}
	return h.entries[0], true
}

// Clear drops all entries. The Enclaves' ciphertext becomes garbage; the
// key that could decrypt it is destroyed by memguard.Purge on exit.
func (h *History) Clear() {
	clear(h.entries)
	h.entries = nil
}

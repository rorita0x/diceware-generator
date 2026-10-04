// Package wordlist parses and validates diceware word lists.
package wordlist

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"diceware/wordlists"
)

// MinWords is the smallest list accepted. Below this, each word adds
// too little entropy for diceware to make sense.
const MinWords = 1024

// List is an immutable, validated word list.
type List struct {
	Name  string
	Words []string
}

// Len returns the number of words in the list.
func (l *List) Len() int { return len(l.Words) }

// BitsPerWord returns the entropy contributed by one uniformly chosen word.
func (l *List) BitsPerWord() float64 { return math.Log2(float64(len(l.Words))) }

// Parse reads a word list. Each non-empty line holds either a single word or
// a dice index followed by a word ("11111 abacus"). Lines starting with '#'
// are ignored. Words must be unique, valid UTF-8 and contain no whitespace or
// control characters.
func Parse(name string, r io.Reader) (*List, error) {
	sc := bufio.NewScanner(r)
	seen := make(map[string]struct{}, 7776)
	var words []string
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Fields(text)
		var w string
		switch len(fields) {
		case 1:
			w = fields[0]
		case 2:
			w = fields[1]
		default:
			return nil, fmt.Errorf("%s:%d: expected \"word\" or \"index word\", got %d fields", name, line, len(fields))
		}
		if err := validWord(w); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", name, line, err)
		}
		if _, dup := seen[w]; dup {
			return nil, fmt.Errorf("%s:%d: duplicate word %q", name, line, w)
		}
		seen[w] = struct{}{}
		words = append(words, w)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}
	if len(words) < MinWords {
		return nil, fmt.Errorf("%s: %d words, need at least %d", name, len(words), MinWords)
	}
	return &List{Name: name, Words: words}, nil
}

func validWord(w string) error {
	if !utf8.ValidString(w) {
		return fmt.Errorf("word is not valid UTF-8")
	}
	for _, r := range w {
		if unicode.IsSpace(r) || unicode.IsControl(r) || !unicode.IsPrint(r) {
			return fmt.Errorf("word %q contains a non-printable or whitespace character", w)
		}
	}
	return nil
}

// Embedded returns the bundled lists: EFF (English) first, then German.
func Embedded() ([]*List, error) {
	eff, err := Parse("EFF (en)", bytes.NewReader(wordlists.EFF))
	if err != nil {
		return nil, fmt.Errorf("embedded EFF list: %w", err)
	}
	de, err := Parse("Diceware (de)", bytes.NewReader(wordlists.German))
	if err != nil {
		return nil, fmt.Errorf("embedded German list: %w", err)
	}
	return []*List{eff, de}, nil
}

// Load reads and validates a word list from a file.
func Load(path string) (*List, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open wordlist: %w", err)
	}
	defer f.Close()
	return Parse("custom ("+filepath.Base(path)+")", f)
}

// Package gen generates diceware passphrases from a word list.
//
// The passphrase is assembled directly inside a memguard LockedBuffer and
// returned sealed in an encrypted Enclave. The chosen word indices and
// random draws are wiped right after use.
package gen

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"unicode"
	"unicode/utf8"

	"github.com/awnumar/memguard"
)

// Caps selects how words are capitalized.
type Caps int

const (
	CapsNone   Caps = iota // abacus
	CapsFirst              // Abacus
	CapsAll                // ABACUS
	CapsRandom             // each word: abacus or Abacus, chosen at random (+1 bit/word)
)

func (c Caps) String() string {
	switch c {
	case CapsFirst:
		return "First letter"
	case CapsAll:
		return "ALL CAPS"
	case CapsRandom:
		return "Random per word"
	default:
		return "none"
	}
}

// DigitPos selects where the random digit block goes.
type DigitPos int

const (
	DigitsOff     DigitPos = iota
	DigitsBefore           // 4711-word-word
	DigitsAfter            // word-word-4711
	DigitsBetween          // word-4711-word, in a random gap
)

func (d DigitPos) String() string {
	switch d {
	case DigitsBefore:
		return "before"
	case DigitsAfter:
		return "after"
	case DigitsBetween:
		return "between (random gap)"
	default:
		return "off"
	}
}

// Limits for Config values.
const (
	MinWords     = 3
	MaxWords     = 20
	MinDigitLen  = 1
	MaxDigitLen  = 8
	MaxSepLength = 8
)

// Config describes how a passphrase is built. It is not secret.
type Config struct {
	Words    int
	Sep      string
	Caps     Caps
	DigitPos DigitPos
	// The digit block length is drawn uniformly from [DigitMin, DigitMax].
	DigitMin int
	DigitMax int
}

// Default returns the default config: six words, joined by '-'.
func Default() Config {
	return Config{Words: 6, Sep: "-", DigitMin: 1, DigitMax: 3}
}

// Validate reports whether c can be used to generate a passphrase.
func (c Config) Validate() error {
	if c.Words < MinWords || c.Words > MaxWords {
		return fmt.Errorf("word count %d out of range [%d, %d]", c.Words, MinWords, MaxWords)
	}
	if len(c.Sep) > MaxSepLength || !utf8.ValidString(c.Sep) {
		return fmt.Errorf("separator must be valid UTF-8 and at most %d bytes", MaxSepLength)
	}
	for _, r := range c.Sep {
		if unicode.IsControl(r) {
			return errors.New("separator must not contain control characters")
		}
	}
	if c.DigitPos != DigitsOff {
		if c.DigitMin < MinDigitLen || c.DigitMax > MaxDigitLen || c.DigitMin > c.DigitMax {
			return fmt.Errorf("digit range [%d, %d] invalid, must lie within [%d, %d]", c.DigitMin, c.DigitMax, MinDigitLen, MaxDigitLen)
		}
	}
	return nil
}

// Entropy returns a conservative estimate of the passphrase entropy in bits
// for a word list of n words. It assumes the attacker knows the word list
// and the full Config, and counts the digit block by its min-entropy: the
// shortest length is the most likely guess, so variable length only adds
// log2(number of possible lengths).
func Entropy(c Config, n int) float64 {
	if n < 2 || c.Words < 1 {
		return 0
	}
	bits := float64(c.Words) * math.Log2(float64(n))
	if c.Caps == CapsRandom {
		bits += float64(c.Words)
	}
	if c.DigitPos != DigitsOff {
		bits += math.Log2(float64(c.DigitMax-c.DigitMin+1)) + float64(c.DigitMin)*math.Log2(10)
		if c.DigitPos == DigitsBetween && c.Words > 1 {
			bits += math.Log2(float64(c.Words - 1))
		}
	}
	return bits
}

// Generate builds a new passphrase from words according to c and returns it
// sealed in an Enclave.
func Generate(c Config, words []string) (*memguard.Enclave, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if len(words) < 2 {
		return nil, errors.New("word list too short")
	}

	// All random draws that make up the secret. Wiped before returning.
	idx := make([]int, c.Words)
	upper := make([]bool, c.Words)
	var digits [MaxDigitLen]byte
	digitLen, gap := 0, 0
	defer func() {
		clear(idx)
		clear(upper)
		clear(digits[:])
		digitLen, gap = 0, 0
	}()

	for i := range idx {
		v, err := Int(len(words))
		if err != nil {
			return nil, err
		}
		idx[i] = v
		switch c.Caps {
		case CapsFirst:
			upper[i] = true
		case CapsRandom:
			b, err := Int(2)
			if err != nil {
				return nil, err
			}
			upper[i] = b == 1
		}
	}

	if c.DigitPos != DigitsOff {
		l, err := Int(c.DigitMax - c.DigitMin + 1)
		if err != nil {
			return nil, err
		}
		digitLen = c.DigitMin + l
		for i := range digitLen {
			d, err := Int(10)
			if err != nil {
				return nil, err
			}
			digits[i] = byte('0' + d)
		}
		switch c.DigitPos {
		case DigitsBefore:
			gap = 0
		case DigitsAfter:
			gap = c.Words
		case DigitsBetween:
			g, err := Int(c.Words - 1)
			if err != nil {
				return nil, err
			}
			gap = 1 + g // 1..Words-1: never at either end
		}
	}

	// Pass 1 counts bytes, pass 2 writes into the locked buffer.
	size := assemble(nil, c, words, idx, upper, digits[:digitLen], gap)
	buf := memguard.NewBuffer(size)
	assemble(buf.Bytes(), c, words, idx, upper, digits[:digitLen], gap)
	return buf.Seal(), nil
}

// assemble writes the passphrase into dst and returns its length. With a nil
// dst it only counts. The digit block, if any, is inserted as its own token
// before word number gap.
func assemble(dst []byte, c Config, words []string, idx []int, upper []bool, digits []byte, gap int) int {
	n := 0
	tokens := 0
	put := func(b []byte) {
		if dst != nil {
			copy(dst[n:], b)
		}
		n += len(b)
	}
	sep := func() {
		if tokens > 0 {
			put([]byte(c.Sep))
		}
		tokens++
	}
	for i := 0; i <= len(idx); i++ {
		if len(digits) > 0 && i == gap {
			sep()
			put(digits)
		}
		if i == len(idx) {
			break
		}
		sep()
		n += writeWord(dst, n, words[idx[i]], upper[i], c.Caps == CapsAll)
	}
	return n
}

// writeWord writes w into dst at off (or only counts if dst is nil),
// upper-casing the first rune or all runes as requested.
func writeWord(dst []byte, off int, w string, first, all bool) int {
	n := 0
	for i, r := range w {
		if all || (first && i == 0) {
			r = unicode.ToUpper(r)
		}
		if dst != nil {
			n += utf8.EncodeRune(dst[off+n:], r)
		} else {
			n += utf8.RuneLen(r)
		}
	}
	return n
}

// Int returns a uniformly distributed integer in [0, n) read from
// crypto/rand. It uses rejection sampling, so there is no modulo bias, and
// keeps the raw random bytes on the stack, where they are cleared afterwards.
func Int(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("gen.Int: invalid bound %d", n)
	}
	if n == 1 {
		return 0, nil
	}
	bound := uint64(n)
	// Largest multiple of bound that fits in a uint64; values at or above it
	// would bias the result and are rejected.
	limit := math.MaxUint64 - (math.MaxUint64 % bound)
	var b [8]byte
	defer clear(b[:])
	for {
		if _, err := rand.Read(b[:]); err != nil {
			return 0, fmt.Errorf("reading crypto/rand: %w", err)
		}
		v := binary.LittleEndian.Uint64(b[:])
		if v < limit {
			return int(v % bound), nil
		}
	}
}

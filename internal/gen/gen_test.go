package gen

import (
	"math"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

var testWords = []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel", "über"}

func generate(t *testing.T, c Config) string {
	t.Helper()
	e, err := Generate(c, testWords)
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer b.Destroy()
	return string(b.Bytes())
}

func inList(w string) bool {
	for _, x := range testWords {
		if strings.EqualFold(x, w) {
			return true
		}
	}
	return false
}

var digitRun = regexp.MustCompile(`[0-9]+`)

// countMatches returns how many tokens match the regular expression.
func countMatches(tok []string, expr string) int {
	re := regexp.MustCompile(expr)
	n := 0
	for _, w := range tok {
		if re.MatchString(w) {
			n++
		}
	}
	return n
}

func TestGenerate(t *testing.T) {
	tests := []struct {
		name  string
		cfg   Config
		check func(t *testing.T, tokens []string)
	}{
		{"plain", Config{Words: 5, Sep: "-"}, func(t *testing.T, tok []string) {
			if len(tok) != 5 {
				t.Fatalf("got %d tokens", len(tok))
			}
		}},
		{"multi-byte separator", Config{Words: 4, Sep: "·"}, func(t *testing.T, tok []string) {
			if len(tok) != 4 {
				t.Fatalf("got %d tokens", len(tok))
			}
		}},
		{"first caps", Config{Words: 6, Sep: " ", Caps: CapsFirst}, func(t *testing.T, tok []string) {
			for _, w := range tok {
				if r := []rune(w)[0]; !unicode.IsUpper(r) {
					t.Errorf("%q not capitalized", w)
				}
			}
		}},
		{"all caps", Config{Words: 6, Sep: " ", Caps: CapsAll}, func(t *testing.T, tok []string) {
			for _, w := range tok {
				if w != strings.ToUpper(w) {
					t.Errorf("%q not upper case", w)
				}
			}
		}},
		{"digits before a word", Config{Words: 4, Sep: "_", DigitPos: DigitsBefore, DigitMin: 2, DigitMax: 4}, func(t *testing.T, tok []string) {
			if len(tok) != 4 || countMatches(tok, `^[0-9]{2,4}\pL+$`) != 1 {
				t.Fatalf("bad tokens %q", tok)
			}
		}},
		{"digits after a word", Config{Words: 4, Sep: "_", DigitPos: DigitsAfter, DigitMin: 3, DigitMax: 3}, func(t *testing.T, tok []string) {
			if len(tok) != 4 || countMatches(tok, `^\pL+[0-9]{3}$`) != 1 {
				t.Fatalf("bad tokens %q", tok)
			}
		}},
		{"digits between words", Config{Words: 3, Sep: "_", DigitPos: DigitsBetween, DigitMin: 1, DigitMax: 8}, func(t *testing.T, tok []string) {
			if len(tok) != 2 || countMatches(tok, `^\pL+[0-9]{1,8}\pL+$`) != 1 {
				t.Fatalf("bad tokens %q", tok)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for range 200 {
				tok := strings.Split(generate(t, tt.cfg), tt.cfg.Sep)
				for _, w := range tok {
					for _, part := range digitRun.Split(w, -1) {
						if part != "" && !inList(part) {
							t.Fatalf("unknown word %q in %q", part, tok)
						}
					}
				}
				tt.check(t, tok)
			}
		})
	}
}

func TestGenerateNoSeparator(t *testing.T) {
	for range 50 {
		got := generate(t, Config{Words: 3, Sep: "", DigitPos: DigitsAfter, DigitMin: 2, DigitMax: 2})
		if len(digitRun.FindAllString(got, -1)) != 1 || strings.ContainsAny(got, " -") {
			t.Fatalf("unexpected %q", got)
		}
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		ok   bool
	}{
		{"default", Default(), true},
		{"too few words", Config{Words: 2}, false},
		{"too many words", Config{Words: 21}, false},
		{"control sep", Config{Words: 6, Sep: "\n"}, false},
		{"long sep", Config{Words: 6, Sep: "123456789"}, false},
		{"digit range inverted", Config{Words: 6, DigitPos: DigitsAfter, DigitMin: 4, DigitMax: 2}, false},
		{"digit range too wide", Config{Words: 6, DigitPos: DigitsAfter, DigitMin: 1, DigitMax: 9}, false},
		{"digit range ignored when off", Config{Words: 6, DigitMin: 0, DigitMax: 0}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); (err == nil) != tt.ok {
				t.Errorf("Validate() = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}

func TestEntropy(t *testing.T) {
	eff := math.Log2(7776)
	tests := []struct {
		name string
		cfg  Config
		want float64
	}{
		{"6 words", Config{Words: 6}, 6 * eff},
		{"first caps adds nothing", Config{Words: 6, Caps: CapsFirst}, 6 * eff},
		{"random caps", Config{Words: 6, Caps: CapsRandom}, 6*eff + 6},
		{"fixed 3 digits after a word", Config{Words: 6, DigitPos: DigitsAfter, DigitMin: 3, DigitMax: 3}, 6*eff + 3*math.Log2(10) + math.Log2(6)},
		{"fixed 2 digits before a word", Config{Words: 4, DigitPos: DigitsBefore, DigitMin: 2, DigitMax: 2}, 4*eff + 2*math.Log2(10) + 2},
		{"1-4 digits between", Config{Words: 5, DigitPos: DigitsBetween, DigitMin: 1, DigitMax: 4}, 5*eff + 2 + math.Log2(10) + 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Entropy(tt.cfg, 7776); math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("got %.4f, want %.4f", got, tt.want)
			}
		})
	}
	if got := Entropy(Config{Words: 6}, 7776); math.Abs(got-77.55) > 0.01 {
		t.Errorf("6 EFF words = %.2f bits, want ~77.55", got)
	}
}

// TestIntUniform is a sanity check, not a proof: a chi-square test over a
// small range should not be wildly off.
func TestIntUniform(t *testing.T) {
	const n, draws = 7, 70000
	var counts [n]int
	for range draws {
		v, err := Int(n)
		if err != nil {
			t.Fatal(err)
		}
		counts[v]++
	}
	exp := float64(draws) / n
	chi := 0.0
	for _, c := range counts {
		d := float64(c) - exp
		chi += d * d / exp
	}
	// 6 degrees of freedom; p=0.0001 critical value is ~27.9.
	if chi > 27.9 {
		t.Errorf("chi-square %.2f too high, counts %v", chi, counts)
	}
}

func TestIntBounds(t *testing.T) {
	if _, err := Int(0); err == nil {
		t.Error("Int(0) should fail")
	}
	if v, err := Int(1); err != nil || v != 0 {
		t.Errorf("Int(1) = %d, %v", v, err)
	}
}

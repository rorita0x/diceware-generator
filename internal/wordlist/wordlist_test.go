package wordlist

import (
	"fmt"
	"strings"
	"testing"
)

func TestEmbedded(t *testing.T) {
	lists, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range lists {
		if l.Len() != 7776 {
			t.Errorf("%s: got %d words, want 7776", l.Name, l.Len())
		}
	}
}

// numbered returns n distinct words in "index word" format.
func numbered(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "%05d w%d\n", i, i)
	}
	return b.String()
}

// plain returns n distinct words, one per line, without index.
func plain(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "w%d\n", i)
	}
	return b.String()
}

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr string
	}{
		{"indexed", numbered(MinWords), MinWords, ""},
		{"plain words", plain(MinWords), MinWords, ""},
		{"comments and blanks", "# header\n\n" + numbered(MinWords), MinWords, ""},
		{"too short", numbered(MinWords - 1), 0, "need at least"},
		{"duplicate", numbered(MinWords) + "99999 w1\n", 0, "duplicate"},
		{"three fields", numbered(MinWords) + "1 2 3\n", 0, "fields"},
		{"control char", numbered(MinWords) + "1 a\x07b\n", 0, "non-printable"},
		{"invalid utf8", numbered(MinWords) + "1 a\xffb\n", 0, "UTF-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := Parse("test", strings.NewReader(tt.input))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("got err %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if l.Len() != tt.want {
				t.Errorf("got %d words, want %d", l.Len(), tt.want)
			}
		})
	}
}

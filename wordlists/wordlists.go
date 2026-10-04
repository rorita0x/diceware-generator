// Package wordlists embeds the bundled diceware word lists.
package wordlists

import _ "embed"

// EFF is the EFF large wordlist (English, 7776 words).
//
//go:embed eff-large-wordlist.txt
var EFF []byte

// German is the German diceware list (7776 words).
//
//go:embed diceware-de.txt
var German []byte

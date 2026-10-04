// Package guide holds the numbers behind the threat-model / entropy page:
// attacker tiers, their guess rates against SHA-256 and Argon2id, and the
// resulting average cracking times.
//
// All rates are order-of-magnitude estimates for offline attacks, meaning
// the attacker has the hash and can guess as fast as their hardware allows.
// They are meant to show the shape of the problem, not to predict exact times.
package guide

import (
	"fmt"
	"math"
	"slices"
)

// Hash is the password hashing scheme protecting the passphrase.
type Hash int

const (
	// SHA256 is a single, unsalted SHA-256: a fast hash. This is the worst
	// realistic case: legacy systems, leaked databases, homemade key
	// derivation.
	SHA256 Hash = iota
	// Argon2id with the second recommended RFC 9106 profile: m=64 MiB, t=3,
	// p=4. Every guess has to fill and re-read 64 MiB of memory, so GPUs
	// and ASICs are limited by memory bandwidth instead of compute.
	Argon2id
)

func (h Hash) String() string {
	if h == Argon2id {
		return "Argon2id (m=64 MiB, t=3, p=4)"
	}
	return "SHA-256 (single, unsalted)"
}

// Tier is a class of attacker with its guess rates in guesses per second.
type Tier struct {
	Name     string
	Hardware string
	SHA256   float64
	Argon2id float64
}

// Rate returns the tier's guesses per second against h.
func (t Tier) Rate(h Hash) float64 {
	if h == Argon2id {
		return t.Argon2id
	}
	return t.SHA256
}

// Tiers lists the attacker classes, from weakest to strongest.
//
// Sources and assumptions:
//   - SHA-256 on one RTX 4090 runs at ~22 GH/s in hashcat (mode 1400), and
//     the RTX 5090 is faster still. We use 2.5e10 per high-end GPU.
//   - Argon2id with 64 MiB is memory-bandwidth bound. One GPU with ~1 TB/s
//     can do at most on the order of 1e3 guesses/s (each guess moves
//     several hundred MB through memory). A CPU manages a few dozen.
//   - Datacenter: ~10,000 such GPUs. Nation state: ~1,000,000 GPUs or
//     custom ASICs. For comparison, the whole Bitcoin network computes
//     ~1e21 SHA-256 per second on hardware that can't be reused for
//     password cracking. It shows what fast hashes allow in principle.
var Tiers = []Tier{
	{"Laptop", "8-core CPU", 2e8, 2e1},
	{"Gamer", "1 high-end GPU (RTX 4090/5090)", 2.5e10, 1e3},
	{"Pro rig", "8 GPUs, ~€20k commercial", 2e11, 8e3},
	{"Datacenter", "~10,000 GPUs", 2.5e14, 1e7},
	{"Nation state", "~1,000,000 GPUs / ASICs", 1e17, 1e9},
}

// StandardBits are the entropy rows shown in the tables.
var StandardBits = []float64{40, 50, 60, 70, 80, 90, 100, 128}

// Rows returns StandardBits with extra inserted in sorted order (unless it
// is already present). The TUI uses this to show the current passphrase.
func Rows(extra float64) []float64 {
	rows := slices.Clone(StandardBits)
	if extra <= 0 || slices.ContainsFunc(rows, func(b float64) bool { return math.Abs(b-extra) < 0.05 }) {
		return rows
	}
	rows = append(rows, extra)
	slices.Sort(rows)
	return rows
}

// CrackSeconds returns the average time to find a passphrase with the given
// entropy at rate guesses per second. On average half the key space has to
// be searched: 2^(bits-1) / rate.
func CrackSeconds(bits, rate float64) float64 {
	return math.Exp2(bits-1) / rate
}

const (
	minute = 60.0
	hour   = 60 * minute
	day    = 24 * hour
	year   = 365.25 * day
	// AgeOfUniverse in years.
	AgeOfUniverse = 1.38e10
)

// FormatDuration renders seconds as a short, human-readable duration.
func FormatDuration(s float64) string {
	switch {
	case s < 1:
		return "instant"
	case s < minute:
		return fmt.Sprintf("%.0f s", s)
	case s < hour:
		return fmt.Sprintf("%.0f min", s/minute)
	case s < day:
		return fmt.Sprintf("%.0f h", s/hour)
	case s < year:
		return fmt.Sprintf("%.0f days", s/day)
	}
	y := s / year
	switch {
	case y < 1000:
		return fmt.Sprintf("%.0f yrs", y)
	case y < 1e6:
		return fmt.Sprintf("%.0fk yrs", y/1e3)
	case y < 1e9:
		return fmt.Sprintf("%.0fM yrs", y/1e6)
	case y < AgeOfUniverse:
		return fmt.Sprintf("%.1fG yrs", y/1e9)
	default:
		return "> universe"
	}
}

// Threat is one threat model with the entropy we recommend for it.
type Threat struct {
	Name    string
	Example string
	MinBits float64
	Words   string // typical word count with a 7776-word list
}

// Threats lists the threat models, from weakest to strongest attacker.
var Threats = []Threat{
	{"Online, rate-limited", "web login with lockout, phone PIN-backed unlock", 40, "4 words"},
	{"Offline, slow KDF", "password manager master key, LUKS2/argon2id, age/scrypt", 70, "6 words"},
	{"Offline, fast hash", "leaked unsalted SHA-256 DB, legacy systems, homemade KDF", 85, "7 words"},
	{"Long-term / nation state", "can't be rotated: PGP/SSH keys, wallets, archives kept for decades", 110, "9–10 words"},
}

// Rating classifies an entropy value for the meter.
func Rating(bits float64) string {
	switch {
	case bits < 60:
		return "weak"
	case bits < 80:
		return "ok"
	case bits < 100:
		return "strong"
	default:
		return "very strong"
	}
}

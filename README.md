# diceware

A terminal diceware passphrase generator written in Go and built with OpSec in mind.
Passphrases are kept in encrypted, locked memory, the history exists only for the
current session, and nothing reaches the disk unless you explicitly save it.

```
  ⚄ diceware

  ╭─────────────────────────────────────────────────╮
  │  example-sample-words-shown-here-only  │
  ╰─────────────────────────────────────────────────╯

  ███████████████████░░░░░░░░░░░░░  77.5 bits · ok
  EFF (en) · 6 words · sep "-" · caps none · digits off

  r/space re-roll • v reveal/hide • c copy • s save to file
  h history • o options • t threat model • q quit

  🔒 memguard enclaves · core dumps off · ptrace/dumpable off · mlockall skipped · clipboard: wl-copy
```

## Features

- **Diceware from `crypto/rand`**: unbiased word selection (rejection sampling, no modulo bias), never `math/rand`.
- **Two built-in word lists**: the [EFF large wordlist](https://www.eff.org/dice) (English) and a German diceware list, each 7776 words (12.9 bits per word). You can add your own list with `--wordlist`.
- **Re-roll** replaces the whole passphrase at once. Single words can't be re-rolled, because picking words by hand reduces entropy.
- **Options**: number of words (3–20), separator (presets or custom), capitalization (none / first letter / ALL CAPS / random per word), and a random digit block glued to the words with no separator: **before** a random word (`alpha-42bravo-charlie`), **after** a random word (`alpha-bravo42-charlie`) or **between** two words in place of the separator (`alpha-bravo42charlie`). Its **length is random within a range you choose**.
- **Live entropy meter**. The estimate is conservative and assumes the attacker knows your word list and settings.
- **Masked by default**: the passphrase shows as `••••` until you press `v`. The mask has a fixed width, so it doesn't reveal the length.
- **Session-only history** of up to 50 passphrases, each one encrypted in memory. You can reveal, copy, save or wipe entries.
- **Copy to clipboard** through `wl-copy` (Wayland) or `xclip`/`xsel` (X11). The clipboard is cleared automatically after 30 s and on exit. An optional *paste once* mode is available.
- **Save to file**: only to a new file (it never overwrites), with mode `0600`.
- **Threat model & entropy guide** inside the app (`t`). It shows how long your current passphrase holds up against SHA-256 and Argon2id, for attackers from a laptop up to a nation state.

## Build & run

Requires Go 1.27+ and Linux (other Unixes work, but without the process hardening).

```sh
go build -o diceware .
./diceware
./diceware --wordlist ~/my-list.txt   # adds a custom list to the options
./diceware --require-mlock            # refuses to start unless no memory can be swapped
```

Custom word lists contain one word per line, either `word` or `index word`
(the standard diceware format). Lines starting with `#` are ignored. Lists must
have at least 1024 unique words without whitespace.

For clipboard support, install `wl-clipboard` (Wayland) or `xclip`/`xsel` (X11).

Run the tests with `go test ./...`.

## Keys

| Screen   | Keys |
|----------|------|
| Main     | `r`/`space` re-roll · `v` reveal/hide · `c` copy · `s` save · `h` history · `o` options · `t` threat model · `q` quit |
| Options  | `↑↓` select · `←→` change · `enter` custom separator · `r` re-roll · `esc` back |
| History  | `↑↓` select · `v` reveal selected · `c` copy · `s` save · `x` wipe all · `esc` back |
| Guide    | `↑↓`/`pgup`/`pgdn` scroll · `esc` back |

`ctrl+c` quits from anywhere.

## Security design

### What the program does

| Measure | Effect |
|---|---|
| [memguard](https://github.com/awnumar/memguard) Enclaves | Every passphrase is encrypted (XSalsa20-Poly1305) while at rest in RAM. It is decrypted only for the moment it is shown, copied or saved, into a `LockedBuffer`: memory that is `mlock`ed (never swapped), surrounded by guard pages and canaries, and wiped after use. |
| Direct assembly | The passphrase is written straight into the locked buffer and never built up as Go strings. The random draws (word indices, digits, the gap position) are cleared right after use. |
| `RLIMIT_CORE = 0` | A crash doesn't produce a core dump containing the memory. |
| `PR_SET_DUMPABLE = 0` | Other processes running as your user can't `ptrace` it or read `/proc/<pid>/mem`. |
| `mlockall(MCL_CURRENT\|MCL_FUTURE\|MCL_ONFAULT)` | Locks the whole process, so nothing of it is ever swapped. This needs `CAP_IPC_LOCK` or an unlimited memlock limit; see [Locking all memory](#locking-all-memory). Without either it's skipped and the footer says so. memguard mlocks the secret buffers in any case. |
| `memguard.Purge()` on exit | Wipes all buffers and destroys the key that protects the enclaves. This also runs on `ctrl+c`, SIGTERM and SIGHUP. |
| Alternate screen | Nothing is left in your terminal's scrollback after you quit. |
| Clipboard over stdin | The secret is never passed in argv (visible in `ps`) or in environment variables. `wl-copy --sensitive` tells KDE Klipper and other clipboard managers that honor the hint to leave it out of their history. |
| Clipboard ownership | The clipboard helper runs as a child process that owns the selection. If you copy something else, it exits on its own. Clearing means ending that process, so we never wipe something you copied later, and we never need to read the clipboard back. |
| No disk I/O | No config file, no log file, no history file. Settings and history last only for the session. |

### Can the RAM be encrypted?

Not completely. A Go program can't encrypt its own heap. What memguard gives us is
the next best thing: secrets are encrypted whenever they aren't actively in use,
and when they are, they sit in locked memory that is wiped afterwards. Known gaps:

- **Rendering creates Go strings.** To draw the passphrase on screen, the TUI
  ([Bubble Tea](https://github.com/charmbracelet/bubbletea) / [Lip Gloss](https://github.com/charmbracelet/lipgloss))
  has to turn it into immutable strings, which the garbage collector frees but
  never zeroes. That's why the passphrase stays masked until you press `v`:
  while it's hidden, no plaintext string is created.
- **The terminal emulator** keeps the shown text in its own memory.
- **The clipboard** is readable by every program in your session while it holds
  the passphrase. Clipboard managers that ignore the sensitive hint can store it
  permanently. Also, *paste once* doesn't mix well with managers that read the
  clipboard right away, because they use up the one paste.
- **Swap and hibernation**: memguard's buffers are never swapped. The rest of
  the process, including rendered strings, is only protected when `mlockall` is
  active (see below). Hibernation writes even locked memory to disk, so don't
  hibernate while the program is running.
- **root** can read any process memory. If your machine is compromised, no
  password generator can help you.

### Locking all memory

The kernel only allows `mlockall` when the process's entire *virtual* size fits
under `RLIMIT_MEMLOCK`. The Go runtime reserves well over 1 GiB of address space,
even though it actually uses only about 10 MB, while the default limit is
8 MiB. So you have two options:

```sh
# Recommended: allow just this binary to lock memory (repeat after every build)
sudo setcap cap_ipc_lock=ep ./diceware

# Alternative: raise the limit for your user in /etc/security/limits.d/memlock.conf
#   youruser  -  memlock  unlimited
```

The capability isn't passed on to the clipboard helper or any other child
process. `MCL_ONFAULT` locks pages only once they are actually used, so the
large address-space reservations don't cost real RAM.

With `--require-mlock`, the program exits with an error (and the `setcap`
command to fix it) instead of running without the lock. To check while it's
running: `grep VmLck /proc/$(pgrep -n diceware)/status` shows the locked
amount.

Locking only covers this process. The terminal emulator holding the
revealed text and the programs you paste into are separate processes. If your
swap is zram only (compressed RAM, the default on many distributions) and
hibernation is off, nothing reaches the disk anyway.

### Saving to a file

A saved file can outlive deletion in backups, filesystem journals, snapshots and
remapped SSD blocks. If you can, copy the passphrase straight into your password
manager instead. When you do save, the file is created with `O_EXCL` (an existing
file is never overwritten), mode `0600`, and `fsync`ed.

## Threat model & entropy guide

Press `t` in the app to see this guide live, with your current passphrase
highlighted in the tables.

Diceware's strength comes **only from the random choice**, never from keeping the
method secret. All numbers below assume the attacker knows your word list and
settings. Every bit of entropy doubles the attacker's work. One word from a
7776-word list is worth 12.9 bits.

### Pick your threat model

| Threat model | Example | Target | Words (EFF/DE) |
|---|---|---|---|
| Online, rate-limited | web login with lockout, PIN-backed device unlock | ≥ 40 bits | 4 |
| Offline, slow KDF | password manager master key, LUKS2 with argon2id, age/scrypt | ≥ 70 bits | 6 |
| Offline, fast hash | leaked unsalted SHA-256 database, legacy systems, homemade KDF | ≥ 85 bits | 7 |
| Long-term / nation state | secrets you can't rotate: PGP/SSH keys, wallets, archives kept for decades | ≥ 110 bits | 9–10 |

### SHA-256 vs. Argon2id

**SHA-256** was designed to be fast. A single gaming GPU running hashcat tries tens
of billions of guesses per second. **Argon2id** (the winner of the Password Hashing
Competition, [RFC 9106](https://www.rfc-editor.org/rfc/rfc9106)) is slow on
purpose and *memory-hard*: with the RFC's second recommended profile
(m = 64 MiB, t = 3, p = 4), every single guess has to fill and re-read 64 MiB of
RAM. GPUs and ASICs are then limited by memory bandwidth instead of raw compute.
Each guess becomes roughly 25 million times more expensive, which is worth about
24 bits, or two extra words, for free.

You rarely get to choose which hash a service uses, so plan for the worst case
you expect.

### Average time to crack

On average, half the key space has to be searched: `2^(bits−1) / guesses per second`.

**SHA-256 (single, unsalted)**

| Entropy | Laptop | Gamer | Pro rig | Datacenter | Nation state |
|---|---|---|---|---|---|
| 40 bits | 46 min | 22 s | 3 s | instant | instant |
| 50 bits | 33 days | 6 h | 47 min | 2 s | instant |
| 60 bits | 91 yrs | 267 days | 33 days | 38 min | 6 s |
| 70 bits | 94k yrs | 748 yrs | 94 yrs | 27 days | 2 h |
| **77.5 bits** (6 words) | 18M yrs | 140k yrs | 18k yrs | 14 yrs | 13 days |
| 80 bits | 96M yrs | 766k yrs | 96k yrs | 77 yrs | 70 days |
| 90 bits | > universe | 785M yrs | 98M yrs | 78k yrs | 196 yrs |
| 100 bits | > universe | > universe | > universe | 80M yrs | 201k yrs |
| 128 bits | > universe | > universe | > universe | > universe | > universe |

**Argon2id (m=64 MiB, t=3, p=4)**

| Entropy | Laptop | Gamer | Pro rig | Datacenter | Nation state |
|---|---|---|---|---|---|
| 40 bits | 871 yrs | 17 yrs | 2 yrs | 15 h | 9 min |
| 50 bits | 892k yrs | 18k yrs | 2k yrs | 2 yrs | 7 days |
| 60 bits | 913M yrs | 18M yrs | 2M yrs | 2k yrs | 18 yrs |
| 70 bits | > universe | > universe | 2.3G yrs | 2M yrs | 19k yrs |
| **77.5 bits** (6 words) | > universe | > universe | > universe | 351M yrs | 4M yrs |
| 80 bits | > universe | > universe | > universe | 1.9G yrs | 19M yrs |
| 90 bits | > universe | > universe | > universe | > universe | > universe |
| 100 bits | > universe | > universe | > universe | > universe | > universe |
| 128 bits | > universe | > universe | > universe | > universe | > universe |

"> universe" means longer than the age of the universe (1.38·10¹⁰ years).

### Attacker tiers

These are order-of-magnitude estimates, in guesses per second.

| Tier | Hardware | SHA-256 /s | Argon2id /s |
|---|---|---|---|
| Laptop | 8-core CPU | 2·10⁸ | 2·10¹ |
| Gamer | 1 high-end GPU (RTX 4090/5090) | 2.5·10¹⁰ | 10³ |
| Pro rig | 8 GPUs, ~€20k commercial | 2·10¹¹ | 8·10³ |
| Datacenter | ~10,000 GPUs | 2.5·10¹⁴ | 10⁷ |
| Nation state | ~1,000,000 GPUs / ASICs | 10¹⁷ | 10⁹ |

Where these come from: an RTX 4090 does about 22 GH/s of SHA-256 in hashcat
(mode 1400). Argon2id at 64 MiB is limited by memory bandwidth, and a GPU with
~1 TB/s can't do much more than ~10³ guesses per second. For scale, the entire
Bitcoin network computes about 10²¹ SHA-256 hashes per second, on ASICs that
can't be repurposed for password cracking. That figure shows what fast hashes
allow in principle.

### Rules of thumb

- **One more word (+12.9 bits) beats any capitalization or digit trick.**
  Capitalizing the first letter or using ALL CAPS adds 0 bits, because the
  attacker knows the rule. Random capitalization per word adds 1 bit per word.
  A random 1–3 digit block glued to one of 6 words adds about 7.5 bits, less
  than a single extra word.
- Hardware keeps getting faster: add about 1 bit for every two years the secret
  has to stay safe.
- Online attacks (rate-limited login forms) are millions of times slower than
  offline ones.
- Never reuse a passphrase. Once it leaks somewhere else, no amount of entropy
  helps.

### How entropy is counted

`words × log2(list size)`, plus `words` bits for random capitalization. For a
digit block of random length L ∈ [min, max], we count
`log2(max − min + 1) + min × log2(10)` (min-entropy: the shortest length is the
attacker's best first guess), plus `log2(words)` for which word it's glued to
(before/after), or `log2(words − 1)` for which gap it fills (between).

## Built with

- [Bubble Tea](https://github.com/charmbracelet/bubbletea): the TUI framework (Elm architecture)
- [Bubbles](https://github.com/charmbracelet/bubbles): text inputs, viewport, key bindings and help
- [Lip Gloss](https://github.com/charmbracelet/lipgloss): styling, layout and tables
- [memguard](https://github.com/awnumar/memguard): encrypted enclaves and locked memory
- [golang.org/x/sys](https://pkg.go.dev/golang.org/x/sys/unix): rlimits, prctl, mlockall

## Word lists

- `wordlists/eff-large-wordlist.txt`: [EFF large wordlist](https://www.eff.org/deeplinks/2016/07/new-wordlists-random-passphrases) by the Electronic Frontier Foundation (CC BY 3.0 US)
- `wordlists/diceware-de.txt`: German diceware list

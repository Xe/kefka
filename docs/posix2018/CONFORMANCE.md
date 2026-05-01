# POSIX 2018 Conformance Audit

> **Audit date:** 2026-04-30
> **Scope:** All 43 utilities documented under `docs/posix2018/*.md`
> plus the POSIX `tee` (registered without a local spec doc) and 17
> non-POSIX extras shipped under `command/internal/`.
> **Method:** read each spec doc, read the matching implementation
> in `command/internal/<pkg>/*.go`, read the matching `_test.go`,
> record deviations, test gaps, and notes with `file:line` and
> spec-section citations.

This document is a starting point, not a finished compliance
certification. Citations are precise enough that any reader can
re-derive each finding by opening the cited file and the cited spec
section. Findings are conservative: where the implementation is too
shallow to fully audit (e.g., `tr`, `ls`, `printf`), the entry says
so rather than claiming completeness.

The README already warns: *"Most of the commands have not been
audited for correctness or POSIX compliance."* This document is the
first audit pass.

## How to read this document

- **Severity** is one of:
  - **None** — no deviations found.
  - **Minor** — extensions beyond POSIX, or small behavioral
    differences that don't affect typical usage.
  - **Major** — missing required options, wrong behavior on
    documented inputs, or output that breaks scripts depending on
    the spec.
  - **Missing** — utility is not implemented at all.
- **Spec options** lists every option the spec mentions plus any
  *extra* options the impl ships. Tag each as `implemented`,
  `missing`, `accepted-but-ignored`, or `extra (not-in-POSIX)`.
- **Required tests** are concrete enough that a reader can drop
  each into a table-driven `_test.go` in under 10 minutes.
- **Citations**: `cat.go:44` is the implementation; `cat.md OPTIONS`
  is the spec section header.

## Table of contents

1. [Summary table](#summary-table)
2. [Cross-cutting concerns](#cross-cutting-concerns)
3. [Missing commands](#missing-commands)
4. [Per-command deep dives](#per-command-deep-dives)
5. [Non-POSIX extras](#non-posix-extras)
6. [Recommendations](#recommendations)

---

## Summary table

Severity legend: ⬜ None · 🟡 Minor · 🟠 Major · ⛔ Missing.
"Tests" is rated against the deviations identified in the deep dive
(adequate ≈ exercises the major spec contract; partial ≈ covers
common path; none ≈ no test file at all).

| Command | Impl | Severity | Tests | Notes |
| --- | --- | --- | --- | --- |
| basename | yes | 🟠 | partial | Ships GNU `-a`/`-s` flags; spec OPTIONS = "None" |
| cal | **no** | ⛔ | n/a | Not registered |
| cat | yes | 🟠 | partial | Ships `-n` (POSIX RATIONALE explicitly omits); missing `-u`; hardcoded errors |
| chmod | **no** | ⛔ | n/a | Not registered. High-priority gap |
| cp | yes | 🟠 | partial | Missing `-i`, `-H`, `-L`, `-P`; `-p` accepted-but-ignored; modes hardcoded |
| cut | yes | 🟠 | adequate | Missing `-b` and `-n`; field deduplication bug |
| date | yes | 🟡 | partial | UTC-only by design (sandbox); `%E`/`%O` modifiers unsupported |
| df | **no** | ⛔ | n/a | Not registered |
| diff | yes | 🟠 | partial | Only `-u` truly works; `-c`/`-C`/`-e`/`-f`/`-r`/`-U`/`-b` missing or stub |
| dirname | yes | ⬜ | adequate | Conformant |
| du | yes | 🟠 | partial | Default unit is 1024 (spec is 512); `-H`/`-L`/`-x` missing |
| expand | yes | 🟠 | adequate | Ships GNU `-i`; tab-stop trailing rule wrong; not locale-aware |
| expr | yes | 🟡 | adequate | Regex is Go-flavored, not BRE; string compare not collation-aware |
| false | yes | 🟠 | **none** | Likely conforming but no test file |
| file | yes | 🟠 | partial | Extension-based only; `-M`/`-m` missing |
| fold | yes | 🟠 | adequate | CR handling wrong; tab-stop indexing wrong; not locale-aware |
| grep | **no** | ⛔ | n/a | Not registered. High-priority gap |
| head | yes | 🟡 | adequate | Ships GNU `-c`/`-q`/`-v` (POSIX RATIONALE omits `-c`); adds trailing newline |
| join | yes | ⬜ | adequate | Conformant |
| ln | **no** | ⛔ | n/a | Not registered |
| ls | yes | 🟠 | partial | Many options unimplemented or marked `_unused`; long-format incomplete |
| mkdir | yes | 🟠 | adequate | `-m mode` missing entirely; intermediate-dir mode logic missing |
| more | **no** | ⛔ | n/a | Not registered |
| mv | yes | 🟠 | adequate | `-i` missing; ships GNU `-n`; no same-file or cross-fs handling |
| nl | yes | 🟠 | adequate | Missing `-d`, `-f`, `-h`, `-l`, `-p`; logical pages not supported |
| od | yes | 🟠 | partial | `-t` parses only `c`/`x1`/`o*`; `-v`/`-j`/`-N` missing |
| paste | yes | 🟠 | adequate | No backslash-escape parsing in `-d list` |
| patch | **no** | ⛔ | n/a | Not registered |
| printf | yes | 🟡 | partial | Adds bash `-v VAR`; otherwise large/complete impl, edge cases unaudited |
| pwd | yes | ⬜ | adequate | Conformant (`-L`/`-P` both honored) |
| rm | yes | 🟡 | adequate | `-i` missing; otherwise correct |
| rmdir | yes | ⬜ | adequate | Conformant; ships GNU `-v` extension |
| sleep | yes | 🟡 | partial | Accepts fractional and `m`/`h`/`d` suffixes (GNU); caps at 1 hour |
| split | yes | ⬜ | adequate | Conformant |
| tail | yes | 🟡 | adequate | `-f` follow mode missing |
| tee | yes | 🟡 | adequate | `-i` (ignore SIGINT) missing. No local spec doc |
| time | yes | 🟠 | partial | GNU-style impl, not POSIX `time`; couples to registry |
| touch | yes | 🟠 | adequate | `-a`/`-m`/`-r`/`-t` accepted-but-ignored; create mode 0o644 (spec: 0o666) |
| tr | yes | 🟠 | partial | No octal/escape parsing; no `[=equiv=]`; no LC_COLLATE; `-c` vs `-C` not split |
| true | yes | 🟠 | **none** | Likely conforming but no test file |
| unexpand | yes | 🟡 | partial | `-a`/`-t` interaction unclear; backspace + last-tabstop edges untested |
| uniq | yes | 🟠 | partial | `-f`/`-s` missing; ships GNU `-i`; output-file operand unsupported |
| wc | yes | 🟠 | adequate | `-c`/`-m` conflated; ASCII-only word splitting; counts bytes for `-m` |
| zcat | yes | ⬜ | adequate | Conformant |

| Extra (non-POSIX) | Impl | Tests | Reference |
| --- | --- | --- | --- |
| base64 | yes | yes | GNU coreutils `base64(1)` |
| checksum (lib) | n/a | no | Internal helper for `md5sum`/`sha1sum`/`sha256sum`; not a registered command |
| clear | yes | yes | `terminfo`/`ncurses` `clear(1)` |
| column | yes | yes | BSD `column(1)` / `util-linux column(1)` |
| gunzip | yes | yes | RFC 1952 / GNU `gunzip(1)` |
| gzip | yes | yes | RFC 1952 / GNU `gzip(1)` |
| hostname | yes | no | BSD `hostname(1)` |
| md5sum | yes | yes | GNU coreutils `md5sum(1)` |
| python3 | yes (wasm) | no | Embedded `python.wasm`; not a coreutil. Not registered with the registry |
| readlink | yes | yes | GNU coreutils `readlink(1)` |
| seq | yes | yes | GNU coreutils `seq(1)` |
| sha1sum | yes | yes | GNU coreutils `sha1sum(1)` |
| sha256sum | yes | yes | GNU coreutils `sha256sum(1)` |
| stat | yes | yes | GNU coreutils `stat(1)` |
| tac | yes | yes | GNU coreutils `tac(1)` |
| tree | yes | yes | BSD/Steve Baker `tree(1)` |
| whoami | yes | yes | GNU coreutils `whoami(1)` |

---

## Cross-cutting concerns

These issues recur across many commands and are worth fixing once
in a shared layer rather than 36 times.

### 1. Locale awareness (none)

POSIX requires nearly every utility to honor `LC_CTYPE`,
`LC_COLLATE`, `LC_MESSAGES`, and `LC_ALL` (see ENVIRONMENT VARIABLES
in any spec doc). Not one implementation reads these env vars.
Concrete consequences:

- `wc.go:201` counts bytes for `-m` instead of characters.
- `wc.go:211` splits words on ASCII whitespace only.
- `expand.go` / `fold.go` / `unexpand.go` assume one byte = one
  column; multibyte and combining-character widths are wrong.
- `tr.go` ranges and character classes ignore collation order.
- `expr.go:150` string compare uses byte equality, not
  `LC_COLLATE`.
- `ls.go` date format and column padding ignore locale.
- `nl.go`, `cut.go` field counting use ASCII delimiters.

**Recommended approach:** add a `command/internal/locale` helper that
reads `ec.Environ` for `LC_*` keys and exposes:

- `IsSpace(r rune) bool` (LC_CTYPE-aware)
- `Width(r rune) int` (column width per LC_CTYPE; default to
  `golang.org/x/text/width` rules)
- `Compare(a, b string) int` (LC_COLLATE-aware; default to
  byte-wise, but pluggable)

Wire commands through it. A test fixture with multibyte input
exercises every consumer at once.

### 2. Hardcoded error messages

Several commands print a fixed string regardless of the underlying
errno:

- `cat.go:94`, `cat.go:100`: always says `"No such file or
  directory"` even on EACCES, EISDIR.
- `wc.go:106`: same pattern.

POSIX is intentionally loose about diagnostic wording, but tests
break when the error doesn't reflect reality. Use
`fmt.Fprintf(stderr, "%s: %s: %s\n", prog, file, err)` with the
underlying error to surface the actual cause.

### 3. Help/usage output stream

GNU convention writes `--help` to **stdout** so it can be piped to
`less`. Most kefka impls write usage to **stderr** (e.g.,
`cat.go:42`). Pick a convention and apply it uniformly. Document
the choice in `command/command.go`.

### 4. `--` end-of-options handling

`getopt/v2` honors `--` automatically. Add at least one test per
command verifying that `cmd -- -file` treats `-file` as an operand
(important for files whose names start with `-`). Currently no
command tests this.

### 5. Stdin / `-` operand

POSIX many utilities treat `-` as stdin. The current impls vary:

- `cat`, `wc`, `head`, `tail`, `cut`, `paste`, `tr`, etc. all
  accept `-`.
- POSIX `cat` specifically requires that stdin not be closed and
  reopened on subsequent `-` operands. The impl reads all of stdin
  on the first `-` and gets EOF afterwards (consistent), but no
  test exercises this.

Add a shared test helper that, for every command accepting file
operands, runs the matrix `[file, -, file -]` against a known
stdin to lock the contract.

### 6. Exit status

Most utilities specify `0` and `>0`. Specific exit codes are
mandated for:

- `cmp`: 0 = same, 1 = differ, >1 = error. (Not implemented.)
- `diff`: 0 = same, 1 = differ, 2 = error. `diff.go:115` returns
  1 on differences (correct).
- `expr`: 0 = nonzero/nonempty, 1 = zero/empty, 2 = invalid expr,
  3 = other error. Verify the impl's mapping (`expr.go:57-59`,
  276) — looks right but should be tested explicitly.
- `grep`: 0 = match, 1 = no match, 2 = error. Not implemented.

Add explicit "exit code on error" tests for these four (only two
are implemented today: `diff` and `expr`).

### 7. Unicode / rune vs byte

Several length/column-counting bugs share a root cause: code uses
`len(string)` (bytes) where the spec wants characters. Touched
commands: `wc -m`, `cut -c`, `fold -w`, `expand`, `unexpand`,
`tr`, `nl -w`. Fix as part of (1) above.

### 8. Help-flag convention

Most commands ship `--help` even when POSIX doesn't mention it.
This is harmless but should be documented in a "kefka extensions"
note next to each affected command, so reviewers don't keep
flagging it as a deviation.

---

## Missing commands

These POSIX 2018 utilities have spec docs in `docs/posix2018/` but
no corresponding implementation in `command/internal/` and no entry
in `command/registry/coreutils/coreutils.go`.

| Command | Spec size | Estimated effort | Why it matters |
| --- | --- | --- | --- |
| `cal` | small (~5KB) | 1–2 days | Calendar printing; rarely used by scripts |
| `chmod` | medium (~21KB) | 3–5 days | Permission management; **critical** for any shell environment. Octal *and* symbolic mode (`u=rwx,g+r`, etc.). |
| `df` | small (~6KB) | 1–2 days | Filesystem usage; partial-info OK in a virtual FS |
| `grep` | large (~15KB) | 1–2 weeks | Search via BRE/ERE; **critical** for scripts. `-E`, `-F`, `-c`, `-l`, `-q`, `-i`, `-n`, `-s`, `-v`, `-x`, `-e`, `-f` all required |
| `ln` | medium (~9KB) | 2–4 days | Hard and symbolic links; needs `billy.Symlink` capability |
| `more` | medium (~10KB) | 3–5 days | Pager. Interactive — possibly out of scope for a batch shell environment |
| `patch` | large (~25KB) | 1–2 weeks | Apply diff output. Complex format support (`-c`, `-u`, `-e`, `-l`, `-N`, `-R`) |

The two most consequential gaps are **`chmod`** and **`grep`** —
their absence means scripted shell workflows that touch
permissions or text search cannot run unmodified.

---

## Per-command deep dives

Sections are alphabetical. Trivially-conforming commands (e.g.,
`dirname`, `pwd`, `rmdir`) appear here only for completeness; their
deep dive is one paragraph.

### `basename`

**Implementation:** `command/internal/basename/basename.go` (95 lines)
**Test file:** `basename_test.go` (158 lines)
**Severity:** 🟠 Major (per strict POSIX) / ⬜ None (per GNU coreutils)

**Spec options:** spec `basename.md` OPTIONS section says **"None"**.

**Deviations:**
- Ships GNU `--multiple`/`-a` (`basename.go:45`). Cites: spec
  `basename.md` OPTIONS.
- Ships GNU `--suffix`/`-s` (`basename.go:46`). Cites: spec
  `basename.md` OPTIONS.

These are GNU coreutils standard but not in POSIX 2018. Decide
policy: keep (and document as kefka/GNU extension) or strip.

**Required tests:**
- `basename /usr/lib` → `lib` (POSIX example).
- `basename /usr/` → `usr` (trailing-slash handling, spec
  DESCRIPTION step 3).
- `basename / ` → `/` (all-slash special case, spec step 2).
- `basename "" ` → `""` or `.` per spec step 1.
- `basename foo.c .c` → `foo` (positional suffix).

**Notes:**
- The two-positional-operand form `basename string suffix` works
  per spec; the GNU extras add a multi-file mode that isn't in
  POSIX.

### `cat`

**Implementation:** `command/internal/cat/cat.go` (148 lines)
**Test file:** `cat_test.go` (175 lines)
**Severity:** 🟠 Major

**Spec options:**
- `-u` (write without delay): **missing**
- `-n`/`--number`: **extra (not-in-POSIX)** — `cat.md` RATIONALE
  explicitly: *"The **-n** option was omitted because similar
  functionality can be obtained from the **-n** option of the
  pr utility."*

**Deviations:**
- Missing `-u` (`cat.md` OPTIONS — the sole option POSIX requires).
- Ships `-n`/`--number` (`cat.go:44`).
- Hardcoded `"No such file or directory"` regardless of the
  underlying errno (`cat.go:94`, `cat.go:100`). EACCES/EISDIR
  cases mis-report.

**Required tests:**
- `cat -u file` is accepted (even if it's a no-op given Go's
  unbuffered `io.Writer` writes).
- `cat -- -file` reads a file literally named `-file`.
- `printf foo | cat - -` → `foo` (second `-` produces no extra
  output; spec OPERANDS).
- `cat <directory>` produces a diagnostic mentioning EISDIR-like
  semantics, not "No such file or directory".

**Notes:**
- Stdout is already direct `io.Writer.Write`, so `-u` is naturally
  honored — accept the flag as a no-op for spec compliance.
- Decide policy on `-n`: drop it, or keep as documented extension.

### `cp`

**Implementation:** `command/internal/cp/cp.go` (198 lines)
**Test file:** `cp_test.go` (298 lines)
**Severity:** 🟠 Major

**Spec options (cp.md):**
- `-f`: **missing** (the impl has GNU `-n` no-clobber instead)
- `-H`: **missing** (follow command-line symlinks only)
- `-L`: **missing** (follow all symlinks)
- `-P`: **missing** (don't follow any symlinks)
- `-i`: **missing** (interactive prompt before overwrite)
- `-p`: **accepted-but-ignored** (`cp.go:55`)
- `-R`: implemented
- `-r`: implemented (alias of `-R`)

**Deviations:**
- `-p` parsed but never preserves anything; mtime/atime/uid/gid/
  mode are not applied to the destination (`cp.go:55`,
  `cp.go:148`, `cp.go:171`).
- Directory creation uses `info.Mode().Perm()` (`cp.go:148`),
  which is source mode — closer to spec, but still doesn't apply
  the file mode creation mask the way `cp.md` step 2e prescribes.
- `copyFile` writes `0o644` (`cp.go:171`). Spec `cp.md` step 3b
  requires source mode.
- `-i` (interactive) and `-f` (force-unlink) are not implemented;
  the impl's `-n` is a GNU-coreutils inversion.
- Three-form synopsis (`cp [-Pfip] src dst`, `cp [-Pfip] src... dir`,
  `cp -R [-H|-L|-P] [...] src... dir`) is not handled — the impl
  collapses these.
- No same-file detection (spec step 1: "If source_file references
  the same file as dest_file, cp may write a diagnostic and
  exit unsuccessfully").

**Required tests:**
- `cp -p src dst` then stat: dst's mtime equals src's mtime.
- `cp -p src dst` then stat: dst's mode equals src's mode (not
  0o644).
- `cp src dst` where `dst` exists: with `-i` should prompt; with
  no flag should overwrite (current behavior).
- `cp -R dir1 dir2` where dir1 contains files with mode 0o600:
  copies preserve mode.
- `cp src dst` where src and dst are the same file: emits
  diagnostic, exits non-zero.
- `cp -- -src dst`: handles file named `-src`.

**Notes:**
- `billy.Filesystem` lacks rich symlink primitives in some
  backends; `-H`/`-L`/`-P` are non-trivial to add and may need
  capability detection on the FS.
- Pre-implementation: clarify policy on `-n` (keep as extension or
  remove).

### `cut`

**Implementation:** `command/internal/cut/cut.go` (250 lines)
**Test file:** `cut_test.go` (233 lines)
**Severity:** 🟠 Major

**Spec options (cut.md):**
- `-b list`: **missing** (byte-cutting)
- `-c list`: implemented
- `-d delim`: implemented
- `-f list`: implemented
- `-n`: **missing** (multibyte safety modifier for `-b`)
- `-s`: implemented (as `--only-delimited`)

**Deviations:**
- No `-b` option; only `-c` and `-f` are usable selectors.
  Cites: `cut.go:48-51`, spec `cut.md` OPTIONS.
- No `-n` option (would gate `-b` against splitting multibyte
  sequences). Cites: spec `cut.md` OPTIONS lines 65-88.
- `-b`, `-c`, `-f` are mutually exclusive per spec; impl never
  rejects conflicting selectors. Cites: `cut.go:64-81`.
- `extractByRanges` (`cut.go:169`) deduplicates by value via a
  map. Spec EXTENDED DESCRIPTION says fields are output once per
  selection and in input order; repeated selections (`-f 1,1`)
  should still output the field once but not silently dedup
  unrelated equal values across positions.

**Required tests:**
- `printf 'a\tb\tc' | cut -b 1-3` → first three bytes (currently:
  not implemented, errors).
- `printf 'a\tb\tc' | cut -c 1` and `cut -f 1` together — should
  reject as conflict.
- `printf 'aXa\n' | cut -d X -f 1,1` → `aXa` should not
  collapse the two `a`s.
- `printf "λa\n" | cut -c 1` → `λ` (one character, two bytes).

**Notes:**
- `-b` and `-n` together are non-trivial; a clean
  implementation needs locale-aware character iteration (see
  cross-cutting #1).

### `date`

**Implementation:** `command/internal/date/date.go` (231 lines)
**Test file:** present
**Severity:** 🟡 Minor (deliberate sandbox choices)

**Spec options:** `-u` (UTC), `+FORMAT` (operand).

**Deviations:**
- `date.go:70-73` documents that the sandbox always renders UTC,
  ignoring `TZ`. POSIX says `-u` makes the implementation use UTC;
  by default, `TZ` controls. The impl's UTC-always policy is
  reasonable for a sandbox but should be documented as a kefka
  invariant, not a bug.
- Format modifiers `%E` and `%O` (locale alternatives) — POSIX
  says they're optional; if unsupported, the unmodified spec
  applies. Verify the impl falls back rather than erroring.
- XSI extension to set system time via positional `[mmddHHMM[CC]yy[.ss]]`
  is not implemented (out of scope for a sandbox).

**Required tests:**
- `date -u +'%Y-%m-%d'` produces a parseable date string.
- `date +'%Z'` returns `UTC` (matches sandbox invariant).
- `date +'%E%Y'` does not error (locale modifier fallback).
- All conversion specs in spec `date.md` lines 65-240 produce
  defined output.

**Notes:**
- The "always UTC" sandbox decision is fine if explicit. Add a
  one-line comment in `date.go` linking to this audit so future
  contributors don't "fix" it.

### `diff`

**Implementation:** `command/internal/diff/diff.go` (173 lines)
**Test file:** `diff_test.go` (207 lines)
**Severity:** 🟠 Major

**Spec options (diff.md):**
- `-b`: parsed (`diff.go:86`) but underscored-out (unused)
- `-c`: **missing**
- `-C n`: **missing**
- `-e`: **missing**
- `-f`: **missing**
- `-r`: **missing**
- `-u`: implemented (and made the de facto default — see below)
- `-U n`: **missing**

**Deviations:**
- The default output format (no format flag) should be the
  traditional `Nc`, `Na`, `Nd` ed-style format. Impl always
  outputs unified format (`diff.go:102-115`). Cites: spec
  `diff.md` STDOUT.
- `-b` (ignore whitespace amount) is parsed but not applied
  (`diff.go:86`).
- Recursive directory comparison (`-r`) is not implemented.
- `-q` (brief) and `-s` (report-identical) are GNU additions;
  POSIX `diff.md` does specify `-q` indirectly through
  exit-code semantics but not as a flag — however these are
  widely accepted GNU extensions.

**Required tests:**
- `diff a b` with default output produces traditional format
  (currently produces unified).
- `diff -c a b` produces context format.
- `diff -e a b` produces an `ed` script.
- `diff -b a b` ignores whitespace differences.
- Exit status: 0 = same, 1 = differ, 2 = error (`diff.md`
  EXIT STATUS). Verify the 2-on-error path with a missing file.

**Notes:**
- The "default = unified" choice is convenient for humans but
  breaks scripts piping diff output to `patch` (which expects
  the traditional or `-c`/`-u` format explicitly).

### `dirname`

**Implementation:** `command/internal/dirname/dirname.go` (83 lines)
**Test file:** `dirname_test.go` (135 lines)
**Severity:** ⬜ None

**Notes:** correctly handles all five POSIX `dirname` transformation
steps. Add tests for the edge cases `dirname //`, `dirname ""`,
`dirname /usr/bin/` if not already present.

### `du`

**Implementation:** `command/internal/du/du.go` (230 lines)
**Test file:** present
**Severity:** 🟠 Major

**Spec options (du.md):**
- `-a`: implemented
- `-H`: **missing** (follow command-line symlinks)
- `-L`: **missing** (follow all symlinks)
- `-k`: implemented (1024-byte units; matches GNU default)
- `-s`: implemented
- `-x`: **missing** (skip files on different filesystems)
- `--max-depth`: **extra (GNU, not-in-POSIX)**

**Deviations:**
- Default unit is 1024 bytes (`du.go:194`); spec `du.md` line 33
  says default is **512-byte units** (`-k` switches to 1024).
  This is a behavioral break for any script parsing `du` output.
- `-H`, `-L`, `-x` missing entirely.

**Required tests:**
- `du file` (no `-k`) reports size in 512-byte units. (Today this
  test would *fail* — the bug.)
- `du -k file` reports size in 1024-byte units.
- `du -H` follows only command-line symlinks.

**Notes:**
- The 512 vs 1024 default is the kind of cross-cutting "POSIX
  vs GNU" choice that should be made explicitly: either honor
  POSIX strictly, or ship a `BLOCKSIZE` env var (POSIX-allowed)
  letting users pick.

### `expand`

**Implementation:** `command/internal/expand/expand.go` (233 lines)
**Test file:** `expand_test.go` (232 lines)
**Severity:** 🟠 Major

**Spec options (expand.md):**
- `-t tablist`: implemented
- `-i`/`--initial`: **extra (GNU, not-in-POSIX)**

**Deviations:**
- `-i` is a GNU addition (`expand.go:49`). spec OPTIONS lists
  only `-t`.
- Tab-stop trailing rule is wrong: when input has tabs past the
  last specified stop, spec says emit a **single space**;
  `expand.go:121-129` extrapolates using the last interval.
- Column counting is byte-based (`expand.go:164-173`). Multibyte
  characters and combining marks count incorrectly.

**Required tests:**
- `printf 'a\t\t\tx' | expand -t 4,8` → past column 8, each tab
  becomes a single space, not an 8-wide tab.
- `printf 'λ\tx' | expand` → `λ` is one column (depending on
  locale); the tab stop after it should land at column 8, not 9.
- `expand -i 'leading\tword\ttrailing'` should be rejected (or
  documented as a kefka extension).

### `expr`

**Implementation:** `command/internal/expr/expr.go` (429 lines)
**Test file:** `expr_test.go` (318 lines)
**Severity:** 🟡 Minor

**Spec options:** none (all behavior is via operators).

**Deviations:**
- `:` (regex match) uses Go regex, not POSIX BRE. Cites:
  `expr.go:266`, spec `expr.md` EXTENDED DESCRIPTION lines 173-188.
  POSIX BRE differs from Go regex in: `\(`/`\)` for grouping
  (Go uses `(...)`), `*` literal at start, `\{n,m\}` interval.
- String compare (`=`, `<`, `>`, etc.) uses byte equality, not
  collation order (`expr.go:150`).
- Exit status mapping appears correct: 0 = nonzero/nonempty,
  1 = zero/empty, 2 = invalid expression. Verify code 3 (other
  error) is also present.

**Required tests:**
- `expr 'abc' : 'a\(b\)c'` → `b` (BRE grouping).
- `expr ' ' = '	'` (space vs tab) — collation-aware result
  in locales where they sort differently.
- `expr 1 + 'foo'` → exit 2.
- `expr 0` → exit 1 (zero is "false" exit code).
- `expr ''` → exit 1.

**Notes:**
- The BRE→Go-regex translation could live in a shared helper
  (also useful for the missing `grep` and for `sed` if ever
  added).

### `false`

**Implementation:** `command/internal/falsecmd/falsecmd.go` (15 lines)
**Test file:** **none**
**Severity:** 🟠 Major (only because of test gap)

**Notes:**
- The implementation is correct: returns `interp.ExitStatus(1)`
  unconditionally.
- The package is named `falsecmd` because `false` is reserved
  in Go expressions; the registry name is still `false`
  (`coreutils.go:74`).
- **Missing test file**. POSIX `false.md` says: ignore arguments,
  exit non-zero, no output. Add a one-line test exercising this.

**Required tests:**
- `false` → exit 1, empty stdout, empty stderr.
- `false foo bar baz` → exit 1, empty stdout, empty stderr.

### `file`

**Implementation:** `command/internal/file/file.go` (sampled ~80 lines)
**Test file:** present
**Severity:** 🟠 Major

**Spec options (file.md):**
- `-d`: spec mentions; impl behavior unclear from sample
- `-h`: present, behavior on symlinks unclear
- `-i` (mime): present
- `-M file`: **missing**
- `-m file`: **missing**

**Deviations:**
- `-M` and `-m` (custom magic test files) are completely missing.
  Spec `file.md` lines 74-94 define a layered system where
  multiple `-M`/`-m`/`-d` may be passed, applied in order.
- Detection logic is essentially extension-based, not the
  content-heuristic-based test sequence the spec mandates
  (`file.md` DESCRIPTION steps 1-6).

**Required tests:**
- `file unknown.bin` (binary content, no extension) → reports
  "data" or similar non-text classification.
- `file -i` (or `--mime`) returns MIME types per RFC 2046.
- `file -h symlink` does not dereference; `file symlink` does.

**Notes:**
- A truly POSIX-compliant `file` requires a magic-number engine.
  This is a substantial body of work; consider documenting
  kefka's `file` as best-effort and not POSIX-compliant.

### `fold`

**Implementation:** `command/internal/fold/fold.go` (235 lines)
**Test file:** `fold_test.go` (229 lines)
**Severity:** 🟠 Major

**Spec options (fold.md):** `-b` (bytes), `-s` (break at spaces),
`-w width`. All implemented.

**Deviations:**
- Carriage return handling: spec says `\r` resets the column
  count to 0 (DESCRIPTION). Impl treats `\r` as decrement-by-1,
  i.e. backspace semantics (`fold.go:167-170`).
- Tab-stop arithmetic uses `8 - (col % 8)` (`fold.go:168`,
  0-indexed). Spec defines tab stops at columns where
  `(col mod 8) == 1` (1-indexed): columns 1, 9, 17, ...
- Per-character width assumed to be 1 (`utf8.EncodeRune` returns
  byte count but width is treated as 1 per rune). Multibyte and
  combining characters wrong.

**Required tests:**
- `printf 'a\rb' | fold -w 5` → after `\r`, column resets to 0;
  `b` is at column 0, total length under 5.
- `printf '\tab' | fold -w 4` → `\t` advances to column 8,
  forcing a fold immediately. With current impl the off-by-one
  produces a different fold point.
- Multibyte: `printf 'λλλ' | fold -w 3` (East-Asian wide chars)
  folds after one or two characters depending on width definition.

### `head`

**Implementation:** `command/internal/head/head.go` (254 lines)
**Test file:** `head_test.go` (223 lines)
**Severity:** 🟡 Minor

**Spec options:** `-n number` (only POSIX option).

**Deviations:**
- `-c`/`--bytes` (`head.go:50`) is a GNU addition. spec `head.md`
  RATIONALE explicitly: *"There is no -c option (as there is in
  tail) because it is not historical practice."*
- `-q`/`--quiet` and `-v`/`--verbose` are GNU additions.
- `head` adds a trailing newline if the input lacks one
  (`head.go:194`). Spec STDOUT says copy "in entirety" —
  preserve the absence of a final newline.

**Required tests:**
- `printf 'a\nb\nc' | head -n 2` → `a\nb` (no trailing newline).
- `head -- -n2` reads a file named `-n2`.

**Notes:**
- `-c`/`-q`/`-v` are universally expected today; consider
  documenting them as kefka extensions rather than removing.

### `join`

**Implementation:** `command/internal/join/join.go` (324 lines)
**Test file:** `join_test.go` (222 lines)
**Severity:** ⬜ None

**Notes:**
- All POSIX options (`-1`, `-2`, `-a`, `-e`, `-o`, `-t`, `-v`)
  are implemented and tested. Spot-check: missing-field handling
  with `-e` looks correct.
- Verify the spec-required behavior of refusing unsorted input
  (`join.md` says behavior is unspecified if not sorted; impl
  could continue silently, which is allowed).

### `ls`

**Implementation:** `command/internal/ls/ls.go` (large; sampled
first 100 lines)
**Test file:** present
**Severity:** 🟠 Major

**Spec options (ls.md is ~37KB, the longest spec):**

| Flag | Status | Notes |
| --- | --- | --- |
| `-A` | implemented | line 62 |
| `-C` | unimplemented | multi-column output |
| `-F` | implemented | type indicators `*/=>@` |
| `-H` | unimplemented | command-line symlink follow |
| `-L` | unimplemented | follow all symlinks |
| `-R` | implemented | line 68 |
| `-S` | implemented | sort by size |
| `-a` | implemented | line 61 |
| `-c` | unimplemented | use ctime |
| `-d` | implemented | line 63 |
| `-f` | unimplemented | unsorted, implies -a |
| `-h` | implemented | human-readable sizes |
| `-i` | unimplemented | inode |
| `-k` | unimplemented | 1024-byte blocks |
| `-l` | partially | long format incomplete |
| `-m` | unimplemented | comma-separated |
| `-n` | unimplemented | numeric uid/gid |
| `-o` | unimplemented | long without group |
| `-p` | unimplemented | type slash on dirs |
| `-q` | unimplemented | non-printable as `?` |
| `-r` | implemented | line 67 |
| `-s` | unimplemented | block count |
| `-t` | parsed but unused | `_` underscore (line 83) |
| `-u` | unimplemented | use atime |
| `-x` | unimplemented | column order |
| `-1` | parsed but unused | `_` underscore (line 84) |

**Deviations:**
- `sortByTime` and `onePerLine` are accepted at the flag layer
  but never wired to behavior (`ls.go:83-84`). They look like
  the work was started and not completed.
- Long format (`-l`) has incomplete implementations of the spec's
  STDOUT format (file mode bits including suid/sgid `s`/`S` and
  sticky `t`/`T`, the date format that switches between
  `Mon DD HH:MM` and `Mon DD  YYYY` based on age, locale-aware
  date formatting).

**Required tests:**
- `ls -t` orders by mtime (currently unwired).
- `ls -1` outputs one entry per line.
- `ls -l` on a file with mode `04755` shows `-rwsr-xr-x`
  (suid bit display).
- `ls -l` on a directory with sticky bit shows `t` or `T`.
- `ls -l` on a recently-modified file shows `Mon DD HH:MM`;
  on a 1-year-old file shows `Mon DD  YYYY`.

**Notes:**
- `ls` is the largest single-utility audit item. The cleanest
  path is probably to (a) finish wiring the parsed-but-unused
  flags first, then (b) tackle the `-l` long-format STDOUT
  spec section by section.

### `mkdir`

**Implementation:** `command/internal/mkdir/mkdir.go` (123 lines)
**Test file:** `mkdir_test.go` (251 lines)
**Severity:** 🟠 Major

**Spec options (mkdir.md):**
- `-m mode`: **missing entirely**
- `-p`: implemented (line 48)

**Deviations:**
- `-m mode` is one of only two POSIX options and is missing.
  Spec lines 35-43.
- Created directories use hardcoded `0o755` (`mkdir.go:88`);
  spec says "absolute pathname's permissions modified by the
  file mode creation mask".
- Intermediate directories (created via `-p`) have no
  permissions logic. Spec mkdir.md line 49-51 / RATIONALE 168-173
  prescribes intermediate-dir mode of `(S_IWUSR|S_IXUSR|~filemask)`
  before they receive the final mode.

**Required tests:**
- `mkdir -m 0700 newdir`, then stat: mode is 0o700.
- `mkdir -m u=rwx,g=rx,o= newdir` (symbolic mode).
- `mkdir -p a/b/c` with restrictive umask: intermediates get
  `u+wx` for traversal, leaf gets the requested mode.
- `mkdir existing_dir` exits non-zero with "File exists" error.
- `mkdir -p existing_dir` is silent and exits 0 (today: works).

**Notes:**
- Symbolic mode parsing is non-trivial (`u+rwx,g-w,o=`); shares
  logic with the missing `chmod`. Implement once.

### `mv`

**Implementation:** `command/internal/mv/mv.go` (152 lines)
**Test file:** `mv_test.go` (299 lines)
**Severity:** 🟠 Major

**Spec options (mv.md):**
- `-f`: implemented (line 49)
- `-i`: **missing**
- `-n`: **extra (GNU, not-in-POSIX)** (line 50)

**Deviations:**
- `-i` (interactive prompt) missing. Spec `mv.md` DESCRIPTION
  step 1 mandates a prompt when (a) destination exists and is
  not writable, *and* stdin is a terminal, or (b) `-i` is set.
- "Same file" detection (spec DESCRIPTION step 2) missing —
  `mv a b` where b hard-links to a should produce a diagnostic.
- Cross-filesystem move fallback (spec steps 4-7: copy hierarchy,
  preserve metadata, then unlink source) is not implemented.
  Impl just calls `Rename` (line 118), which fails across FSes.
- Mode/uid/gid preservation on cross-fs moves: not implemented.
- `mv.go:64-68` makes `-n` take precedence over `-f`. POSIX
  `mv.md` says "the last specified shall take precedence";
  `-n` is GNU so this is a kefka invariant, but document it.

**Required tests:**
- `mv -i src dst` with dst existing prompts y/n.
- `mv samefile linkedfile` (both same inode) emits diagnostic.
- Cross-fs mv: pre-populate two billy memfs roots and `mv`
  across them; verify the file ends up at dst with src removed
  and metadata preserved.

### `nl`

**Implementation:** `command/internal/nl/nl.go` (267 lines)
**Test file:** `nl_test.go` (225 lines)
**Severity:** 🟠 Major

**Spec options (nl.md):**
- `-b type`: implemented
- `-d delim`: **missing** (logical-page delimiters)
- `-f type`: **missing** (footer numbering)
- `-h type`: **missing** (header numbering)
- `-i incr`: implemented
- `-l num`: **missing** (consecutive-blank-line grouping)
- `-n format`: implemented
- `-p`: **missing** (don't reset numbering at page breaks)
- `-s sep`: implemented
- `-v startnum`: implemented
- `-w width`: implemented

**Deviations:**
- 5 of 11 spec options are missing.
- The "logical page" concept (header/body/footer separated by
  `\:\:\:`, `\:\:`, `\:` markers) is unimplemented. Without it,
  most of POSIX `nl`'s behavior cannot be exercised.
- Separator-suppression rule (when a line is not numbered, its
  number-and-separator field should be blanks of the same width)
  partially implemented (line 170 emits spaces + the separator
  even when unnumbered).

**Required tests:**
- Input with `\:\:\:` (header), `\:\:` (body), `\:` (footer)
  delimiters: numbering resets per section per `-h`/`-b`/`-f`.
- `-p` keeps numbering across page breaks.
- `-l 3` collapses 3+ consecutive empty lines into one numbered
  unit.
- Unnumbered line: leading field is `width + sep_len` worth of
  spaces, not `width` spaces + separator.

### `od`

**Implementation:** `command/internal/od/od.go` (223 lines)
**Test file:** present
**Severity:** 🟠 Major

**Spec options (od.md):**
- `-A` (address base): implemented
- `-t` (type spec): partial — only `c`, `x1`, `o*` parsed
  (`od.go:74-78`)
- `-v` (verbose, no compression): **missing**
- `-j` (skip): **missing**
- `-N` (count): **missing**
- `-b`/`-c`/`-d`/`-o`/`-s`/`-x` (XSI shorthands): only `-c`
  appears (`od.go:57`)

**Deviations:**
- `-t` accepts a tiny subset; full grammar (`a`, `c`, `d`, `f`,
  `o`, `u`, `x`, with optional size suffix or `C`/`S`/`I`/`L`,
  comma-separated lists) unsupported.
- Repeated identical 16-byte lines should be collapsed to a `*`
  unless `-v` is given; impl always emits all lines (no
  compression mode at all).
- 16-bytes-per-line layout hardcoded (`od.go:139`); spec allows
  variable block sizes derived from `-t`.

**Required tests (extending the existing `od_test.go`):**
- `printf '%s' "AB" | od -t d2` → 2-byte signed decimal value.
- `printf 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' | od` → repeated
  blocks collapsed to `*`.
- `od -j 4 -N 8 file` → skips 4 bytes, reads next 8.
- `od -A d` → addresses in decimal.

**Notes:**
- The existing test file likely covers only the small subset of
  `-t` formats the impl supports; expand it as the
  implementation grows.

### `paste`

**Implementation:** `command/internal/paste/paste.go` (214 lines)
**Test file:** `paste_test.go` (193 lines)
**Severity:** 🟠 Major

**Spec options (paste.md):** `-d list`, `-s`. Both implemented.

**Deviations:**
- `-d list` does not parse backslash escapes. Spec says `\n`,
  `\t`, `\\`, `\0` are recognized (`paste.md` OPTIONS); impl
  treats every char as literal (`paste.go:133-152`).
- Empty `-d ""` is silently accepted; spec says behavior with
  empty list is unspecified — fine to allow but document.
- Circular reset of the delimiter list: spec says reset to first
  element after each output line in non-`-s` mode, and after
  each *file* in `-s` mode. The non-`-s` case is correct;
  `-s` reset rule untested.

**Required tests:**
- `paste -d '\t' a b` outputs columns separated by literal
  tab (currently outputs literal `\` + `t`).
- `paste -d 'a\nb' f1 f2 f3` cycles `a`, `\n`, `b`, then `a`,
  ...
- `paste -s -d 'X' f` joins f's lines with X; reset to X at
  end-of-file when next file is processed.

### `printf`

**Implementation:** `command/internal/printf/printf.go` (1228 lines!)
**Test file:** present
**Severity:** 🟡 Minor

**Spec options:** none in POSIX. Impl ships bash's `-v VAR`
(target a variable instead of stdout) — that's a bash builtin
extension, not a POSIX `printf`(1) option.

**Deviations:**
- The `-v VAR` flag conflicts with POSIX `printf VAR ...` where
  `VAR` would be a format string starting with `-`. Spec
  recommends users prefix with `--` for unambiguous parsing —
  verify this works.
- Format string reuse rule (`printf.md` lines 155-161: "format
  shall be reused as often as necessary to satisfy the argument
  operands") looks correct (`printf.go:96-110`).
- `%b` (escape interpretation per spec lines 123-149) and
  `%c` (single-character) are the historical rough edges. Audit
  with explicit tests.

**Required tests:**
- `printf '%b' '\\n'` → newline.
- `printf '%b' '\\c'` → terminate, no further output.
- `printf '%d %d\n' 1 2 3 4` → `1 2\n3 4\n` (format reuse).
- `printf '%5.3s' hello` → `  hel` (precision truncates).
- `printf '%(%Y)T' 0` → `1970` (POSIX strftime conversion;
  optional but commonly supported).
- `printf -- '-v=stuff' ` (literal dash-v as format).

**Notes:**
- 1228 lines is large; most of the surface is likely correct.
  Focus tests on `%b`, `%c`, width/precision, multi-arg reuse.

### `pwd`

**Implementation:** `command/internal/pwd/pwd.go` (136 lines)
**Test file:** present
**Severity:** ⬜ None

**Notes:**
- `-L` and `-P` both implemented (`pwd.go:45-46`).
- Realpath logic with symlink resolution at `pwd.go:99-135`.
- Verify `-L` falls back to physical when `$PWD` is invalid
  (spec `pwd.md` DESCRIPTION step 1).

### `rm`

**Implementation:** `command/internal/rm/rm.go` (155 lines)
**Test file:** `rm_test.go` (286 lines)
**Severity:** 🟡 Minor

**Spec options:** `-f` (line 53), `-i` (**missing**), `-R` /
`-r` (lines 51-52).

**Deviations:**
- `-i` (interactive prompt) missing. POSIX `rm.md` step 2b
  requires a prompt when (file is not writable AND stdin is
  terminal) AND `-f` is not set; `-i` makes the prompt
  unconditional.
- Symlink handling (`rm.md` step 2c says "shall not traverse
  directories by following symbolic links into other parts of
  the file hierarchy") looks correct (`rm.go:51`).

**Required tests:**
- `rm -i file` prompts before removing.
- `rm readonly_file` (no `-f`, stdin is terminal) prompts.
- `rm -r dir_with_symlink_to_outside` does not follow the
  symlink.

### `rmdir`

**Implementation:** `command/internal/rmdir/rmdir.go` (154 lines)
**Test file:** `rmdir_test.go` (282 lines)
**Severity:** ⬜ None

**Notes:**
- `-p` correctly removes parent dirs only when they become
  empty (`rmdir.go:48`, tested at `rmdir_test.go:130-182`).
- GNU `-v` (verbose) is shipped; document as kefka extension.

### `sleep`

**Implementation:** `command/internal/sleep/sleep.go` (123 lines)
**Test file:** present
**Severity:** 🟡 Minor

**Spec options (sleep.md):** "None".

**Deviations:**
- Accepts multiple operands and sums them (`sleep.go:62-76`).
  Spec `sleep.md` OPERANDS says one operand. Multiple is a GNU
  extension.
- Accepts fractional seconds (`sleep.go:21` `\d+\.?\d*`). Spec
  says "non-negative decimal integer".
- Accepts `m`/`h`/`d` suffixes. Spec says seconds only.
- Caps sleep at one hour (`sleep.go:19` `maxSleep = time.Hour`).
  Spec RATIONALE says implementations must accept up to
  2147483647 seconds (~68 years). The cap is a kefka invariant —
  document it.

**Required tests:**
- `sleep 0` returns immediately, exit 0.
- `sleep abc` exits non-zero with diagnostic.
- `sleep 1.5` — kefka extension, document.
- `sleep 7200` (2 hours) — would currently cap at 1 hour. Either
  fix or document.

### `split`

**Implementation:** `command/internal/split/split.go` (345 lines)
**Test file:** `split_test.go` (448 lines)
**Severity:** ⬜ None

**Notes:**
- Comprehensive impl; `-l`, `-a`, `-b` all present and tested.
- `-` operand for stdin: verify.

### `tail`

**Implementation:** `command/internal/tail/tail.go` (291 lines)
**Test file:** `tail_test.go` (238 lines)
**Severity:** 🟡 Minor

**Spec options (tail.md):**
- `-c number`: implemented
- `-n number`: implemented
- `-f`: **missing** (follow mode)

**Deviations:**
- `-f` is the most-requested missing feature. POSIX `tail.md`
  says: "if the input file is a regular file or if the *file*
  operand specifies a FIFO, do not terminate after the last
  line of the input file has been copied, but read and copy
  further bytes from the input file when they become
  available."
- Trailing-newline addition in fromLine mode (`tail.go:208-211`)
  mirrors the same bug as `head`. Preserve absence of final
  newline.

**Required tests:**
- `tail -f file` continues reading as file grows (challenging
  to test deterministically; needs the harness to write to the
  file from another goroutine).
- `printf 'a\nb\nc' | tail -n 1` → `c` (no trailing newline).

### `tee`

**Implementation:** `command/internal/tee/tee.go` (126 lines)
**Test file:** `tee_test.go` (197 lines)
**Severity:** 🟡 Minor

**Note:** **No local spec doc** at `docs/posix2018/tee.md`,
even though POSIX 2018 includes `tee` (XCU §2.30, 9699919799).

**Spec options:** `-a` (append, implemented), `-i` (ignore SIGINT,
**missing**).

**Deviations:**
- No SIGINT handler (`tee.go` lacks `signal.Notify`). When the
  shell interrupts, tee dies with the rest; in real tee, `-i`
  causes it to keep running until stdin EOFs.

**Required tests:**
- `tee -i file` continues to write after a simulated SIGINT.
  Harder than typical table-driven tests; consider a separate
  signal-handling test file.

**Action:** Add `docs/posix2018/tee.md` from upstream POSIX text
and delete this "no spec doc" note.

### `time`

**Implementation:** `command/internal/time/time.go` (220+ lines)
**Test file:** present
**Severity:** 🟠 Major

**Spec options (time.md per spec):** `-p` portable format; the
spec is short.

**Deviations:**
- Impl ships `-f`, `-o`, `-a`, `-v` (GNU `/usr/bin/time` flags),
  not POSIX `time` (a shell-builtin spec).
- `Impl` struct holds `Registry` (`time.go:23-26`) and dispatches
  to a registered command. POSIX `time COMMAND ARGS` invokes
  the command directly via `execvp`-style search. The registry
  coupling is fine for an in-process shell environment but
  diverges from any user expectation set by the spec.
- A `time_test.go` file exists; verify it exercises the spec
  surface (real/user/sys reporting, `-p` portable format,
  exit-status propagation) and not just GNU extras.

**Required tests:**
- `time true` writes a three-line `real`/`user`/`sys` summary to
  stderr (or, with `-p`, the portable format).
- `time false`: exit status is 1 (propagated from `false`).
- `time` with no command: behavior per spec is to print usage —
  what does the impl do? Verify.

### `touch`

**Implementation:** `command/internal/touch/touch.go` (167 lines)
**Test file:** `touch_test.go` (306 lines)
**Severity:** 🟠 Major

**Spec options (touch.md):**
- `-a`: **accepted-but-ignored** (line 54)
- `-c`: implemented (line 52)
- `-d date`: implemented (line 53; lenient parser)
- `-h`: not in spec; needed only on systems with symlinks
- `-m`: **accepted-but-ignored** (line 55)
- `-r ref_file`: **accepted-but-ignored** (line 56)
- `-t [[CC]YY]MMDDhhmm[.SS]`: **accepted-but-ignored** (line 57)

**Deviations:**
- `-a`/`-m` are no-ops; both atime and mtime are always set
  (`touch.go:110`).
- `-r` and `-t` accept arguments but never consult them.
- Default file creation mode is `0o644` (`touch.go:96`); spec
  says `S_IRUSR|S_IWUSR|S_IRGRP|S_IWGRP|S_IROTH|S_IWOTH` (0o666)
  modified by umask.
- `-d` (`touch.go:127-143`) accepts RFC3339 etc., looser than
  spec lines 73-107 which define a strict
  `YYYY-MM-DDThh:mm:SS[.frac][tz]` format.

**Required tests:**
- `touch -a file`, then stat: mtime unchanged, atime updated.
- `touch -m file`, then stat: atime unchanged, mtime updated.
- `touch -r ref new`, then stat: new's times equal ref's.
- `touch -t 199501010100.30 new` parses to 1995-01-01 01:00:30.
- New file created mode (umask 022): 0o644 today; should be
  0o644 (= 0o666 & ~0o022) — coincidentally correct under that
  umask, but wrong under others. Fix to compute from umask.

**Notes:**
- The four accepted-but-ignored flags should either be wired
  up or removed (silently ignoring user intent is worse than
  rejecting the flag).

### `tr`

**Implementation:** `command/internal/tr/tr.go` (sampled ~80 lines;
likely larger)
**Test file:** present (sampled)
**Severity:** 🟠 Major

**Spec options:** `-c` / `-C` (complement), `-d` (delete),
`-s` (squeeze).

**Deviations:**
- No octal escape parsing (`\nnn`); spec mandates these (`tr.md`
  EXTENDED DESCRIPTION).
- No backslash-escape parsing (`\n`, `\t`, `\\`, `\r`, etc.).
- `[=equiv=]` equivalence classes unsupported.
- `[:class:]` character classes likely unsupported beyond a
  hardcoded set; LC_CTYPE not consulted.
- Range expansion (`a-z`) is byte-based; spec says "characters
  in the current locale collation sequence" (LC_COLLATE).
- `-c` and `-C` are not distinguished; `-c` complement is "by
  numeric byte value", `-C` is "by character per LC_CTYPE".

**Required tests:**
- `printf 'A' | tr '\101' 'a'` → `a` (octal 101 = 'A').
- `printf 'a\nb' | tr '\n' ' '` → `a b`.
- `tr '[=e=]' E` (equivalence class) — locale-dependent.
- `tr '[:lower:]' '[:upper:]'`.
- `tr -c 'a-z' 'X'` (complement-by-value).
- `tr -C 'a-z' 'X'` (complement-by-character — different in
  multibyte locales).

**Notes:**
- A real `tr` is substantial work. Document current impl as
  "ASCII-only, GNU-compatible subset; not full POSIX".

### `true`

**Implementation:** `command/internal/truecmd/truecmd.go` (14 lines)
**Test file:** **none**
**Severity:** 🟠 Major (only because of test gap)

**Notes:**
- Returns `nil` (zero exit). Correct per `true.md`.
- Registered as `true` (`coreutils.go:106`).
- **Missing test file**.

**Required tests:**
- `true` → exit 0, empty stdout, empty stderr.
- `true ignored args here` → still exit 0.

### `unexpand`

**Implementation:** `command/internal/unexpand/unexpand.go` (~100
lines sampled)
**Test file:** present
**Severity:** 🟡 Minor

**Spec options (unexpand.md):**
- `-a`: implemented (line 49)
- `-t tablist`: implemented (line 50)

**Deviations:**
- `-t` should *imply* `-a` per spec OPTIONS: "When `-t` is
  specified, the presence or absence of the `-a` option shall
  be ignored". Verify the impl does this — current code tracks
  the two flags independently.
- Beyond-last-tabstop rule: spec says "no \<space\>-to-\<tab\>
  conversions shall occur for characters at positions beyond
  the last of those specified in a multiple tab-stop list".
- Backspace handling: column never decrements below 1.

**Required tests:**
- `unexpand -t 4` (no `-a`) on `'    a    b    c'` converts
  every leading space-quad to a tab, even after non-blanks
  (spec's `-t` overrides leading-only behavior).
- `unexpand -a -t 4 '...beyond_last_stop spaces...'` leaves
  trailing spaces untouched.
- Backspace input never produces column < 1.

### `uniq`

**Implementation:** `command/internal/uniq/uniq.go` (~100 lines
sampled)
**Test file:** present
**Severity:** 🟠 Major

**Spec options (uniq.md):**
- `-c`: implemented
- `-d`: implemented
- `-f fields`: **missing**
- `-s chars`: **missing**
- `-u`: implemented
- `-i`: **extra (GNU, not-in-POSIX)** (`uniq.go:50`)

**Deviations:**
- `-f` (skip first N fields when comparing) and `-s` (skip first
  N chars) are core POSIX options and missing.
- The `[input_file [output_file]]` operand pair is collapsed:
  impl treats both positionals as input files (`uniq.go:63-64`).
  Spec says the second operand is the output file.

**Required tests:**
- `printf 'x foo\ny foo\n' | uniq -f 1` → `x foo\n` (treats
  as same line because field 1 is identical).
- `printf 'AAfoo\nBBfoo\n' | uniq -s 2` → `AAfoo\n`.
- `uniq input.txt output.txt` writes deduplicated content to
  output.txt.

### `wc`

**Implementation:** `command/internal/wc/wc.go` (268 lines)
**Test file:** `wc_test.go` (225 lines)
**Severity:** 🟠 Major

**Spec options (wc.md):** `-c` (bytes), `-l` (lines),
`-m` (chars), `-w` (words). All accepted.

**Deviations:**
- `-c` and `-m` are conflated: `showChars := *bytesFlag ||
  *charsFlag` (`wc.go:78`). Spec says they are independent;
  in a multibyte locale they differ.
- `s.chars = len(content)` (`wc.go:201`) counts bytes, not
  characters, so `-m` returns the byte count.
- Word splitting checks only ASCII whitespace `' '`, `'\t'`,
  `'\r'`, `'\n'` (`wc.go:211`). Spec says
  "characters defined by the LC_CTYPE category as white-space".

**Required tests:**
- `printf 'λλλ' | wc -c` → 6 (bytes).
- `printf 'λλλ' | wc -m` → 3 (characters).
- `printf 'λλλ' | wc -c -m` → both shown distinctly. Today
  this would show the same number.
- `wc -w` on input with no-break space (U+00A0): currently
  treated as part of a word; in en_US.UTF-8 locale, NBSP is
  whitespace.

### `zcat`

**Implementation:** `command/internal/zcat/zcat.go` (248 lines)
**Test file:** `zcat_test.go` (255 lines)
**Severity:** ⬜ None

**Notes:**
- Decompresses gzip; correct for the tiny POSIX `zcat.md`
  surface area (which is essentially `gunzip -c`).
- GNU extras (`-f`, `-l`, `-q`, `-S`, `-t`, `-v`) are
  conventionally accepted.

---

## Non-POSIX extras

These commands have no POSIX 2018 spec; the audit is against the
de-facto reference for each.

### `base64` (376 lines, tested)
Reference: GNU coreutils `base64(1)`. Standard `-d`/`--decode`,
`-w`/`--wrap`. No deep audit performed; implementation looks
straightforward (uses `encoding/base64`).

### `checksum` (222 lines, **no test file**, **not registered**)
Internal helper module shared by `md5sum`, `sha1sum`,
`sha256sum`. Not exposed to the registry. The shared parser of
the `<hash>  <file>` BSD-tag and GNU formats is the obvious
test target.

### `clear` (142 lines, tested)
Reference: `terminfo`/ncurses `clear(1)`. Likely emits a fixed
ANSI sequence; there's no deep spec.

### `column` (505 lines, tested)
Reference: BSD `column(1)` / `util-linux column(1)`. Behavior
between BSD and util-linux differs (`-t`, `-s`, `-c`, `-x`).
Pick one and document it.

### `gunzip` (702 lines, tested)
Reference: RFC 1952 + GNU `gunzip(1)`. `gzip` and `gunzip`
are the largest extras. Verify multi-member gzip handling.

### `gzip` (920 lines, tested)
Reference: RFC 1952 + GNU `gzip(1)`. Largest single extra. The
RFC defines the byte format precisely; GNU adds many CLI flags.

### `hostname` (17 lines, **no test file**)
Reference: BSD `hostname(1)`. Probably just returns
`os.Hostname()` — verify. No tests.

### `md5sum` (298 lines, tested)
Reference: GNU coreutils `md5sum(1)`. Uses `command/internal/checksum`.

### `python3` (65 lines + `python.wasm`, **no test file**, **not
registered**)
A wasm shim. Not part of POSIX or GNU coreutils; out of scope
for the conformance audit. Confirm whether it should be exposed
via the registry — currently it isn't.

### `readlink` (377 lines, tested)
Reference: GNU coreutils `readlink(1)` (BSD's is simpler).
Verify `-f` (canonicalize), `-e`, `-m`, `-n` semantics match
the GNU variant the size suggests.

### `seq` (388 lines, tested)
Reference: GNU coreutils `seq(1)`. Common script tool;
not in POSIX. `-f format`, `-s separator`, `-w` (equal width).

### `sha1sum` (104 lines, tested)
Reference: GNU coreutils `sha1sum(1)`. Thin wrapper around
`checksum`.

### `sha256sum` (104 lines, tested)
Reference: GNU coreutils `sha256sum(1)`. Thin wrapper around
`checksum`.

### `stat` (419 lines, tested)
Reference: GNU coreutils `stat(1)` (BSD `stat(1)` differs
substantially). Format string `-c`/`--format` and `--printf`
are GNU-specific. Document which variant is targeted.

### `tac` (304 lines, tested)
Reference: GNU coreutils `tac(1)` (cat reversed). `-r`, `-s`,
`-b` are non-trivial to implement correctly — verify.

### `tree` (488 lines, tested)
Reference: Steve Baker's `tree(1)`. Many flags (`-L`, `-d`,
`-f`, `-a`, `-J`, `-X`, etc.); audit against the upstream
man page if conformance matters.

### `whoami` (150 lines, tested)
Reference: GNU coreutils `whoami(1)`. Small surface; the impl
is 56 lines, the test 94 lines.

---

## Recommendations

In priority order:

1. **Add tests for `true` and `false`** — these are the only
   implementations with no `_test.go` at all. Even a one-line
   "exits 0/1 with no output" test prevents regressions.

2. **Decide and document the GNU-extension policy.** Many
   commands ship GNU-coreutils options that POSIX doesn't list
   (`cat -n`, `head -c`/`-q`/`-v`, `uniq -i`, `mv -n`,
   `basename -a`/`-s`, `sleep` fractional, etc.). Three options:
   1. Strip them (strict POSIX).
   2. Keep them, add a `## kefka extensions` section in each
      relevant docs/posix2018 doc.
   3. Track them in a single `EXTENSIONS.md` next to this file.

   Whichever you pick, do it once, uniformly.

3. **Fix `wc` and `du` defaults** — these are the "scripts will
   break" tier:
   - `wc -m` must count characters, not bytes
     (`wc.go:201`).
   - `wc -c` and `-m` must be independent
     (`wc.go:78`).
   - `du` default block size must be 512, not 1024
     (`du.go:194`). Or expose `BLOCKSIZE` env var.

4. **Implement `chmod` and `grep`** — the two missing utilities
   that scripts rely on most. `grep` needs a BRE↔Go-regex
   translator that `expr` could share.

5. **Wire up accepted-but-ignored flags or remove them.**
   `touch -a`/`-m`/`-r`/`-t`, `cp -p`, `diff -b` all silently
   ignore user intent today. Either implement or reject.

6. **Build a shared `command/internal/locale` helper** (see
   cross-cutting #1) and route `wc`, `tr`, `expand`, `unexpand`,
   `fold`, `cut`, `nl`, `expr` through it.

7. **Build a `command/posixtest` shared harness** that, for any
   registered command accepting file operands, runs the
   `[--, -, multiple --]` matrix automatically. Catches
   end-of-options and stdin-`-` regressions across the whole
   coreutils set.

8. **Document the sandbox invariants explicitly** — the "always
   UTC" choice in `date.go:70-73` and the "1-hour cap" in
   `sleep.go:19` are real product decisions that future
   contributors will otherwise mistake for bugs.

9. **Add `docs/posix2018/tee.md`** — `tee` is in POSIX 2018
   but has no local spec doc.

10. **Consider exit-code semantics for `expr`** — verify
    `expr.go` returns code 1 (not 0) when the result is the
    string `"0"` or empty. The current logic looks right
    (`expr.go:57-59`) but lacks an explicit test.

---

## Appendix: methodology checklist

When adding a new command to `command/internal/`, the following
checklist would prevent the recurrent issues found here:

- [ ] Read every section of the POSIX spec doc, especially
      RATIONALE (which calls out historical options POSIX
      *omitted*).
- [ ] Implement every option in OPTIONS or document its absence
      in this file.
- [ ] Do not add GNU options without a comment explaining
      why and a corresponding entry in the kefka-extensions
      doc.
- [ ] Use locale-aware character/whitespace/collation helpers,
      not byte-level operations.
- [ ] Surface the actual error (`%w` it through), not a
      hardcoded "No such file or directory".
- [ ] Write a test for: zero operands, multiple operands, `-`
      stdin operand, `--` end-of-options, and the spec's
      EXIT STATUS section.
- [ ] If a flag is accepted but not implemented, return an
      error rather than silently ignoring it.

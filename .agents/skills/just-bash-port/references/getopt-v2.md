# pborman/getopt/v2 cookbook

The just-bash sources use a custom `parseArgs` shape; map it onto
`github.com/pborman/getopt/v2` using the patterns below.

## Setup

```go
import "github.com/pborman/getopt/v2"

set := getopt.New()
set.SetProgram("<name>")
set.SetParameters("[FILE]")     // shows up after Usage: <name> [opts]
set.SetUsage(usage)             // function called on parse errors
```

`getopt.New()` returns a fresh `*Set`; never use the package-level
default (`getopt.Bool`, `getopt.Parse`) inside an `Execer` because those
mutate global state shared across calls.

## Mapping just-bash flag types

| just-bash `argDefs` | getopt/v2 method                                              |
|---------------------|---------------------------------------------------------------|
| `type: "boolean"`   | `set.BoolLong(long, short, helpvalue)`                        |
| `type: "number"`    | `set.IntLong(long, short, default, helpvalue)`                |
| `type: "string"`    | `set.StringLong(long, short, default, helpvalue)`             |
| `type: "string[]"`  | `set.ListLong(long, short, helpvalue)` (returns `*[]string`)  |
| short-only          | drop the long form, use `set.Bool(rune, helpvalue)` etc.      |
| long-only           | pass rune `0` for the short-name argument                     |

All `*Long` methods return a pointer; dereference after parsing.

## Parsing

```go
err := set.Getopt(append([]string{"<name>"}, args...), nil)
```

- `args[0]` must be the program name. The user's args (which already
  have `argv[0]` stripped by the kefka registry) need a synthetic
  prefix.
- `nil` for the second arg = "no per-option callback".
- On parse failure `err` is non-nil and getopt has *already* called the
  usage function once. You typically want to print a one-line error
  prefix (`"<name>: <err>"`) and return `interp.ExitStatus(1)` (or
  `2` for usage errors — pick one and stay consistent inside the file).

```go
if err := set.Getopt(append([]string{"<name>"}, args...), nil); err != nil {
    fmt.Fprintf(stderr, "<name>: %s\n", err)
    usage()
    return interp.ExitStatus(1)
}
```

## Usage function

`SetUsage(func())` overrides what getopt prints on error. Write the
usage text by hand instead of relying on `set.PrintUsage` — the
auto-formatter aligns columns differently from the just-bash source,
and the goal of the port is byte-for-byte parity with the TS help.

```go
usage := func() {
    fmt.Fprint(stderr, "Usage: <name> [OPTION]... [FILE]\n")
    fmt.Fprint(stderr, "<one-line summary copied from the TS help block>\n\n")
    fmt.Fprint(stderr, "  -d, --decode      decode data\n")
    fmt.Fprint(stderr, "  -w, --wrap=COLS   wrap encoded lines after COLS character (default 76, 0 to disable)\n")
    fmt.Fprint(stderr, "      --help        display this help and exit\n")
}
set.SetUsage(usage)
```

Always print to **stderr**, not stdout — pipelines should not be
poisoned by help text.

## Help flag

`getopt/v2` does not auto-handle `--help`. Add it explicitly:

```go
help := set.BoolLong("help", 0, "display this help and exit")
// ... after parse:
if *help {
    usage()
    return nil
}
```

Use rune `0` to mean "no short alias" — `-h` typically aliases
`--human-readable` in coreutils-style commands, so leaving it free is
correct.

## Positionals

After `Getopt` succeeds, `set.Args()` returns the leftover positional
arguments. This is the equivalent of just-bash's
`parsed.result.positional`.

```go
files := set.Args()
```

## Mixed short/long combinations the parser accepts

Per the package docs:

- `-d` / `--decode` (single bool)
- `-dw 10` (combined: `-d` flag + `-w 10`)
- `-w10` / `-w 10` / `--wrap=10` / `--wrap 10` (all equivalent)
- `--` ends option parsing; everything after is positional

Tests should cover at least the long-form (`--wrap=10`) and
short-with-value (`-w 10`) variants if the command takes values, since
those are the two forms most likely to be wired wrong.

## Avoid

- `getopt.Parse()` (package-level) — mutates global state, breaks
  re-entrancy
- `set.SetOptional()` — needed only for value-optional flags, which
  just-bash never uses
- Calling `set.PrintUsage` from inside `usage` — it produces a
  different layout than the TS help, defeating the whole point of
  setting a custom usage function

## Example end-to-end

For a hypothetical `wc -lwc [FILE]...`:

```go
set := getopt.New()
set.SetProgram("wc")
set.SetParameters("[FILE]...")

usage := func() {
    fmt.Fprint(stderr, "Usage: wc [OPTION]... [FILE]...\n")
    fmt.Fprint(stderr, "Print newline, word, and byte counts for each FILE.\n\n")
    fmt.Fprint(stderr, "  -c, --bytes   print the byte counts\n")
    fmt.Fprint(stderr, "  -l, --lines   print the newline counts\n")
    fmt.Fprint(stderr, "  -w, --words   print the word counts\n")
    fmt.Fprint(stderr, "      --help    display this help and exit\n")
}
set.SetUsage(usage)

bytesFlag := set.BoolLong("bytes", 'c', "print the byte counts")
linesFlag := set.BoolLong("lines", 'l', "print the newline counts")
wordsFlag := set.BoolLong("words", 'w', "print the word counts")
help      := set.BoolLong("help",  0,  "display this help and exit")

if err := set.Getopt(append([]string{"wc"}, args...), nil); err != nil {
    fmt.Fprintf(stderr, "wc: %s\n", err)
    usage()
    return interp.ExitStatus(1)
}
if *help {
    usage()
    return nil
}

files := set.Args()
_ = bytesFlag; _ = linesFlag; _ = wordsFlag; _ = files
```

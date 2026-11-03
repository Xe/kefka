package find

import (
	"io/fs"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// evalCtx is what an expression can see about one entry. isEmpty is a
// function so that a directory is only read for -empty when that test is
// actually reached.
type evalCtx struct {
	name    string
	path    string
	typ     fs.FileMode // type bits only
	isFile  bool
	isDir   bool
	isEmpty func() bool
	mtime   time.Time
	size    int64
	mode    fs.FileMode
	now     time.Time
	newer   map[string]time.Time
}

// evalResult carries the match, whether -prune fired, and every action
// reached along the way, in expression order.
type evalResult struct {
	matches bool
	pruned  bool
	actions []*action
}

func (e *expr) eval(c *evalCtx) evalResult {
	switch e.kind {
	case exprName:
		return evalResult{matches: matchGlob(e.pattern, c.name, e.ignoreCase)}
	case exprPath:
		return evalResult{matches: matchGlob(e.pattern, c.path, e.ignoreCase)}
	case exprRegex:
		return evalResult{matches: e.re.MatchString(c.path)}
	case exprType:
		return evalResult{matches: fileTypeLetter(c.typ) == e.fileType}
	case exprEmpty:
		return evalResult{matches: c.isEmpty()}
	case exprMtime:
		// POSIX: whole 24-hour periods since find started, remainder
		// discarded, compared with n for +n, n, and -n alike.
		days := int64(math.Floor(float64(c.now.Sub(c.mtime)) / float64(24*time.Hour)))
		return evalResult{matches: compare(days, e.cmp, e.value)}
	case exprNewer:
		ref, ok := c.newer[e.refPath]
		return evalResult{matches: ok && c.mtime.After(ref)}
	case exprSize:
		// POSIX: without a suffix (or with b), the size is counted in
		// 512-byte blocks, rounded up. With c, in bytes. The k, M, and G
		// extensions compare bytes, as just-bash does.
		switch e.unit {
		case 'b':
			return evalResult{matches: compare((c.size+511)/512, e.cmp, e.value)}
		case 'c':
			return evalResult{matches: compare(c.size, e.cmp, e.value)}
		}
		return evalResult{matches: compare(c.size, e.cmp, e.value*sizeMultiplier(e.unit))}
	case exprPerm:
		fileMode := unixMode(c.mode)
		want := e.value & 0o7777
		switch e.perm {
		case permAll:
			return evalResult{matches: fileMode&want == want}
		case permAny:
			return evalResult{matches: fileMode&want != 0}
		}
		return evalResult{matches: fileMode == want}
	case exprPrune:
		return evalResult{matches: true, pruned: true}
	case exprTrue:
		return evalResult{matches: true}
	case exprAction:
		return evalResult{matches: true, actions: []*action{e.act}}
	case exprNot:
		inner := e.left.eval(c)
		return evalResult{matches: !inner.matches, pruned: inner.pruned, actions: inner.actions}
	case exprAnd:
		left := e.left.eval(c)
		if !left.matches {
			return left
		}
		right := e.right.eval(c)
		return evalResult{
			matches: right.matches,
			pruned:  left.pruned || right.pruned,
			actions: concatActions(left.actions, right.actions),
		}
	case exprOr:
		left := e.left.eval(c)
		if left.matches {
			return left
		}
		right := e.right.eval(c)
		return evalResult{
			matches: right.matches,
			pruned:  left.pruned || right.pruned,
			actions: concatActions(left.actions, right.actions),
		}
	}
	return evalResult{}
}

func compare(got int64, cmp comparison, n int64) bool {
	switch cmp {
	case cmpMore:
		return got > n
	case cmpLess:
		return got < n
	}
	return got == n
}

// fileTypeLetter maps type bits to the -type letter POSIX uses for them.
func fileTypeLetter(typ fs.FileMode) byte {
	switch {
	case typ.IsRegular():
		return 'f'
	case typ&fs.ModeDir != 0:
		return 'd'
	case typ&fs.ModeSymlink != 0:
		return 'l'
	case typ&fs.ModeNamedPipe != 0:
		return 'p'
	case typ&fs.ModeSocket != 0:
		return 's'
	case typ&fs.ModeCharDevice != 0:
		return 'c'
	case typ&fs.ModeDevice != 0:
		return 'b'
	}
	return 0
}

// unixMode returns the permission bits of m as a Unix mode, including
// setuid (04000), setgid (02000), and sticky (01000).
func unixMode(m fs.FileMode) int64 {
	mode := int64(m.Perm())
	if m&fs.ModeSetuid != 0 {
		mode |= 0o4000
	}
	if m&fs.ModeSetgid != 0 {
		mode |= 0o2000
	}
	if m&fs.ModeSticky != 0 {
		mode |= 0o1000
	}
	return mode
}

func concatActions(a, b []*action) []*action {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	out := make([]*action, 0, len(a)+len(b))
	return append(append(out, a...), b...)
}

// walk calls fn on e and every sub-expression.
func (e *expr) walk(fn func(*expr)) {
	if e == nil {
		return
	}
	fn(e)
	e.left.walk(fn)
	e.right.walk(fn)
}

// needsStat reports whether evaluating e needs size, mtime, or mode,
// which directory entries alone do not give.
func (e *expr) needsStat() bool {
	found := false
	e.walk(func(x *expr) {
		switch x.kind {
		case exprEmpty, exprMtime, exprNewer, exprSize, exprPerm:
			found = true
		}
	})
	return found
}

func (e *expr) actions() []*action {
	var out []*action
	e.walk(func(x *expr) {
		if x.kind == exprAction {
			out = append(out, x.act)
		}
	})
	return out
}

func (e *expr) newerRefs() []string {
	var out []string
	e.walk(func(x *expr) {
		if x.kind == exprNewer {
			out = append(out, x.refPath)
		}
	})
	return out
}

// matchGlob matches name against a shell pattern the way fnmatch(3) does
// without FNM_PATHNAME or FNM_PERIOD: '*' and '?' also match '/' and a
// leading '.'. Supports '*', '?', '[...]' (with '!' or '^' negation,
// ranges, and named classes such as [:digit:]), and backslash escapes. An
// unterminated '[' is a literal.
func matchGlob(pattern, name string, ignoreCase bool) bool {
	if ignoreCase {
		pattern = strings.ToLower(pattern)
		name = strings.ToLower(name)
	}
	p := []rune(pattern)
	s := []rune(name)

	pi, si := 0, 0
	starP, starS := -1, 0
	for si < len(s) {
		if pi < len(p) && p[pi] == '*' {
			starP, starS = pi, si
			pi++
			continue
		}
		if pi < len(p) {
			if width, ok := matchOne(p[pi:], s[si], ignoreCase); ok {
				pi += width
				si++
				continue
			}
		}
		if starP < 0 {
			return false
		}
		starS++
		si = starS
		pi = starP + 1
	}
	for pi < len(p) && p[pi] == '*' {
		pi++
	}
	return pi == len(p)
}

// matchOne matches a single non-'*' pattern element at the start of p
// against c, returning how many pattern runes it used.
func matchOne(p []rune, c rune, fold bool) (int, bool) {
	switch p[0] {
	case '?':
		return 1, true
	case '\\':
		if len(p) > 1 {
			return 2, p[1] == c
		}
		return 1, c == '\\'
	case '[':
		if width, matched, ok := matchBracket(p, c, fold); ok {
			return width, matched
		}
		return 1, c == '['
	}
	return 1, p[0] == c
}

// matchBracket matches a bracket expression at the start of p. ok is false
// when the bracket is not closed.
func matchBracket(p []rune, c rune, fold bool) (width int, matched, ok bool) {
	i := 1
	negate := false
	if i < len(p) && (p[i] == '!' || p[i] == '^') {
		negate = true
		i++
	}
	first := true
	for {
		if i >= len(p) {
			return 0, false, false
		}
		if p[i] == ']' && !first {
			break
		}
		first = false

		if p[i] == '[' && i+1 < len(p) && p[i+1] == ':' {
			if end := classEnd(p, i+2); end >= 0 {
				if inClass(string(p[i+2:end]), c, fold) {
					matched = true
				}
				i = end + 2
				continue
			}
		}

		lo := p[i]
		if lo == '\\' && i+1 < len(p) {
			i++
			lo = p[i]
		}
		i++
		hi := lo
		if i+1 < len(p) && p[i] == '-' && p[i+1] != ']' {
			i++
			hi = p[i]
			if hi == '\\' && i+1 < len(p) {
				i++
				hi = p[i]
			}
			i++
		}
		if lo <= c && c <= hi {
			matched = true
		}
	}
	return i + 1, matched != negate, true
}

// classEnd returns the index of the ":" that closes a "[:name:]" whose
// name starts at start, or -1.
func classEnd(p []rune, start int) int {
	for j := start; j+1 < len(p); j++ {
		if p[j] == ':' && p[j+1] == ']' {
			return j
		}
		if !unicode.IsLower(p[j]) {
			return -1
		}
	}
	return -1
}

// inClass reports whether c is in the named POSIX character class. An
// unknown name matches nothing, as with fnmatch(3). With fold set (for
// -iname and -ipath, where the name is already lowercase), [:upper:] and
// [:lower:] match letters of either case.
func inClass(name string, c rune, fold bool) bool {
	if fold && (name == "upper" || name == "lower") {
		return unicode.IsUpper(c) || unicode.IsLower(c)
	}
	switch name {
	case "alnum":
		return unicode.IsLetter(c) || unicode.IsDigit(c)
	case "alpha":
		return unicode.IsLetter(c)
	case "blank":
		return c == ' ' || c == '\t'
	case "cntrl":
		return unicode.IsControl(c)
	case "digit":
		return c >= '0' && c <= '9'
	case "graph":
		return unicode.IsPrint(c) && c != ' '
	case "lower":
		return unicode.IsLower(c)
	case "print":
		return unicode.IsPrint(c)
	case "punct":
		return unicode.IsPunct(c) || unicode.IsSymbol(c)
	case "space":
		return unicode.IsSpace(c)
	case "upper":
		return unicode.IsUpper(c)
	case "xdigit":
		return c < utf8.RuneSelf && isHex(byte(c))
	}
	return false
}

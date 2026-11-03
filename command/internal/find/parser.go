package find

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// maxSafeInteger mirrors JavaScript's Number.MAX_SAFE_INTEGER, which
// just-bash uses as the upper bound for every numeric operand.
const maxSafeInteger = 1<<53 - 1

type exprKind int

const (
	exprName exprKind = iota
	exprPath
	exprRegex
	exprType
	exprEmpty
	exprMtime
	exprNewer
	exprSize
	exprPerm
	exprPrune
	exprTrue // -xdev, which only changes traversal
	exprAction
	exprNot
	exprAnd
	exprOr
)

type comparison int

const (
	cmpExact comparison = iota
	cmpMore
	cmpLess
)

type permMatch int

const (
	permExact permMatch = iota // MODE
	permAll                    // -MODE
	permAny                    // /MODE
)

type actionKind int

const (
	actPrint actionKind = iota
	actPrint0
	actPrintf
	actDelete
	actExec
)

// action is a side effect reached while evaluating an expression. Batch
// -exec actions are grouped by pointer identity, so each parsed -exec
// gets its own *action.
type action struct {
	kind    actionKind
	format  string   // -printf
	command []string // -exec
	batch   bool     // -exec ... +
}

type expr struct {
	kind exprKind

	pattern    string         // -name, -path, -regex
	ignoreCase bool           // -iname, -ipath, -iregex
	re         *regexp.Regexp // -regex, compiled at parse time

	fileType byte // -type: one of b c d f l p s

	cmp   comparison // -mtime, -size
	value int64      // -mtime days, -size count, -perm mode
	unit  byte       // -size: c, k, M, G, b

	perm permMatch // -perm

	refPath string // -newer

	act *action // -print, -print0, -printf, -delete, -exec

	left, right *expr // -not uses left only
}

// options is the parsed command line minus the starting points.
type options struct {
	expr       *expr
	maxDepth   int // -1 when unset
	minDepth   int
	depthFirst bool
	xdev       bool
	help       bool
}

type tokenKind int

const (
	tokExpr tokenKind = iota
	tokAnd
	tokOr
	tokNot
	tokLParen
	tokRParen
)

type token struct {
	kind tokenKind
	expr *expr
}

// parseExpression parses everything after the starting points. On failure
// it returns the exact stderr line (with trailing newline) to print.
func parseExpression(args []string) (*options, string) {
	opts := &options{maxDepth: -1}
	var tokens []token

	missingArgument := func(pred string) string {
		return fmt.Sprintf("find: missing argument to `%s'\n", pred)
	}
	invalidArgument := func(pred, value string) string {
		return fmt.Sprintf("find: invalid argument `%s' to `%s'\n", value, pred)
	}
	push := func(e *expr) {
		tokens = append(tokens, token{kind: tokExpr, expr: e})
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// operand returns the value following a predicate.
		hasOperand := i+1 < len(args)
		operand := func() string {
			i++
			return args[i]
		}

		switch arg {
		case "(", "\\(":
			tokens = append(tokens, token{kind: tokLParen})
		case ")", "\\)":
			tokens = append(tokens, token{kind: tokRParen})
		case "--help":
			opts.help = true
			return opts, ""
		case "-name", "-iname", "-path", "-ipath":
			if !hasOperand {
				return nil, missingArgument(arg)
			}
			kind := exprName
			if arg == "-path" || arg == "-ipath" {
				kind = exprPath
			}
			push(&expr{kind: kind, pattern: operand(), ignoreCase: arg[1] == 'i'})
		case "-regex", "-iregex":
			if !hasOperand {
				return nil, missingArgument(arg)
			}
			pattern := operand()
			ignoreCase := arg == "-iregex"
			re, err := compileRegex(pattern, ignoreCase)
			if err != nil {
				return nil, invalidArgument(arg, pattern)
			}
			push(&expr{kind: exprRegex, pattern: pattern, ignoreCase: ignoreCase, re: re})
		case "-type":
			if !hasOperand {
				return nil, missingArgument(arg)
			}
			fileType := operand()
			if len(fileType) != 1 || !strings.Contains("bcdflps", fileType) {
				return nil, fmt.Sprintf("find: Unknown argument to -type: %s\n", fileType)
			}
			push(&expr{kind: exprType, fileType: fileType[0]})
		case "-empty":
			push(&expr{kind: exprEmpty})
		case "-mtime":
			if !hasOperand {
				return nil, missingArgument(arg)
			}
			raw := operand()
			cmp, digits := splitComparison(raw)
			days, ok := parseSafeUint(digits)
			if !ok {
				return nil, invalidArgument(arg, raw)
			}
			push(&expr{kind: exprMtime, cmp: cmp, value: days})
		case "-newer":
			if !hasOperand {
				return nil, missingArgument(arg)
			}
			push(&expr{kind: exprNewer, refPath: operand()})
		case "-size":
			if !hasOperand {
				return nil, missingArgument(arg)
			}
			raw := operand()
			e, ok := parseSize(raw)
			if !ok {
				return nil, invalidArgument(arg, raw)
			}
			push(e)
		case "-perm":
			if !hasOperand {
				return nil, missingArgument(arg)
			}
			raw := operand()
			e, ok := parsePerm(raw)
			if !ok {
				return nil, invalidArgument(arg, raw)
			}
			push(e)
		case "-prune":
			push(&expr{kind: exprPrune})
		case "-xdev":
			// Applies to the whole walk wherever it appears, like -depth.
			opts.xdev = true
			push(&expr{kind: exprTrue})
		case "-not", "!":
			tokens = append(tokens, token{kind: tokNot})
		case "-o", "-or":
			tokens = append(tokens, token{kind: tokOr})
		case "-a", "-and":
			tokens = append(tokens, token{kind: tokAnd})
		case "-maxdepth", "-mindepth":
			if !hasOperand {
				return nil, missingArgument(arg)
			}
			raw := operand()
			depth, ok := parseSafeUint(raw)
			if !ok || depth > int64(^uint(0)>>1) {
				return nil, invalidArgument(arg, raw)
			}
			if arg == "-maxdepth" {
				opts.maxDepth = int(depth)
			} else {
				opts.minDepth = int(depth)
			}
		case "-depth":
			opts.depthFirst = true
		case "-exec":
			// ";" always ends the command. "+" ends it only right after a
			// bare "{}"; anywhere else it is an ordinary argument.
			var command []string
			batch := false
			i++
			for ; i < len(args); i++ {
				if args[i] == ";" {
					break
				}
				if args[i] == "+" && len(command) > 0 && command[len(command)-1] == "{}" {
					batch = true
					break
				}
				command = append(command, args[i])
			}
			if i >= len(args) || len(command) == 0 {
				return nil, missingArgument("-exec")
			}
			if batch && countBraces(command) != 1 {
				return nil, invalidArgument("-exec", "+")
			}
			push(&expr{kind: exprAction, act: &action{kind: actExec, command: command, batch: batch}})
		case "-print":
			push(&expr{kind: exprAction, act: &action{kind: actPrint}})
		case "-print0":
			push(&expr{kind: exprAction, act: &action{kind: actPrint0}})
		case "-printf":
			if !hasOperand {
				return nil, missingArgument(arg)
			}
			format := operand()
			if err := checkPrintf(format); err != nil {
				return nil, "find: " + err.Error() + "\n"
			}
			push(&expr{kind: exprAction, act: &action{kind: actPrintf, format: format}})
		case "-delete":
			push(&expr{kind: exprAction, act: &action{kind: actDelete}})
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return nil, fmt.Sprintf("find: unknown predicate '%s'\n", arg)
			}
			return nil, fmt.Sprintf("find: paths must precede expression: `%s'\n", arg)
		}
	}

	if len(tokens) == 0 {
		return opts, ""
	}

	e, errMsg := buildExpressionTree(tokens)
	if errMsg != "" {
		return nil, errMsg
	}
	opts.expr = e
	return opts, ""
}

// compileRegex compiles a -regex pattern. Like GNU find, the pattern must
// match the whole path, not a substring of it. The pattern is checked on
// its own first: wrapping it in a group can turn a malformed pattern such
// as ".*)|(?:.*" into a valid one that matches every path.
func compileRegex(pattern string, ignoreCase bool) (*regexp.Regexp, error) {
	if _, err := regexp.Compile(pattern); err != nil {
		return nil, err
	}
	flags := ""
	if ignoreCase {
		flags = "(?i)"
	}
	return regexp.Compile(flags + "^(?:" + pattern + ")$")
}

func splitComparison(raw string) (comparison, string) {
	switch {
	case len(raw) > 0 && raw[0] == '+':
		return cmpMore, raw[1:]
	case len(raw) > 0 && raw[0] == '-':
		return cmpLess, raw[1:]
	}
	return cmpExact, raw
}

// parseSafeUint accepts only ASCII digits and rejects values above
// maxSafeInteger.
func parseSafeUint(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	for _, c := range []byte(s) {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n > maxSafeInteger {
		return 0, false
	}
	return n, true
}

func sizeMultiplier(unit byte) int64 {
	switch unit {
	case 'c':
		return 1
	case 'k':
		return 1024
	case 'M':
		return 1024 * 1024
	case 'G':
		return 1024 * 1024 * 1024
	}
	return 512 // 'b', the default
}

func parseSize(raw string) (*expr, bool) {
	cmp, rest := splitComparison(raw)
	unit := byte('b')
	if n := len(rest); n > 0 {
		switch rest[n-1] {
		case 'c', 'k', 'M', 'G', 'b':
			unit = rest[n-1]
			rest = rest[:n-1]
		}
	}
	value, ok := parseSafeUint(rest)
	if !ok || value > maxSafeInteger/sizeMultiplier(unit) {
		return nil, false
	}
	return &expr{kind: exprSize, cmp: cmp, value: value, unit: unit}, true
}

func parsePerm(raw string) (*expr, bool) {
	match := permExact
	digits := raw
	if len(raw) > 0 && raw[0] == '-' {
		match, digits = permAll, raw[1:]
	} else if len(raw) > 0 && raw[0] == '/' {
		match, digits = permAny, raw[1:]
	}
	if len(digits) < 1 || len(digits) > 4 {
		return nil, false
	}
	for _, c := range []byte(digits) {
		if c < '0' || c > '7' {
			return nil, false
		}
	}
	mode, err := strconv.ParseInt(digits, 8, 64)
	if err != nil {
		return nil, false
	}
	return &expr{kind: exprPerm, perm: match, value: mode}, true
}

func countBraces(parts []string) int {
	n := 0
	for _, p := range parts {
		if p == "{}" {
			n++
		}
	}
	return n
}

// buildExpressionTree is a recursive descent parser with find's
// precedence: parentheses, then NOT, then AND (explicit or implicit),
// then OR.
func buildExpressionTree(tokens []token) (*expr, string) {
	pos := 0
	errMsg := ""

	var parseOr, parseAnd, parseNot, parsePrimary func() *expr

	parseOr = func() *expr {
		left := parseAnd()
		if left == nil {
			return nil
		}
		for pos < len(tokens) && tokens[pos].kind == tokOr {
			pos++
			right := parseAnd()
			if right == nil {
				errMsg = "find: expected an expression after `-o'\n"
				return nil
			}
			left = &expr{kind: exprOr, left: left, right: right}
		}
		return left
	}

	parseAnd = func() *expr {
		left := parseNot()
		if left == nil {
			return nil
		}
		for pos < len(tokens) {
			switch tokens[pos].kind {
			case tokAnd:
				pos++
				right := parseNot()
				if right == nil {
					errMsg = "find: expected an expression after `-a'\n"
					return nil
				}
				left = &expr{kind: exprAnd, left: left, right: right}
			case tokExpr, tokNot, tokLParen:
				right := parseNot()
				if right == nil {
					return left
				}
				left = &expr{kind: exprAnd, left: left, right: right}
			default:
				return left
			}
		}
		return left
	}

	parseNot = func() *expr {
		if pos < len(tokens) && tokens[pos].kind == tokNot {
			pos++
			inner := parseNot()
			if inner == nil {
				errMsg = "find: expected an expression after `!'\n"
				return nil
			}
			return &expr{kind: exprNot, left: inner}
		}
		return parsePrimary()
	}

	parsePrimary = func() *expr {
		if pos >= len(tokens) {
			return nil
		}
		switch tok := tokens[pos]; tok.kind {
		case tokLParen:
			pos++
			inner := parseOr()
			if inner == nil || pos >= len(tokens) || tokens[pos].kind != tokRParen {
				errMsg = "find: missing closing `)'\n"
				return nil
			}
			pos++
			return inner
		case tokExpr:
			pos++
			return tok.expr
		}
		return nil
	}

	e := parseOr()
	if errMsg == "" && pos < len(tokens) {
		if tokens[pos].kind == tokRParen {
			errMsg = "find: unexpected `)'\n"
		} else {
			errMsg = "find: invalid expression\n"
		}
	}
	if errMsg == "" && e != nil && containsNegatedDelete(e, false) {
		errMsg = "find: refusing to evaluate `-delete' under negation\n"
	}
	if errMsg != "" {
		return nil, errMsg
	}
	return e, ""
}

func containsNegatedDelete(e *expr, negated bool) bool {
	switch e.kind {
	case exprAction:
		return negated && e.act.kind == actDelete
	case exprNot:
		return containsNegatedDelete(e.left, !negated)
	case exprAnd, exprOr:
		return containsNegatedDelete(e.left, negated) || containsNegatedDelete(e.right, negated)
	}
	return false
}

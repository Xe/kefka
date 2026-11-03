package grep

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/dlclark/regexp2/v2"
)

func (m *matcher) matchText(match *regexp2.Match) (string, error) {
	if m.resetGroup != "" {
		if g := match.GroupByName(m.resetGroup); g != nil && len(g.Captures) > 0 {
			start := g.RuneIndex - match.RuneIndex
			text := match.Runes()
			if start < 0 || start > len(text) {
				return "", fmt.Errorf("invalid match reset position")
			}
			return string(text[start:]), nil
		}
	}
	return match.String(), nil
}

const (
	maxSearchWork    = 1000000
	maxSearchResults = 100000
	maxSearchOutput  = 8 << 20
	maxSearchLine    = 8 << 20
	maxSearchContext = 8 << 20
)

var (
	errWorkLimit    = errors.New("matching work limit exceeded")
	errResultLimit  = errors.New("match result limit exceeded")
	errOutputLimit  = errors.New("output size limit exceeded")
	errLineLimit    = errors.New("input line size limit exceeded")
	errContextLimit = errors.New("context size limit exceeded")
	errInput        = errors.New("Input/output error")
)

// One budget covers the entire invocation, including all files and occurrences.
type searchBudget struct{ work, results, output int }

func (b *searchBudget) step() error {
	if b.work >= maxSearchWork {
		return errWorkLimit
	}
	b.work++
	return nil
}
func (b *searchBudget) result() error {
	if err := b.step(); err != nil {
		return err
	}
	if b.results >= maxSearchResults {
		return errResultLimit
	}
	b.results++
	return nil
}
func (b *searchBudget) write(out *strings.Builder, parts ...string) error {
	if err := b.step(); err != nil {
		return err
	}
	size := 0
	for _, part := range parts {
		if len(part) > maxSearchOutput-b.output-size {
			return errOutputLimit
		}
		size += len(part)
	}
	b.output += size
	for _, part := range parts {
		out.WriteString(part)
	}
	return nil
}

// ReadSlice allows long lines without an unbounded allocation. Unlike ScanLines,
// this preserves CR bytes and removes only the LF terminator used by just-bash.
func readLine(r *bufio.Reader) (string, bool, error) {
	var line []byte
	for {
		part, err := r.ReadSlice('\n')
		if len(part) > maxSearchLine-len(line) {
			return "", false, errLineLimit
		}
		line = append(line, part...)
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil && err != io.EOF {
			return "", false, errInput
		}
		if len(line) == 0 {
			return "", false, nil
		}
		if line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}
		return decodeUTF8(line), true, nil
	}
}

type contextLine struct {
	index int
	text  string
}

func search(ctx context.Context, input *bufio.Reader, m *matcher, o options, filename string, budget *searchBudget) (string, bool, error) {
	if o.quiet || o.list || o.listWithout {
		o.before, o.after = 0, 0
	}
	var out strings.Builder
	prefix := func(index int, selected bool) string {
		sep := "-"
		if selected {
			sep = ":"
		}
		p := ""
		if filename != "" {
			p = filename + sep
		}
		if o.numbers {
			p += strconv.Itoa(index+1) + sep
		}
		return p
	}
	emit := func(index int, line string, selected bool, match *regexp2.Match) error {
		p := prefix(index, selected)
		if selected && o.only {
			for match != nil {
				if err := ctx.Err(); err != nil {
					return err
				}
				if err := budget.result(); err != nil {
					return err
				}
				text, err := m.matchText(match)
				if err != nil {
					return err
				}
				if err := budget.write(&out, p, text, "\n"); err != nil {
					return err
				}
				match, err = m.re.FindNextMatch(match)
				if err != nil {
					return err
				}
			}
			return nil
		}
		return budget.write(&out, p, line, "\n")
	}
	var pending []contextLine
	pendingBytes := 0
	after := 0
	selectedCount, count := 0, 0
	lastPrinted := -1
	withContext := o.before > 0 || o.after > 0
	for index := 0; ; index++ {
		if err := ctx.Err(); err != nil {
			return "", false, err
		}
		exhausted := o.maxCount > 0 && selectedCount >= o.maxCount
		if exhausted && (o.count || after == 0) {
			break
		}
		if err := budget.step(); err != nil {
			return "", false, err
		}
		line, ok, err := readLine(input)
		if err != nil {
			return "", false, err
		}
		if !ok {
			break
		}
		var match *regexp2.Match
		selected := false
		if !exhausted {
			match, err = m.re.FindStringMatch(line)
			if err != nil {
				return "", false, err
			}
			selected = (match != nil) != o.invert
		}
		if selected {
			selectedCount++
			if o.quiet || o.list || o.listWithout {
				return "", true, nil
			}
			if o.count && o.only && !o.invert {
				for match != nil {
					if err := ctx.Err(); err != nil {
						return "", false, err
					}
					if err := budget.result(); err != nil {
						return "", false, err
					}
					count++
					match, err = m.re.FindNextMatch(match)
					if err != nil {
						return "", false, err
					}
				}
			} else {
				count++
				if !o.only || o.count {
					if err := budget.result(); err != nil {
						return "", false, err
					}
				}
			}
			if o.count {
				continue
			}
			first := index
			if len(pending) > 0 {
				first = pending[0].index
			}
			if withContext && lastPrinted >= 0 && first > lastPrinted+1 {
				if err := budget.write(&out, "--\n"); err != nil {
					return "", false, err
				}
			}
			for _, previous := range pending {
				if err := emit(previous.index, previous.text, false, nil); err != nil {
					return "", false, err
				}
			}
			clear(pending)
			pending = pending[:0]
			pendingBytes = 0
			if err := emit(index, line, true, match); err != nil {
				return "", false, err
			}
			lastPrinted = index
			after = o.after
		} else if !o.count && after > 0 {
			if err := emit(index, line, false, nil); err != nil {
				return "", false, err
			}
			lastPrinted = index
			after--
		} else if !o.count && o.before > 0 {
			if len(pending) == o.before {
				pendingBytes -= len(pending[0].text)
				pending[0] = contextLine{}
				pending = pending[1:]
			}
			if len(pending) >= maxSearchResults || len(line) > maxSearchContext-pendingBytes {
				return "", false, errContextLimit
			}
			pending = append(pending, contextLine{index, line})
			pendingBytes += len(line)
		}
	}
	if o.count {
		p := ""
		if filename != "" {
			p = filename + ":"
		}
		if err := budget.write(&out, p, strconv.Itoa(count), "\n"); err != nil {
			return "", false, err
		}
	}
	return out.String(), selectedCount > 0, nil
}

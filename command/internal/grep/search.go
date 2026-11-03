package grep

import (
	"context"
	"fmt"
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

func search(ctx context.Context, content string, m *matcher, o options, filename string) (string, bool, error) {
	lines := splitLines(content)
	var out strings.Builder
	prefix := func(index int, selected bool) {
		sep := "-"
		if selected {
			sep = ":"
		}
		if filename != "" {
			out.WriteString(filename)
			out.WriteString(sep)
		}
		if o.numbers {
			fmt.Fprintf(&out, "%d%s", index+1, sep)
		}
	}
	emit := func(index int, selected bool) error {
		if selected && o.only {
			match, err := m.re.FindStringMatch(lines[index])
			for match != nil && err == nil {
				if err := ctx.Err(); err != nil {
					return err
				}
				text, textErr := m.matchText(match)
				if textErr != nil {
					return textErr
				}
				prefix(index, true)
				out.WriteString(text)
				out.WriteByte('\n')
				match, err = m.re.FindNextMatch(match)
			}
			return err
		}
		prefix(index, selected)
		out.WriteString(lines[index])
		out.WriteByte('\n')
		return nil
	}
	var selected []int
	count := 0
	for i, line := range lines {
		if err := ctx.Err(); err != nil {
			return "", false, err
		}
		if o.maxCount > 0 && len(selected) >= o.maxCount {
			break
		}
		match, err := m.re.FindStringMatch(line)
		if err != nil {
			return "", false, err
		}
		if (match != nil) == o.invert {
			continue
		}
		selected = append(selected, i)
		if o.quiet || o.list || o.listWithout {
			return "", true, nil
		}
		if o.count && o.only && !o.invert {
			for match != nil {
				if err := ctx.Err(); err != nil {
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
		}
		if !o.count && o.before == 0 && o.after == 0 {
			if err := emit(i, true); err != nil {
				return "", false, err
			}
		}
	}
	if o.count {
		if filename != "" {
			out.WriteString(filename)
			out.WriteByte(':')
		}
		fmt.Fprintf(&out, "%d\n", count)
	} else if o.before > 0 || o.after > 0 {
		lastPrinted := -1
		selectedIndex := 0
		for _, lineNum := range selected {
			start := max(0, lineNum-min(o.before, lineNum))
			// Subtract before adding to avoid overflowing on very large -A values.
			end := lineNum + min(o.after, len(lines)-1-lineNum)
			if lastPrinted >= 0 && start > lastPrinted+1 {
				out.WriteString("--\n")
			}
			for i := max(start, lastPrinted+1); i <= end; i++ {
				if err := ctx.Err(); err != nil {
					return "", false, err
				}
				for selectedIndex < len(selected) && selected[selectedIndex] < i {
					selectedIndex++
				}
				isMatch := selectedIndex < len(selected) && selected[selectedIndex] == i
				if err := emit(i, isMatch); err != nil {
					return "", false, err
				}
				lastPrinted = i
			}
		}
	}
	return out.String(), len(selected) > 0, nil
}

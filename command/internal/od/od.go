package od

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"path"
	"strconv"
	"strings"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

type formatKind int

const (
	fmtNamedChar formatKind = iota
	fmtChar
	fmtSignedDec
	fmtUnsignedDec
	fmtOctal
	fmtHex
	fmtFloat
)

type formatSpec struct {
	kind  formatKind
	size  int
	width int
	raw   string
}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("od: nil ExecContext")
	}

	stdout := ec.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := ec.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	set := getopt.New()
	set.SetProgram("od")
	set.SetParameters("[FILE]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: od [OPTION]... [FILE]...\n")
		fmt.Fprint(stderr, "Write an unambiguous representation, octal bytes by default,\n")
		fmt.Fprint(stderr, "of FILE to standard output.\n\n")
		fmt.Fprint(stderr, "  -A, --address-radix=RADIX   output format for file offsets; RADIX is one\n")
		fmt.Fprint(stderr, "                                of [doxn], for Decimal, Octal, Hex or None\n")
		fmt.Fprint(stderr, "  -j, --skip-bytes=BYTES      skip BYTES input bytes first\n")
		fmt.Fprint(stderr, "  -N, --read-bytes=BYTES      limit dump to BYTES input bytes\n")
		fmt.Fprint(stderr, "  -t, --format=TYPE           select output format or formats\n")
		fmt.Fprint(stderr, "  -v, --output-duplicates     do not use * to mark line suppression\n")
		fmt.Fprint(stderr, "  -w, --width[=BYTES]         output BYTES bytes per line (default 16)\n")
		fmt.Fprint(stderr, "  -b                          same as -t o1, select octal bytes\n")
		fmt.Fprint(stderr, "  -c                          same as -t c, select printable chars\n")
		fmt.Fprint(stderr, "  -d                          same as -t u2, select unsigned decimal 2-byte units\n")
		fmt.Fprint(stderr, "  -o                          same as -t o2, select octal 2-byte units\n")
		fmt.Fprint(stderr, "  -s                          same as -t d2, select signed decimal 2-byte units\n")
		fmt.Fprint(stderr, "  -x                          same as -t x2, select hexadecimal 2-byte units\n")
		fmt.Fprint(stderr, "      --help                  display this help and exit\n")
	}
	set.SetUsage(usage)

	addrOpt := set.StringLong("address-radix", 'A', "o", "output format for file offsets")
	skipOpt := set.StringLong("skip-bytes", 'j', "", "skip BYTES input bytes first")
	readOpt := set.StringLong("read-bytes", 'N', "", "limit dump to BYTES input bytes")
	verbose := set.BoolLong("output-duplicates", 'v', "do not use * to mark line suppression")
	widthOpt := set.StringLong("width", 'w', "16", "output BYTES bytes per line")
	_ = set.Bool('b', "same as -t o1")
	_ = set.Bool('c', "same as -t c")
	_ = set.Bool('d', "same as -t u2")
	_ = set.Bool('o', "same as -t o2")
	_ = set.Bool('s', "same as -t d2")
	_ = set.Bool('x', "same as -t x2")
	tList := set.ListLong("format", 't', "select output format or formats")
	helpFlag := set.BoolLong("help", 0, "display this help and exit")

	var formats []formatSpec
	tIdx := 0
	parseErr := ""
	callback := func(opt getopt.Option) bool {
		switch opt.ShortName() {
		case "b":
			formats = append(formats, makeSpec(fmtOctal, 1, "o1"))
		case "c":
			formats = append(formats, makeSpec(fmtChar, 1, "c"))
		case "d":
			formats = append(formats, makeSpec(fmtUnsignedDec, 2, "u2"))
		case "o":
			formats = append(formats, makeSpec(fmtOctal, 2, "o2"))
		case "s":
			formats = append(formats, makeSpec(fmtSignedDec, 2, "d2"))
		case "x":
			formats = append(formats, makeSpec(fmtHex, 2, "x2"))
		case "t":
			// ListLong splits on commas, so a single -t can produce
			// multiple list entries. Consume everything appended since
			// the last -t callback.
			for tIdx < len(*tList) {
				v := (*tList)[tIdx]
				tIdx++
				specs, err := parseTypeString(v)
				if err != nil {
					parseErr = v
					return false
				}
				formats = append(formats, specs...)
			}
		}
		return true
	}

	if err := set.Getopt(append([]string{"od"}, args...), callback); err != nil {
		if parseErr != "" {
			fmt.Fprintf(stderr, "od: invalid type string '%s'\n", parseErr)
		} else {
			fmt.Fprintf(stderr, "od: %s\n", err)
			usage()
		}
		return interp.ExitStatus(1)
	}

	if parseErr != "" {
		fmt.Fprintf(stderr, "od: invalid type string '%s'\n", parseErr)
		return interp.ExitStatus(1)
	}

	if *helpFlag {
		usage()
		return nil
	}

	addrRadix := *addrOpt
	switch addrRadix {
	case "d", "o", "x", "n":
	default:
		fmt.Fprintf(stderr, "od: invalid output address radix '%s'; it must be one character from [doxn]\n", addrRadix)
		return interp.ExitStatus(1)
	}

	skip := int64(0)
	if *skipOpt != "" {
		n, err := parseByteCount(*skipOpt)
		if err != nil {
			fmt.Fprintf(stderr, "od: invalid argument '%s' for '--skip-bytes'\n", *skipOpt)
			return interp.ExitStatus(1)
		}
		skip = n
	}

	limit := int64(-1)
	if *readOpt != "" {
		n, err := parseByteCount(*readOpt)
		if err != nil {
			fmt.Fprintf(stderr, "od: invalid argument '%s' for '--read-bytes'\n", *readOpt)
			return interp.ExitStatus(1)
		}
		limit = n
	}

	if len(formats) == 0 {
		formats = []formatSpec{makeSpec(fmtOctal, 2, "o2")}
	}

	bytesPerLine := 16
	if *widthOpt != "" {
		n, err := strconv.Atoi(*widthOpt)
		if err != nil || n <= 0 {
			fmt.Fprintf(stderr, "od: invalid width specification '%s'\n", *widthOpt)
			return interp.ExitStatus(1)
		}
		bytesPerLine = n
	}
	// Round bytesPerLine up to a multiple of the LCM of format sizes so
	// that no field straddles a line boundary.
	lcm := 1
	for _, f := range formats {
		lcm = lcmInt(lcm, f.size)
	}
	if bytesPerLine%lcm != 0 {
		bytesPerLine = ((bytesPerLine / lcm) + 1) * lcm
	}

	files := set.Args()
	data, err := readInput(ec, files, stderr)
	if err != nil {
		return err
	}

	if skip > 0 {
		if int64(len(data)) < skip {
			fmt.Fprint(stderr, "od: cannot skip past end of combined input\n")
			return interp.ExitStatus(1)
		}
		data = data[skip:]
	}
	if limit >= 0 && int64(len(data)) > limit {
		data = data[:limit]
	}

	io.WriteString(stdout, buildOutput(data, formats, addrRadix, skip, *verbose, bytesPerLine))
	return nil
}

func lcmInt(a, b int) int {
	if a == 0 || b == 0 {
		return 0
	}
	g := gcdInt(a, b)
	return a / g * b
}

func gcdInt(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	return a
}

func makeSpec(kind formatKind, size int, raw string) formatSpec {
	return formatSpec{kind: kind, size: size, width: itemWidth(kind, size), raw: raw}
}

func itemWidth(kind formatKind, size int) int {
	switch kind {
	case fmtNamedChar, fmtChar:
		return 4
	case fmtSignedDec:
		switch size {
		case 1:
			return 5
		case 2:
			return 7
		case 4:
			return 12
		case 8:
			return 21
		}
	case fmtUnsignedDec:
		switch size {
		case 1:
			return 4
		case 2:
			return 6
		case 4:
			return 11
		case 8:
			return 21
		}
	case fmtOctal:
		switch size {
		case 1:
			return 4
		case 2:
			return 7
		case 4:
			return 12
		case 8:
			return 23
		}
	case fmtHex:
		switch size {
		case 1:
			return 3
		case 2:
			return 5
		case 4:
			return 9
		case 8:
			return 17
		}
	case fmtFloat:
		switch size {
		case 4:
			return 16
		case 8:
			return 25
		}
	}
	return 4
}

func parseTypeString(s string) ([]formatSpec, error) {
	var out []formatSpec
	i := 0
	for i < len(s) {
		ch := s[i]
		i++
		switch ch {
		case 'a':
			out = append(out, makeSpec(fmtNamedChar, 1, "a"))
		case 'c':
			out = append(out, makeSpec(fmtChar, 1, "c"))
		case 'd', 'o', 'u', 'x':
			size := 4
			raw := string(ch)
			if i < len(s) {
				if n, w, ok := parseIntSize(s[i:]); ok {
					size = n
					raw += s[i : i+w]
					i += w
				} else if l, ok := parseLetterSize(s[i], "CSIL"); ok {
					size = l
					raw += string(s[i])
					i++
				}
			}
			if !validIntSize(size) {
				return nil, fmt.Errorf("invalid size")
			}
			var kind formatKind
			switch ch {
			case 'd':
				kind = fmtSignedDec
			case 'o':
				kind = fmtOctal
			case 'u':
				kind = fmtUnsignedDec
			case 'x':
				kind = fmtHex
			}
			out = append(out, makeSpec(kind, size, raw))
		case 'f':
			size := 8
			raw := "f"
			if i < len(s) {
				if n, w, ok := parseIntSize(s[i:]); ok {
					size = n
					raw += s[i : i+w]
					i += w
				} else {
					switch s[i] {
					case 'F':
						size = 4
						raw += "F"
						i++
					case 'D':
						size = 8
						raw += "D"
						i++
					case 'L':
						return nil, fmt.Errorf("long double unsupported")
					}
				}
			}
			if size != 4 && size != 8 {
				return nil, fmt.Errorf("invalid float size")
			}
			out = append(out, makeSpec(fmtFloat, size, raw))
		case ',', ' ':
			continue
		default:
			return nil, fmt.Errorf("unknown type char %q", ch)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty type string")
	}
	return out, nil
}

func parseIntSize(s string) (int, int, bool) {
	w := 0
	for w < len(s) && s[w] >= '0' && s[w] <= '9' {
		w++
	}
	if w == 0 {
		return 0, 0, false
	}
	n, err := strconv.Atoi(s[:w])
	if err != nil {
		return 0, 0, false
	}
	return n, w, true
}

func parseLetterSize(c byte, allowed string) (int, bool) {
	if !strings.ContainsRune(allowed, rune(c)) {
		return 0, false
	}
	switch c {
	case 'C':
		return 1, true
	case 'S':
		return 2, true
	case 'I':
		return 4, true
	case 'L':
		return 8, true
	}
	return 0, false
}

func validIntSize(n int) bool {
	return n == 1 || n == 2 || n == 4 || n == 8
}

func parseByteCount(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	mult := int64(1)
	last := s[len(s)-1]
	hexPrefix := strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X")
	switch last {
	case 'b':
		if !hexPrefix {
			mult = 512
			s = s[:len(s)-1]
		}
	case 'k':
		mult = 1024
		s = s[:len(s)-1]
	case 'm':
		mult = 1048576
		s = s[:len(s)-1]
	}
	base := 10
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		base = 16
		s = s[2:]
	} else if strings.HasPrefix(s, "0") && len(s) > 1 {
		base = 8
		s = s[1:]
	}
	n, err := strconv.ParseInt(s, base, 64)
	if err != nil {
		return 0, err
	}
	return n * mult, nil
}

func readInput(ec *command.ExecContext, files []string, stderr io.Writer) ([]byte, error) {
	if len(files) == 0 || files[0] == "-" {
		if ec.Stdin == nil {
			return nil, nil
		}
		return io.ReadAll(ec.Stdin)
	}
	if ec.FS == nil {
		fmt.Fprintf(stderr, "od: %s: No such file or directory\n", files[0])
		return nil, interp.ExitStatus(1)
	}
	full := resolvePath(ec, files[0])
	f, err := ec.FS.Open(full)
	if err != nil {
		fmt.Fprintf(stderr, "od: %s: No such file or directory\n", files[0])
		return nil, interp.ExitStatus(1)
	}
	defer f.Close()
	return io.ReadAll(f)
}

func buildOutput(data []byte, formats []formatSpec, addrRadix string, baseAddr int64, verbose bool, bytesPerLine int) string {
	if bytesPerLine <= 0 {
		bytesPerLine = 16
	}
	addressNone := addrRadix == "n"

	totalLen := int64(len(data))
	if totalLen == 0 {
		if !addressNone {
			return formatAddress(baseAddr, addrRadix) + "\n"
		}
		return ""
	}

	var b strings.Builder
	var prevChunk []byte
	starShown := false

	bpl := int64(bytesPerLine)
	for offset := int64(0); offset < totalLen; offset += bpl {
		end := offset + bpl
		if end > totalLen {
			end = totalLen
		}
		chunk := data[offset:end]
		isLast := end == totalLen

		if !verbose && !isLast && int64(len(chunk)) == bpl && prevChunk != nil && bytesEqual(prevChunk, chunk) {
			if !starShown {
				b.WriteString("*\n")
				starShown = true
			}
			prevChunk = chunk
			continue
		}
		starShown = false
		prevChunk = chunk

		writeBlock(&b, chunk, formats, addrRadix, baseAddr+offset)
	}

	if !addressNone {
		b.WriteString(formatAddress(baseAddr+totalLen, addrRadix))
		b.WriteByte('\n')
	}

	return b.String()
}

func writeBlock(b *strings.Builder, chunk []byte, formats []formatSpec, addrRadix string, addr int64) {
	addressNone := addrRadix == "n"

	maxByteWidth := 0
	for _, f := range formats {
		if f.size == 1 && f.width > maxByteWidth {
			maxByteWidth = f.width
		}
	}

	totalWidth := 0
	for _, f := range formats {
		w := lineWidthForFormat(f, len(chunk), maxByteWidth)
		if w > totalWidth {
			totalWidth = w
		}
	}

	for fIdx, f := range formats {
		switch {
		case fIdx == 0 && !addressNone:
			b.WriteString(formatAddress(addr, addrRadix))
		case fIdx > 0 && !addressNone:
			b.WriteString(strings.Repeat(" ", addressWidth(addrRadix)))
		}

		text := renderFormat(f, chunk, maxByteWidth)
		if pad := totalWidth - visibleWidth(f, len(chunk), maxByteWidth); pad > 0 {
			text = strings.Repeat(" ", pad) + text
		}
		b.WriteString(text)
		b.WriteByte('\n')
	}
}

func lineWidthForFormat(f formatSpec, chunkLen, maxByteWidth int) int {
	return visibleWidth(f, chunkLen, maxByteWidth)
}

func visibleWidth(f formatSpec, chunkLen, maxByteWidth int) int {
	if f.size == 1 {
		w := f.width
		if maxByteWidth > w {
			w = maxByteWidth
		}
		return w * chunkLen
	}
	items := chunkLen / f.size
	rem := chunkLen % f.size
	total := items * f.width
	if rem > 0 {
		total += f.width
	}
	return total
}

func renderFormat(f formatSpec, chunk []byte, maxByteWidth int) string {
	var b strings.Builder
	switch f.kind {
	case fmtNamedChar:
		w := f.width
		if maxByteWidth > w {
			w = maxByteWidth
		}
		for _, c := range chunk {
			s := namedChar(c)
			fmt.Fprintf(&b, "%*s", w, s)
		}
	case fmtChar:
		w := f.width
		if maxByteWidth > w {
			w = maxByteWidth
		}
		for _, c := range chunk {
			s := charRepr(c)
			fmt.Fprintf(&b, "%*s", w, s)
		}
	case fmtHex:
		renderInts(&b, chunk, f, false, 16, maxByteWidth)
	case fmtOctal:
		renderInts(&b, chunk, f, false, 8, maxByteWidth)
	case fmtUnsignedDec:
		renderInts(&b, chunk, f, false, 10, maxByteWidth)
	case fmtSignedDec:
		renderInts(&b, chunk, f, true, 10, maxByteWidth)
	case fmtFloat:
		renderFloats(&b, chunk, f)
	}
	return b.String()
}

func renderInts(b *strings.Builder, chunk []byte, f formatSpec, signed bool, base int, maxByteWidth int) {
	w := f.width
	if f.size == 1 && maxByteWidth > w {
		w = maxByteWidth
	}
	pad := w - 1
	zeroPad := !signed && (base == 8 || base == 16)
	digits := digitsForType(f.kind, f.size)
	for i := 0; i < len(chunk); i += f.size {
		end := i + f.size
		if end > len(chunk) {
			end = len(chunk)
		}
		buf := make([]byte, f.size)
		copy(buf, chunk[i:end])
		var num string
		if signed {
			num = signedString(buf, base)
		} else {
			num = unsignedString(buf, base)
		}
		if zeroPad && len(num) < digits {
			num = strings.Repeat("0", digits-len(num)) + num
		}
		fmt.Fprintf(b, " %*s", pad, num)
	}
}

func digitsForType(kind formatKind, size int) int {
	switch kind {
	case fmtOctal:
		switch size {
		case 1:
			return 3
		case 2:
			return 6
		case 4:
			return 11
		case 8:
			return 22
		}
	case fmtHex:
		return size * 2
	}
	return 0
}

func unsignedString(buf []byte, base int) string {
	var v uint64
	switch len(buf) {
	case 1:
		v = uint64(buf[0])
	case 2:
		v = uint64(binary.LittleEndian.Uint16(buf))
	case 4:
		v = uint64(binary.LittleEndian.Uint32(buf))
	case 8:
		v = binary.LittleEndian.Uint64(buf)
	}
	return strconv.FormatUint(v, base)
}

func signedString(buf []byte, base int) string {
	var v int64
	switch len(buf) {
	case 1:
		v = int64(int8(buf[0]))
	case 2:
		v = int64(int16(binary.LittleEndian.Uint16(buf)))
	case 4:
		v = int64(int32(binary.LittleEndian.Uint32(buf)))
	case 8:
		v = int64(binary.LittleEndian.Uint64(buf))
	}
	return strconv.FormatInt(v, base)
}

func renderFloats(b *strings.Builder, chunk []byte, f formatSpec) {
	pad := f.width - 1
	for i := 0; i < len(chunk); i += f.size {
		end := i + f.size
		if end > len(chunk) {
			end = len(chunk)
		}
		buf := make([]byte, f.size)
		copy(buf, chunk[i:end])
		var s string
		switch f.size {
		case 4:
			v := math.Float32frombits(binary.LittleEndian.Uint32(buf))
			s = strconv.FormatFloat(float64(v), 'g', 8, 32)
		case 8:
			v := math.Float64frombits(binary.LittleEndian.Uint64(buf))
			s = strconv.FormatFloat(v, 'g', 16, 64)
		}
		fmt.Fprintf(b, " %*s", pad, s)
	}
}

func charRepr(c byte) string {
	switch c {
	case 0:
		return `\0`
	case 7:
		return `\a`
	case 8:
		return `\b`
	case 9:
		return `\t`
	case 10:
		return `\n`
	case 11:
		return `\v`
	case 12:
		return `\f`
	case 13:
		return `\r`
	}
	if c >= 32 && c < 127 {
		return string(rune(c))
	}
	return fmt.Sprintf("%03o", c)
}

func namedChar(c byte) string {
	c &= 0x7f
	if c == 0x7f {
		return "del"
	}
	names := [...]string{
		"nul", "soh", "stx", "etx", "eot", "enq", "ack", "bel",
		"bs", "ht", "nl", "vt", "ff", "cr", "so", "si",
		"dle", "dc1", "dc2", "dc3", "dc4", "nak", "syn", "etb",
		"can", "em", "sub", "esc", "fs", "gs", "rs", "us",
		"sp",
	}
	if int(c) < len(names) {
		return names[c]
	}
	return string(rune(c))
}

func formatAddress(addr int64, radix string) string {
	switch radix {
	case "d":
		return fmt.Sprintf("%07d", addr)
	case "x":
		return fmt.Sprintf("%06x", addr)
	case "o":
		return fmt.Sprintf("%07o", addr)
	}
	return ""
}

func addressWidth(radix string) int {
	switch radix {
	case "d", "o":
		return 7
	case "x":
		return 6
	}
	return 0
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func resolvePath(ec *command.ExecContext, p string) string {
	dir := ec.Dir
	if dir == "" {
		dir = "."
	}
	if path.IsAbs(p) {
		p = strings.TrimPrefix(p, "/")
		if p == "" {
			return "."
		}
		return path.Clean(p)
	}
	joined := path.Join(dir, p)
	if joined == "" {
		return "."
	}
	return joined
}

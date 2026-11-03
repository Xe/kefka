// Package find implements find(1), ported from just-bash.
//
// The whole expression is parsed before the filesystem is touched, and
// every action is collected during the walk and applied afterwards, in
// traversal order. Batched "-exec ... {} +" commands run last, once per
// -exec. -delete implies -depth. Like GNU find's default -P mode, symbolic
// links are reported but never followed.
package find

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/Xe/kefka/command"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// Impl is the find command.
type Impl struct{}

var helpOptions = []string{
	"-name PATTERN    file name matches shell pattern PATTERN",
	"-iname PATTERN   like -name but case insensitive",
	"-path PATTERN    file path matches shell pattern PATTERN",
	"-ipath PATTERN   like -path but case insensitive",
	"-regex PATTERN   file path matches regular expression PATTERN",
	"-iregex PATTERN  like -regex but case insensitive",
	"-type TYPE       file is of type: f (regular file), d (directory)",
	"-empty           file is empty or directory is empty",
	"-mtime N         file's data was modified N*24 hours ago",
	"-newer FILE      file was modified more recently than FILE",
	"-size N[ckMGb]   file uses N units of space (c=bytes, k=KB, M=MB, G=GB, b=512B blocks)",
	"-perm MODE       file's permission bits are exactly MODE (octal)",
	"-perm -MODE      all permission bits MODE are set",
	"-perm /MODE      any permission bits MODE are set",
	"-maxdepth LEVELS descend at most LEVELS directories",
	"-mindepth LEVELS do not apply tests at levels less than LEVELS",
	"-depth           process directory contents before directory itself",
	"-prune           do not descend into this directory",
	"-not, !          negate the following expression",
	"-a, -and         logical AND (default)",
	"-o, -or          logical OR",
	"-exec CMD {} ;   execute CMD on each file ({} is replaced by filename)",
	"-exec CMD {} +   execute CMD with multiple files at once",
	"-print           print the full file name (default action)",
	"-print0          print the full file name followed by a null character",
	"-printf FORMAT   print FORMAT with directives: %f %h %p %P %s %d %m %M %t",
	"-delete          delete found files/directories",
	"    --help       display this help and exit",
}

// printfStatDirective matches -printf directives that need stat data.
var printfStatDirective = regexp.MustCompile(`%[-+]?[0-9]*\.?[0-9]*[smMtT]`)

var errDirNotEmpty = errors.New("directory not empty")

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("find: nil ExecContext")
	}

	stdout := ec.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := ec.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	// Starting points come before the expression. Splitting them off first
	// keeps predicate operands and -exec arguments from looking like paths.
	exprStart := len(args)
	for i, arg := range args {
		if strings.HasPrefix(arg, "-") || arg == "(" || arg == "\\(" || arg == ")" || arg == "\\)" || arg == "!" {
			exprStart = i
			break
		}
	}
	starts := args[:exprStart]
	if len(starts) == 0 {
		starts = []string{"."}
	}

	opts, errMsg := parseExpression(args[exprStart:])
	if errMsg != "" {
		io.WriteString(stderr, errMsg)
		return interp.ExitStatus(1)
	}
	if opts.help {
		usage(stderr)
		return nil
	}

	if ec.FS == nil {
		return errors.New("find: ExecContext has no filesystem")
	}

	acts := opts.expr.actions()
	printfNeedsStat := false
	for _, a := range acts {
		if a.kind == actDelete {
			opts.depthFirst = true
		}
		if a.kind == actPrintf && printfStatDirective.MatchString(strings.ReplaceAll(a.format, "%%", "")) {
			printfNeedsStat = true
		}
	}

	w := &walker{
		ctx:       ctx,
		ec:        ec,
		opts:      opts,
		hasAction: len(acts) > 0,
		needsStat: opts.expr.needsStat() || printfNeedsStat,
		now:       time.Now(),
		newer:     map[string]time.Time{},
	}

	// A -newer reference that does not exist never matches.
	for _, ref := range opts.expr.newerRefs() {
		if info, err := ec.FS.Stat(resolvePath(ec, ref)); err == nil {
			w.newer[ref] = info.ModTime()
		}
	}

	for _, start := range starts {
		if err := w.walkStart(start); err != nil {
			var fatal *fatalError
			if errors.As(err, &fatal) {
				fmt.Fprintf(stderr, "find: %s: %s\n", fatal.path, errorReason(fatal.err))
				return interp.ExitStatus(1)
			}
			return err
		}
	}

	return w.apply(stdout, stderr)
}

func usage(w io.Writer) {
	io.WriteString(w, "find - search for files in a directory hierarchy\n\n")
	io.WriteString(w, "Usage: find [path...] [expression]\n")
	io.WriteString(w, "\nOptions:\n")
	for _, opt := range helpOptions {
		io.WriteString(w, "  "+opt+"\n")
	}
}

// fatalError is a directory read failure that is not the directory's own
// problem (a lost connection, say). It ends the search with no output.
type fatalError struct {
	path string
	err  error
}

func (e *fatalError) Error() string { return e.path + ": " + e.err.Error() }
func (e *fatalError) Unwrap() error { return e.err }

// effect is one thing to do once the walk is over: an action reached on
// an entry, or (when act is nil) a diagnostic for stderr.
type effect struct {
	act     *action
	message string
	fsPath  string
	data    printfData
}

// node is one entry being visited. info is nil when the entry was not
// stat'ed because nothing in the expression needs it.
type node struct {
	display string // path as find prints it
	fsPath  string // path inside ec.FS
	name    string
	depth   int
	isFile  bool
	isDir   bool
	info    fs.FileInfo

	read       bool
	entries    []fs.DirEntry
	unreadable string
}

type walker struct {
	ctx       context.Context
	ec        *command.ExecContext
	opts      *options
	hasAction bool
	needsStat bool
	now       time.Time
	newer     map[string]time.Time

	start   string
	effects []effect
	fatal   error
}

func (w *walker) walkStart(start string) error {
	for len(start) > 1 && strings.HasSuffix(start, "/") {
		start = start[:len(start)-1]
	}
	w.start = start
	fsPath := resolvePath(w.ec, start)

	// Lstat, not Stat: a starting point that is a dangling symlink still
	// exists and is reported like any other symlink.
	info, err := w.ec.FS.Lstat(fsPath)
	if err != nil {
		w.effects = append(w.effects, effect{message: fmt.Sprintf("find: %s: No such file or directory\n", start)})
		return nil
	}

	name := start
	if idx := strings.LastIndexByte(start, '/'); idx >= 0 && idx < len(start)-1 {
		name = start[idx+1:]
	}

	return w.visit(&node{
		display: start,
		fsPath:  fsPath,
		name:    name,
		isFile:  info.Mode().IsRegular(),
		isDir:   info.IsDir(),
		info:    info,
	})
}

// readDir lists n once. A failure that belongs to the directory itself
// (permissions, a vanished entry) is remembered and reported later; any
// other failure is fatal.
func (w *walker) readDir(n *node) {
	if n.read {
		return
	}
	n.read = true
	entries, err := w.ec.FS.ReadDir(n.fsPath)
	if err != nil {
		reason, ok := unreadableReason(err)
		if !ok {
			if w.fatal == nil {
				w.fatal = &fatalError{path: n.display, err: err}
			}
			return
		}
		n.unreadable = reason
		return
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	n.entries = entries
}

func (w *walker) visit(n *node) error {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	atMax := w.opts.maxDepth >= 0 && n.depth >= w.opts.maxDepth

	if w.opts.depthFirst {
		// -prune has no effect under -depth.
		if n.isDir && !atMax {
			w.readDir(n)
			if err := w.visitChildren(n); err != nil {
				return err
			}
		}
		evaluated, _ := w.evaluate(n)
		if w.fatal != nil {
			return w.fatal
		}
		w.diagnose(n)
		w.effects = append(w.effects, evaluated...)
		return nil
	}

	evaluated, pruned := w.evaluate(n)
	w.effects = append(w.effects, evaluated...)
	descend := n.isDir && !pruned && !atMax
	if descend {
		w.readDir(n)
	}
	if w.fatal != nil {
		return w.fatal
	}
	w.diagnose(n)
	if descend {
		return w.visitChildren(n)
	}
	return nil
}

func (w *walker) diagnose(n *node) {
	if n.unreadable != "" {
		w.effects = append(w.effects, effect{message: fmt.Sprintf("find: %s: %s\n", n.display, n.unreadable)})
	}
}

func (w *walker) visitChildren(n *node) error {
	for _, entry := range n.entries {
		child := &node{
			display: joinDisplay(n.display, entry.Name()),
			fsPath:  path.Join(n.fsPath, entry.Name()),
			name:    entry.Name(),
			depth:   n.depth + 1,
		}
		if w.needsStat {
			info, err := w.ec.FS.Lstat(child.fsPath)
			if err != nil {
				// Gone between listing and stat; skip it.
				continue
			}
			child.info = info
			child.isFile = info.Mode().IsRegular()
			child.isDir = info.IsDir()
		} else {
			child.isFile = entry.Type().IsRegular()
			child.isDir = entry.Type().IsDir()
		}
		if err := w.visit(child); err != nil {
			return err
		}
	}
	return nil
}

// evaluate runs the expression on n and returns the effects reached, plus
// whether -prune fired. Entries above -mindepth are not tested at all.
func (w *walker) evaluate(n *node) ([]effect, bool) {
	if n.depth < w.opts.minDepth {
		return nil, false
	}

	data := printfData{
		path:          n.display,
		name:          n.name,
		mtime:         w.now,
		mode:          0o644,
		isDir:         n.isDir,
		depth:         n.depth,
		startingPoint: w.start,
	}
	if n.info != nil {
		data.size = n.info.Size()
		data.mtime = n.info.ModTime()
		data.mode = n.info.Mode()
	}

	matches := true
	pruned := false
	var reached []*action
	if w.opts.expr != nil {
		res := w.opts.expr.eval(&evalCtx{
			name:   n.name,
			path:   n.display,
			isFile: n.isFile,
			isDir:  n.isDir,
			isEmpty: func() bool {
				if n.isFile {
					return data.size == 0
				}
				if !n.isDir {
					return false
				}
				w.readDir(n)
				return n.unreadable == "" && w.fatal == nil && len(n.entries) == 0
			},
			mtime: data.mtime,
			size:  data.size,
			mode:  data.mode,
			now:   w.now,
			newer: w.newer,
		})
		matches, pruned, reached = res.matches, res.pruned, res.actions
	}
	if !w.hasAction && matches {
		reached = []*action{{kind: actPrint}}
	}

	out := make([]effect, 0, len(reached))
	for _, a := range reached {
		out = append(out, effect{act: a, fsPath: n.fsPath, data: data})
	}
	return out, pruned
}

// apply performs the collected effects in order, then runs each batched
// -exec once with every path it collected.
func (w *walker) apply(stdout, stderr io.Writer) error {
	for _, e := range w.effects {
		if e.act != nil && e.act.kind == actExec && w.ec.Runner == nil {
			io.WriteString(stderr, "find: -exec not supported in this context\n")
			return interp.ExitStatus(1)
		}
	}

	out := bufio.NewWriter(stdout)
	defer out.Flush()
	warn := func(msg string) {
		out.Flush()
		io.WriteString(stderr, msg)
	}

	var exitCode uint8
	var batchOrder []*action
	batches := map[*action][]string{}

	for _, e := range w.effects {
		if e.act == nil {
			warn(e.message)
			exitCode = 1
			continue
		}
		switch e.act.kind {
		case actPrint:
			out.WriteString(e.data.path)
			out.WriteByte('\n')
		case actPrint0:
			out.WriteString(e.data.path)
			out.WriteByte(0)
		case actPrintf:
			out.WriteString(formatPrintf(e.act.format, e.data))
		case actDelete:
			if err := deleteEntry(w.ec, e.fsPath); err != nil {
				warn(fmt.Sprintf("find: cannot delete '%s': %s\n", e.data.path, errorReason(err)))
				exitCode = 1
			}
		case actExec:
			if e.act.batch {
				if _, seen := batches[e.act]; !seen {
					batchOrder = append(batchOrder, e.act)
				}
				batches[e.act] = append(batches[e.act], e.data.path)
				continue
			}
			argv := make([]string, len(e.act.command))
			for i, part := range e.act.command {
				argv[i] = strings.ReplaceAll(part, "{}", e.data.path)
			}
			out.Flush()
			code, err := runExec(w.ctx, w.ec, argv)
			if err != nil {
				return err
			}
			if code != 0 {
				exitCode = code
			}
		}
	}

	for _, a := range batchOrder {
		var argv []string
		for _, part := range a.command {
			if part == "{}" {
				argv = append(argv, batches[a]...)
			} else {
				argv = append(argv, part)
			}
		}
		out.Flush()
		code, err := runExec(w.ctx, w.ec, argv)
		if err != nil {
			return err
		}
		if code != 0 {
			exitCode = code
		}
	}

	if err := out.Flush(); err != nil {
		return err
	}
	if exitCode != 0 {
		return interp.ExitStatus(exitCode)
	}
	return nil
}

// runExec runs argv through a subshell of ec.Runner so it goes through
// the same exec-handler chain as a command typed at the prompt. It
// returns the command's exit status; err is set only for failures that
// are not an exit status, such as cancellation.
func runExec(ctx context.Context, ec *command.ExecContext, argv []string) (uint8, error) {
	var b strings.Builder
	for i, a := range argv {
		if i > 0 {
			b.WriteByte(' ')
		}
		quoted, err := syntax.Quote(a, syntax.LangBash)
		if err != nil {
			fmt.Fprintf(ec.Stderr, "find: cannot quote argument %q\n", a)
			return 1, nil
		}
		b.WriteString(quoted)
	}

	prog, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(b.String()), "<find>")
	if err != nil {
		fmt.Fprint(ec.Stderr, "find: cannot parse -exec command\n")
		return 1, nil
	}

	sub := ec.Runner.Subshell()
	if err := interp.StdIO(ec.Stdin, ec.Stdout, ec.Stderr)(sub); err != nil {
		return 1, err
	}
	err = sub.Run(ctx, prog)
	if err == nil {
		return 0, nil
	}
	var status interp.ExitStatus
	if errors.As(err, &status) {
		return uint8(status), nil
	}
	return 1, err
}

// deleteEntry removes a file or an empty directory. It refuses non-empty
// directories itself because not every billy backend does.
func deleteEntry(ec *command.ExecContext, p string) error {
	info, err := ec.FS.Lstat(p)
	if err != nil {
		return err
	}
	if info.IsDir() {
		entries, err := ec.FS.ReadDir(p)
		if err != nil {
			return err
		}
		if len(entries) > 0 {
			return errDirNotEmpty
		}
	}
	return ec.FS.Remove(p)
}

// unreadableReason maps a directory read failure that is the directory's
// own problem to the phrase GNU find prints for it. Anything else is not
// recoverable and must end the search.
func unreadableReason(err error) (string, bool) {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return "Permission denied", true
	case errors.Is(err, fs.ErrNotExist):
		return "No such file or directory", true
	case errors.Is(err, syscall.ENOTDIR):
		return "Not a directory", true
	case errors.Is(err, syscall.EIO):
		return "Input/output error", true
	case errors.Is(err, syscall.ELOOP):
		return "Too many levels of symbolic links", true
	case errors.Is(err, syscall.ENAMETOOLONG):
		return "File name too long", true
	}
	return "", false
}

// errorReason renders err without leaking the Go *fs.PathError wrapping.
func errorReason(err error) string {
	if errors.Is(err, errDirNotEmpty) {
		return "Directory not empty"
	}
	if reason, ok := unreadableReason(err); ok {
		return reason
	}
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	return err.Error()
}

func joinDisplay(parent, name string) string {
	if strings.HasSuffix(parent, "/") {
		return parent + name
	}
	return parent + "/" + name
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

package registry

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/go-git/go-billy/v5"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

var (
	ErrCommandNotFound = errors.New("kefka: command not found")
)

type Impl struct {
	lock     sync.Mutex
	commands map[string]command.Execer

	pwdMu sync.Mutex
	pwd   string
}

func (i *Impl) Register(name string, cmd command.Execer) {
	i.lock.Lock()
	defer i.lock.Unlock()

	i.commands[name] = cmd
}

func (i *Impl) Get(name string) (command.Execer, bool) {
	i.lock.Lock()
	defer i.lock.Unlock()

	cmd, ok := i.commands[name]
	return cmd, ok
}

func New() *Impl {
	return &Impl{
		commands: make(map[string]command.Execer),
		pwd:      ".",
	}
}

// Pwd returns the current fsys-relative working directory.
func (i *Impl) Pwd() string {
	i.pwdMu.Lock()
	defer i.pwdMu.Unlock()
	return i.pwd
}

// Resolve maps a shell path (absolute "/foo" or relative to pwd) into an
// fs.FS-relative path. Attempts to escape above the fsys root clamp to ".".
func (i *Impl) Resolve(p string) string {
	pwd := i.Pwd()
	if path.IsAbs(p) {
		p = strings.TrimPrefix(p, "/")
	} else {
		p = path.Join(pwd, p)
	}
	p = path.Clean(p)
	if p == "" || p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return "."
	}
	return p
}

// Chdir changes the fsys-relative working directory. Validates against fsys.
func (i *Impl) Chdir(fsys billy.Filesystem, target string) error {
	if target == "" {
		target = "."
	}
	next := i.Resolve(target)

	info, err := fsys.Stat(next)
	if err != nil {
		return fmt.Errorf("cd: %s: No such file or directory", target)
	}
	if !info.IsDir() {
		return fmt.Errorf("cd: %s: Not a directory", target)
	}

	i.pwdMu.Lock()
	i.pwd = next
	i.pwdMu.Unlock()
	return nil
}

func (i *Impl) Exec(ctx context.Context, fsys billy.Filesystem, runner *interp.Runner, args []string) error {
	hc := interp.HandlerCtx(ctx)

	if len(args) == 0 {
		return nil
	}

	cmdName := args[0]
	cmdArgs := args[1:]

	cmd, ok := i.Get(cmdName)
	if !ok {
		return errors.Join(fmt.Errorf("%w: %s", ErrCommandNotFound, cmdName), interp.ExitStatus(127))
	}

	return cmd.Exec(ctx, &command.ExecContext{
		Stdin:   hc.Stdin,
		Stdout:  hc.Stdout,
		Stderr:  hc.Stderr,
		Dir:     i.Pwd(),
		Environ: hc.Env,
		FS:      fsys,
		Runner:  runner,
	}, cmdArgs)
}

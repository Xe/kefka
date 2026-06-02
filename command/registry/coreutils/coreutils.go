package coreutils

import (
	"tangled.org/xeiaso.net/kefka/command/internal/clear"
	"tangled.org/xeiaso.net/kefka/command/internal/column"
	"tangled.org/xeiaso.net/kefka/command/internal/commands"
	"tangled.org/xeiaso.net/kefka/command/internal/diff"
	"tangled.org/xeiaso.net/kefka/command/internal/du"
	"tangled.org/xeiaso.net/kefka/command/internal/env"
	"tangled.org/xeiaso.net/kefka/command/internal/expr"
	"tangled.org/xeiaso.net/kefka/command/internal/file"
	"tangled.org/xeiaso.net/kefka/command/internal/gunzip"
	"tangled.org/xeiaso.net/kefka/command/internal/gzip"
	"tangled.org/xeiaso.net/kefka/command/internal/hostname"
	"tangled.org/xeiaso.net/kefka/command/internal/pwd"
	"tangled.org/xeiaso.net/kefka/command/internal/stat"
	"tangled.org/xeiaso.net/kefka/command/internal/tac"
	"tangled.org/xeiaso.net/kefka/command/internal/time"
	"tangled.org/xeiaso.net/kefka/command/internal/tree"
	"tangled.org/xeiaso.net/kefka/command/internal/whoami"
	"tangled.org/xeiaso.net/kefka/command/internal/zcat"
	"tangled.org/xeiaso.net/kefka/command/registry"
)

func Register(reg *registry.Impl) {
	reg.Register("clear", clear.Impl{})
	reg.Register("column", column.Impl{})
	reg.Register("commands", commands.Impl{Reg: reg})
	reg.Register("diff", diff.Impl{})
	reg.Register("du", du.Impl{})
	reg.Register("env", env.Impl{})
	reg.Register("expr", expr.Impl{})
	reg.Register("file", file.Impl{})
	reg.Register("gunzip", gunzip.Impl{})
	reg.Register("gzip", gzip.Impl{})
	reg.Register("hostname", hostname.Impl{})
	reg.Register("pwd", pwd.Impl{})
	reg.Register("stat", stat.Impl{})
	reg.Register("tac", tac.Impl{})
	reg.Register("time", time.Impl{})
	reg.Register("tree", tree.Impl{})
	reg.Register("whoami", whoami.Impl{})
	reg.Register("zcat", zcat.Impl{})
}

package coreutils

import (
	"github.com/Xe/kefka/command/internal/clear"
	"github.com/Xe/kefka/command/internal/column"
	"github.com/Xe/kefka/command/internal/commands"
	"github.com/Xe/kefka/command/internal/diff"
	"github.com/Xe/kefka/command/internal/du"
	"github.com/Xe/kefka/command/internal/env"
	"github.com/Xe/kefka/command/internal/file"
	"github.com/Xe/kefka/command/internal/gunzip"
	"github.com/Xe/kefka/command/internal/gzip"
	"github.com/Xe/kefka/command/internal/hostname"
	"github.com/Xe/kefka/command/internal/pwd"
	"github.com/Xe/kefka/command/internal/stat"
	"github.com/Xe/kefka/command/internal/tac"
	"github.com/Xe/kefka/command/internal/time"
	"github.com/Xe/kefka/command/internal/tree"
	"github.com/Xe/kefka/command/internal/whoami"
	"github.com/Xe/kefka/command/internal/zcat"
	"github.com/Xe/kefka/command/registry"
)

func Register(reg *registry.Impl) {
	reg.Register("clear", clear.Impl{})
	reg.Register("column", column.Impl{})
	reg.Register("commands", commands.Impl{Reg: reg})
	reg.Register("diff", diff.Impl{})
	reg.Register("du", du.Impl{})
	reg.Register("env", env.Impl{})
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

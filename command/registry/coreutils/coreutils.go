package coreutils

import (
	"tangled.org/xeiaso.net/kefka/command/internal/base64"
	"tangled.org/xeiaso.net/kefka/command/internal/basename"
	"tangled.org/xeiaso.net/kefka/command/internal/cat"
	"tangled.org/xeiaso.net/kefka/command/internal/clear"
	"tangled.org/xeiaso.net/kefka/command/internal/column"
	"tangled.org/xeiaso.net/kefka/command/internal/cp"
	"tangled.org/xeiaso.net/kefka/command/internal/cut"
	"tangled.org/xeiaso.net/kefka/command/internal/falsecmd"
	"tangled.org/xeiaso.net/kefka/command/internal/hostname"
	"tangled.org/xeiaso.net/kefka/command/internal/ls"
	"tangled.org/xeiaso.net/kefka/command/internal/truecmd"
	"tangled.org/xeiaso.net/kefka/command/registry"
)

func Register(reg *registry.Impl) {
	reg.Register("base64", base64.Impl{})
	reg.Register("basename", basename.Impl{})
	reg.Register("cat", cat.Impl{})
	reg.Register("clear", clear.Impl{})
	reg.Register("column", column.Impl{})
	reg.Register("cp", cp.Impl{})
	reg.Register("cut", cut.Impl{})
	reg.Register("false", falsecmd.Impl{})
	reg.Register("hostname", hostname.Impl{})
	reg.Register("ls", ls.Impl{})
	reg.Register("true", truecmd.Impl{})
}

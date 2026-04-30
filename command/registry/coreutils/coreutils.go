package coreutils

import (
	"tangled.org/xeiaso.net/kefka/command/internal/ls"
	"tangled.org/xeiaso.net/kefka/command/registry"
)

func Register(reg *registry.Impl) {
	reg.Register("ls", ls.Impl{})
}

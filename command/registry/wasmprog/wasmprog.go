package wasmprog

import (
	"tangled.org/xeiaso.net/kefka/command/internal/python3"
	"tangled.org/xeiaso.net/kefka/command/registry"
)

func Register(reg *registry.Impl) {
	// multiple entries for Python should exist
	reg.Register("python", python3.Impl{})
	reg.Register("python3", python3.Impl{})
}

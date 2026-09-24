package wasmprog

import (
	"github.com/Xe/kefka/command/internal/jo"
	"github.com/Xe/kefka/command/internal/jq"
	"github.com/Xe/kefka/command/internal/python3"
	"github.com/Xe/kefka/command/internal/qjs"
	"github.com/Xe/kefka/command/internal/rg"
	"github.com/Xe/kefka/command/registry"
)

func Register(reg *registry.Impl) {
	// multiple entries for Python should exist
	reg.Register("python", python3.Impl{})
	reg.Register("python3", python3.Impl{})

	reg.Register("qjs", qjs.Impl{})
	reg.Register("jo", jo.Impl{})
	reg.Register("jq", jq.Impl{})
	reg.Register("rg", rg.Impl{})
}

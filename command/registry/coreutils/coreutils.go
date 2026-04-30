package coreutils

import (
	"tangled.org/xeiaso.net/kefka/command/internal/base64"
	"tangled.org/xeiaso.net/kefka/command/internal/basename"
	"tangled.org/xeiaso.net/kefka/command/internal/cat"
	"tangled.org/xeiaso.net/kefka/command/internal/clear"
	"tangled.org/xeiaso.net/kefka/command/internal/column"
	"tangled.org/xeiaso.net/kefka/command/internal/cp"
	"tangled.org/xeiaso.net/kefka/command/internal/cut"
	"tangled.org/xeiaso.net/kefka/command/internal/date"
	"tangled.org/xeiaso.net/kefka/command/internal/diff"
	"tangled.org/xeiaso.net/kefka/command/internal/dirname"
	"tangled.org/xeiaso.net/kefka/command/internal/du"
	"tangled.org/xeiaso.net/kefka/command/internal/expand"
	"tangled.org/xeiaso.net/kefka/command/internal/expr"
	"tangled.org/xeiaso.net/kefka/command/internal/file"
	"tangled.org/xeiaso.net/kefka/command/internal/falsecmd"
	"tangled.org/xeiaso.net/kefka/command/internal/fold"
	"tangled.org/xeiaso.net/kefka/command/internal/gunzip"
	"tangled.org/xeiaso.net/kefka/command/internal/gzip"
	"tangled.org/xeiaso.net/kefka/command/internal/head"
	"tangled.org/xeiaso.net/kefka/command/internal/hostname"
	"tangled.org/xeiaso.net/kefka/command/internal/join"
	"tangled.org/xeiaso.net/kefka/command/internal/ls"
	"tangled.org/xeiaso.net/kefka/command/internal/md5sum"
	"tangled.org/xeiaso.net/kefka/command/internal/mkdir"
	"tangled.org/xeiaso.net/kefka/command/internal/mv"
	"tangled.org/xeiaso.net/kefka/command/internal/nl"
	"tangled.org/xeiaso.net/kefka/command/internal/od"
	"tangled.org/xeiaso.net/kefka/command/internal/paste"
	"tangled.org/xeiaso.net/kefka/command/internal/printf"
	"tangled.org/xeiaso.net/kefka/command/internal/pwd"
	"tangled.org/xeiaso.net/kefka/command/internal/readlink"
	"tangled.org/xeiaso.net/kefka/command/internal/rm"
	"tangled.org/xeiaso.net/kefka/command/internal/rmdir"
	"tangled.org/xeiaso.net/kefka/command/internal/seq"
	"tangled.org/xeiaso.net/kefka/command/internal/sha1sum"
	"tangled.org/xeiaso.net/kefka/command/internal/sha256sum"
	"tangled.org/xeiaso.net/kefka/command/internal/sleep"
	"tangled.org/xeiaso.net/kefka/command/internal/split"
	"tangled.org/xeiaso.net/kefka/command/internal/stat"
	"tangled.org/xeiaso.net/kefka/command/internal/tac"
	"tangled.org/xeiaso.net/kefka/command/internal/tail"
	"tangled.org/xeiaso.net/kefka/command/internal/tee"
	"tangled.org/xeiaso.net/kefka/command/internal/time"
	"tangled.org/xeiaso.net/kefka/command/internal/touch"
	"tangled.org/xeiaso.net/kefka/command/internal/tr"
	"tangled.org/xeiaso.net/kefka/command/internal/tree"
	"tangled.org/xeiaso.net/kefka/command/internal/truecmd"
	"tangled.org/xeiaso.net/kefka/command/internal/unexpand"
	"tangled.org/xeiaso.net/kefka/command/internal/uniq"
	"tangled.org/xeiaso.net/kefka/command/internal/wc"
	"tangled.org/xeiaso.net/kefka/command/internal/whoami"
	"tangled.org/xeiaso.net/kefka/command/internal/zcat"
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
	reg.Register("date", date.Impl{})
	reg.Register("diff", diff.Impl{})
	reg.Register("dirname", dirname.Impl{})
	reg.Register("du", du.Impl{})
	reg.Register("expand", expand.Impl{})
	reg.Register("expr", expr.Impl{})
	reg.Register("file", file.Impl{})
	reg.Register("false", falsecmd.Impl{})
	reg.Register("fold", fold.Impl{})
	reg.Register("gunzip", gunzip.Impl{})
	reg.Register("gzip", gzip.Impl{})
	reg.Register("head", head.Impl{})
	reg.Register("hostname", hostname.Impl{})
	reg.Register("join", join.Impl{})
	reg.Register("ls", ls.Impl{})
	reg.Register("md5sum", md5sum.Impl{})
	reg.Register("mkdir", mkdir.Impl{})
	reg.Register("mv", mv.Impl{})
	reg.Register("nl", nl.Impl{})
	reg.Register("od", od.Impl{})
	reg.Register("paste", paste.Impl{})
	reg.Register("printf", printf.Impl{})
	reg.Register("pwd", pwd.Impl{})
	reg.Register("readlink", readlink.Impl{})
	reg.Register("rm", rm.Impl{})
	reg.Register("rmdir", rmdir.Impl{})
	reg.Register("seq", seq.Impl{})
	reg.Register("sha1sum", sha1sum.Impl{})
	reg.Register("sha256sum", sha256sum.Impl{})
	reg.Register("sleep", sleep.Impl{})
	reg.Register("split", split.Impl{})
	reg.Register("stat", stat.Impl{})
	reg.Register("tac", tac.Impl{})
	reg.Register("tail", tail.Impl{})
	reg.Register("tee", tee.Impl{})
	reg.Register("time", time.Impl{Registry: reg})
	reg.Register("touch", touch.Impl{})
	reg.Register("tr", tr.Impl{})
	reg.Register("tree", tree.Impl{})
	reg.Register("true", truecmd.Impl{})
	reg.Register("unexpand", unexpand.Impl{})
	reg.Register("uniq", uniq.Impl{})
	reg.Register("wc", wc.Impl{})
	reg.Register("whoami", whoami.Impl{})
	reg.Register("zcat", zcat.Impl{})
}

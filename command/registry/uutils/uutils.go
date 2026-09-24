package uutils

import (
	"github.com/Xe/kefka/command/registry"
	"github.com/Xe/kefka/command/uutils"
)

func Register(reg *registry.Impl) {
	for _, name := range []string{
		"[", "arch", "b2sum", "base32", "base64", "basename", "basenc", "cat",
		"cksum", "comm", "cp", "csplit", "cut", "date", "dd", "dir",
		"dircolors", "dirname", "expand", "expr", "factor", "false",
		"fmt", "fold", "head", "hostid", "join", "link", "ln", "ls", "md5sum", "mkdir",
		"mktemp", "mv", "nl", "nproc", "numfmt", "od", "paste", "pathchk",
		"pr", "printenv", "printf", "ptx", "readlink", "realpath",
		"rm", "rmdir", "seq", "sha1sum", "sha224sum", "sha256sum",
		"sha384sum", "sha512sum", "shred", "shuf", "sleep", "sort", "split",
		"sum", "tail", "tee", "test", "touch", "tr", "true", "truncate", "tsort",
		"tty", "uname", "unexpand", "uniq", "unlink", "vdir", "wc", "yes",
	} {
		reg.Register(name, uutils.For(name))
	}
}

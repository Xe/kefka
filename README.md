# Kefka

![](./docs/img/kefka.jpg)

> Why do you cling to life when you know you can't live forever?

Kefka is a virtual shell environment in the vein of [just-bash](https://github.com/vercel-labs/just-bash). This is useful when you need consistent cross platform shell behaviour operating against virtual environments without the weight of a sandbox/container/jail/virtual machine/microvm. One of the major usecases for Kefka's toolset is when operating with AI agents in constrained virtual filesystems.

This will become the coreutils implementation for [yeet](https://github.com/TecharoHQ/yeet)'s Windows port.

## Code quality and current lifecycle stage

Kefka is currently _quite experimental_. It is nowhere near ready for primetime or general use. Most of the commands have not been audited for correctness or POSIX compliance. Most if not all of them were ported from just-bash using the [just-bash-port claude skill](./.claude/skills/just-bash-port/SKILL.md).

Don't use this yet. It will be good eventually.

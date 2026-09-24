#!/usr/bin/env bash

# Shrink WASM modules in place: optimize for size with wasm-opt, then remove
# all custom sections (DWARF, names, producers) with wasm-strip.
#
# Usage: wasm/shrink.sh file.wasm [file.wasm...]
#
# Needs wasm-opt (binaryen), wasm-strip (wabt), and wasm-tools.

set -euo pipefail

# wazero implements WebAssembly 2.0. Pin wasm-opt to that feature set so it
# never emits instructions wazero cannot compile (for example extended-const,
# which some toolchains declare in target_features). An explicit set also
# makes the script safe to run again on a module that is already stripped.
FEATURES=(
	--mvp-features
	--enable-bulk-memory
	--enable-bulk-memory-opt
	--enable-multivalue
	--enable-mutable-globals
	--enable-nontrapping-float-to-int
	--enable-reference-types
	--enable-sign-ext
	--enable-simd
)

if [ "$#" -eq 0 ]; then
	echo "usage: $0 file.wasm [file.wasm...]" >&2
	exit 2
fi

for f in "$@"; do
	before=$(wc -c <"$f" | tr -d ' ')
	wasm-opt "${FEATURES[@]}" -Oz --strip-debug "$f" -o "$f.tmp"
	wasm-strip "$f.tmp"
	wasm-tools validate --features=wasm2 "$f.tmp"
	mv "$f.tmp" "$f"
	after=$(wc -c <"$f" | tr -d ' ')
	echo "$f: $before -> $after bytes"
done

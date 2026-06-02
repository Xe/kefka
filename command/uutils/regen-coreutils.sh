#!/usr/bin/env bash

set -euo pipefail

COREUTILS_VERSION=0.9.0

rm -rf var
mkdir -p var
echo '*' >>./var/.gitignore

cd ./var
git clone https://github.com/uutils/coreutils
cd coreutils
git checkout "${COREUTILS_VERSION}"

git apply ../../pwd-hack.patch
cargo build --release --target wasm32-wasip1 --no-default-features --features feat_wasm

cp target/wasm32-wasip1/release/coreutils.wasm ../../coreutils.wasm

#!/bin/bash
# Run MICAPP with user-local Go toolchain deps
export PATH="/home/ttt/.local/go-install/bin:$PATH"
export LD_LIBRARY_PATH="/home/ttt/micapp-deps/usr/lib/x86_64-linux-gnu:/home/ttt/micapp-deps/usr/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
cd "$(dirname "$0")"
exec ./micapp "$@"

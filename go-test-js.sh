#!/bin/sh
here="$(dirname "$0")"
testExec="$here/wasm_exec/wasm_exec"

if [ ! -f "$testExec" ]; then
    go build -o "$testExec" "$here/wasm_exec"
fi

# one package at a time: each starts its own Chrome, and starting them all
# at once on a CI runner can take longer than chromedp waits
GOOS=js GOARCH=wasm go test -p 1 -exec "$(pwd)/$testExec" "$@"
exit $?

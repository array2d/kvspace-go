#!/bin/bash
# expected:
# === tree shows dir and file ===
# /
# ├── b/
# │   └── x	int64:1
# └── b	string:value
# /end

set -e
KV="$HOME/.local/bin/kvspace"
$KV deltree /b/

$KV set /b/ index:
$KV set /b string:value
$KV set /b/x int:1

echo "=== tree shows dir and file ==="
$KV tree --showext=false /

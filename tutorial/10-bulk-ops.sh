#!/bin/bash
# expected:
# === bulk get across dirs ===
# /bulk/a/x	int64:1
# /bulk/a/y	int64:2
# /bulk/b/z	int64:3
# === bulk del siblings ===
# /bulk/c/x	(nil)
# /bulk/c/y	(nil)
# /bulk/c/z	int64:3
# === bulk del across dirs ===
# /bulk/d/a	(nil)
# /bulk/e/b	(nil)
# /bulk/f/c	int64:6
# c
# /end

set -e
KV="$HOME/.local/bin/kvspace"

echo "=== bulk get across dirs ==="
$KV set /bulk/a/x int:1
$KV set /bulk/a/y int:2
$KV set /bulk/b/z int:3
$KV get /bulk/a/x /bulk/a/y /bulk/b/z

echo "=== bulk del siblings ==="
$KV set /bulk/c/x int:1
$KV set /bulk/c/y int:2
$KV set /bulk/c/z int:3
$KV del /bulk/c/x /bulk/c/y
$KV get /bulk/c/x /bulk/c/y /bulk/c/z

echo "=== bulk del across dirs ==="
$KV set /bulk/d/a int:4
$KV set /bulk/e/b int:5
$KV set /bulk/f/c int:6
$KV del /bulk/d/a /bulk/e/b
$KV get /bulk/d/a /bulk/e/b /bulk/f/c
$KV list /bulk/f

$KV deltree /bulk

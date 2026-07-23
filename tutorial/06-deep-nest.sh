#!/bin/bash
# expected:
# === deep tree structure ===
# /deep/a/b/c/d/e/f	int64:6
# a
# /deep/a/b/c	int64:3
# d
# /deep/a/x	int64:99
# c
# === deltree at mid level ===
# /deep/a/b/c/d/e/f	(nil)
# /deep/a/b/c/d/e	(nil)
# /deep/a/b/c	int64:3
# === deltree leaf ===
# /deep/a/x	int64:99
# /deep/a/b/c	(nil)
# a
# /end

set -e
KV="$HOME/.local/bin/kvspace"

echo "=== deep tree structure ==="
$KV set /deep/a/b/c/d/e/f int:6
$KV get /deep/a/b/c/d/e/f
$KV list /deep
$KV set /deep/a/b/c int:3
$KV get /deep/a/b/c
$KV list /deep/a/b/c
$KV set /deep/a/x int:99
$KV get /deep/a/x
$KV list /deep/a/b

echo "=== deltree at mid level ==="
$KV deltree /deep/a/b/c/d/e
$KV get /deep/a/b/c/d/e/f
$KV get /deep/a/b/c/d/e
$KV get /deep/a/b/c
$KV list /deep/a/b/c

echo "=== deltree leaf ==="
$KV get /deep/a/x
$KV deltree /deep/a/b
$KV get /deep/a/b/c
$KV list /deep

$KV deltree /deep

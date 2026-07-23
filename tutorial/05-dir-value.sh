#!/bin/bash
# expected:
# === path as value and dir ===
# /dv	int64:99
# x
# /dv/x	int64:1
# /dv/x/y	int64:2
# y
# === list sibling dirs ===
# a
# b
# /dv2	int64:0
# === del dir-value, children survive ===
# /dv2/x	int64:42
# a
# b
# x
# /end

set -e
KV="$HOME/.local/bin/kvspace"

echo "=== path as value and dir ==="
$KV set /dv int:99
$KV set /dv/x int:1
$KV get /dv
$KV list /dv
$KV get /dv/x
$KV set /dv/x/y int:2
$KV get /dv/x/y
$KV list /dv/x

echo "=== list sibling dirs ==="
$KV set /dv2/a int:10
$KV set /dv2/b int:20
$KV set /dv2 int:0
$KV list /dv2
$KV get /dv2

echo "=== del dir-value, children survive ==="
$KV set /dv2/x int:42
$KV del /dv2
$KV get /dv2/x
$KV list /dv2

$KV deltree /dv
$KV deltree /dv2

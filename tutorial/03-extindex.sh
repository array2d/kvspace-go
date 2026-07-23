#!/bin/bash
# expected:
# === read fallthrough ===
# /merge/a	int64:1
# /merge/b	int64:2
# /merge/c	int64:3
# === write upper ===
# /merge/a	int64:99
# /base/a	int64:1
# === write new key to upper ===
# /merge/d	string:upper_only
# /base/d	(nil)
# === list merge ===
# a
# d
# b
# c
# === del from upper ===
# /merge/a	int64:1
# d
# a
# b
# c
# /end

set -e
KV="$HOME/.local/bin/kvspace"

$KV set /base/a int:1
$KV set /base/b int:2
$KV set /base/c int:3
$KV extindex /merge /base

echo "=== read fallthrough ==="
$KV get /merge/a /merge/b /merge/c

echo "=== write upper ==="
$KV set /merge/a int:99
$KV get /merge/a
$KV get /base/a

echo "=== write new key to upper ==="
$KV set /merge/d string:upper_only
$KV get /merge/d /base/d

echo "=== list merge ==="
$KV list /merge

echo "=== del from upper ==="
$KV del /merge/a
$KV get /merge/a
$KV list /merge

$KV deltree /merge
$KV deltree /base

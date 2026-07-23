#!/bin/bash
# expected:
# === write upper, base unchanged ===
# /ex/upper/a	int64:99
# /ex/base/a	int64:1
# === write new key only in upper ===
# /ex/upper/z	string:upper_only
# /ex/base/z	(nil)
# === list merge upper first ===
# a
# z
# b
# c
# === delete upper reveals base ===
# /ex/upper/a	int64:1
# z
# a
# b
# c
# === nested write under extindex ===
# /ex/upper/nest/x	int64:42
# /ex/upper/nest/y	int64:43
# /ex/base/nest/x	(nil)
# x
# y
# === deltree extindex preserves base ===
# /ex/upper	(nil)
# /ex/base/a	int64:1
# a
# b
# c
# /end

set -e
KV="$HOME/.local/bin/kvspace"

$KV set /ex/base/a int:1
$KV set /ex/base/b int:2
$KV set /ex/base/c int:3
$KV extindex /ex/upper /ex/base

echo "=== write upper, base unchanged ==="
$KV set /ex/upper/a int:99
$KV get /ex/upper/a
$KV get /ex/base/a

echo "=== write new key only in upper ==="
$KV set /ex/upper/z string:upper_only
$KV get /ex/upper/z
$KV get /ex/base/z

echo "=== list merge upper first ==="
$KV list /ex/upper

echo "=== delete upper reveals base ==="
$KV del /ex/upper/a
$KV get /ex/upper/a
$KV list /ex/upper

echo "=== nested write under extindex ==="
$KV set /ex/upper/nest/x int:42
$KV set /ex/upper/nest/y int:43
$KV get /ex/upper/nest/x /ex/upper/nest/y
$KV get /ex/base/nest/x
$KV list /ex/upper/nest

echo "=== deltree extindex preserves base ==="
$KV deltree /ex/upper
$KV get /ex/upper
$KV get /ex/base/a
$KV list /ex/base

$KV deltree /ex/base

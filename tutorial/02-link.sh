#!/bin/bash
# expected:
# === read through ===
# /lnk/x	int64:42
# /lnk/y	int64:7
# === list through ===
# x
# y
# z
# === write through ===
# /tgt/w	int64:99
# x
# y
# z
# w
# === del through ancestor link ===
# /tgt/y	(nil)
# x
# z
# w
# === del link body ===
# /lnk	(nil)
# /tgt/x	int64:42
# === deltree link unlink only ===
# /lnk2	(nil)
# /tgt/z	string:hello
# /end

set -e
KV="$HOME/.local/bin/kvspace"

$KV set /tgt/x int:42
$KV set /tgt/y int:7
$KV set /tgt/z string:hello
$KV link /tgt /lnk

echo "=== read through ==="
$KV get /lnk/x /lnk/y

echo "=== list through ==="
$KV list /lnk

echo "=== write through ==="
$KV set /lnk/w int:99
$KV get /tgt/w
$KV list /tgt

echo "=== del through ancestor link ==="
$KV del /lnk/y
$KV get /tgt/y
$KV list /tgt

echo "=== del link body ==="
$KV del /lnk
$KV get /lnk
$KV get /tgt/x

echo "=== deltree link unlink only ==="
$KV link /tgt /lnk2
$KV deltree /lnk2
$KV get /lnk2
$KV get /tgt/z

$KV deltree /tgt

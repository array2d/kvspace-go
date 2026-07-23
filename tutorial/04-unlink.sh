#!/bin/bash
# expected:
# === unlink link ===
# /lnk/x	int64:1
# /lnk	(nil)
# /tgt/x	int64:1
# === unlink extindex ===
# /merge	(nil)
# /base/a	int64:2
# /end

set -e
KV="$HOME/.local/bin/kvspace"


echo "=== unlink link ==="
$KV set /tgt/x int:1
$KV link /tgt /lnk
$KV get /lnk/x
$KV unlink /lnk
$KV get /lnk
$KV get /tgt/x

echo "=== unlink extindex ==="
$KV set /base/a int:2
$KV extindex /merge /base
$KV set /merge/b int:3
$KV unlink /merge
$KV get /merge
$KV get /base/a

$KV deltree /tgt
$KV deltree /base

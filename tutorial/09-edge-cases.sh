#!/bin/bash
# expected:
# === overwrite with different type ===
# /edge/v	int64:42
# /edge/v	string:hello
# === empty dir list ===
# === del non-existent no error ===
# /edge/ghost	(nil)
# === unicode path and value ===
# /edge/名字	string:世界
# /edge/🌍	string:🫡
# === deltree non-existent no error ===
# /end

set -e
KV="$HOME/.local/bin/kvspace"

echo "=== overwrite with different type ==="
$KV set /edge/v int:42
$KV get /edge/v
$KV set /edge/v string:hello
$KV get /edge/v

echo "=== empty dir list ==="
$KV list /edge/empty

echo "=== del non-existent no error ==="
$KV del /edge/ghost
$KV get /edge/ghost

echo "=== unicode path and value ==="
$KV set /edge/名字 string:世界
$KV get /edge/名字
$KV set /edge/🌍 string:🫡
$KV get /edge/🌍

echo "=== deltree non-existent no error ==="
$KV deltree /edge/nonexist

$KV deltree /edge

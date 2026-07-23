#!/bin/bash
# expected:
# === Set & Get ===
# /t01/a	int64:42
# === Set & List ===
# a
# b
# c
# === Get bulk ===
# /t01/a	int64:42
# /t01/b	int64:7
# /t01/c	string:hello
# === Get nil ===
# /t01/nonexist	(nil)
# === Del ===
# /t01/a	(nil)
# b
# c
# === DelTree ===
# /end

set -e
KV="$HOME/.local/bin/kvspace"


echo "=== Set & Get ==="
$KV set /t01/a int:42
$KV get /t01/a

echo "=== Set & List ==="
$KV set /t01/b int:7
$KV set /t01/c string:hello
$KV list /t01

echo "=== Get bulk ==="
$KV get /t01/a /t01/b /t01/c

echo "=== Get nil ==="
$KV get /t01/nonexist

echo "=== Del ==="
$KV del /t01/a
$KV get /t01/a
$KV list /t01

echo "=== DelTree ==="
$KV deltree /t01
$KV list /

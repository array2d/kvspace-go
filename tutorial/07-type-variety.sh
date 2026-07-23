#!/bin/bash
# expected:
# === integer edge values ===
# /types/zero	int64:0
# /types/neg	int64:-9223372036854775808
# /types/pos	int64:9223372036854775807
# === float edge values ===
# /types/f0	float64:0
# /types/fneg	float64:-3.141592653589793
# /types/fpos	float64:2.718281828459045
# === bool ===
# /types/yes	bool:true
# /types/no	bool:false
# === string edge values ===
# /types/empty	string:
# /types/space	string:hello world
# /types/unicode	string:你好🫡
# /types/slash	string:a/b/c
# === nil ===
# /types/nothing	(nil)
# /end

set -e
KV="$HOME/.local/bin/kvspace"

echo "=== integer edge values ==="
$KV set /types/zero int:0
$KV set /types/neg int:-9223372036854775808
$KV set /types/pos int:9223372036854775807
$KV get /types/zero /types/neg /types/pos

echo "=== float edge values ==="
$KV set /types/f0 float:0
$KV set /types/fneg float:-3.141592653589793
$KV set /types/fpos float:2.718281828459045
$KV get /types/f0 /types/fneg /types/fpos

echo "=== bool ==="
$KV set /types/yes bool:true
$KV set /types/no bool:false
$KV get /types/yes /types/no

echo "=== string edge values ==="
$KV set /types/empty string:
$KV set /types/space "string:hello world"
$KV set /types/unicode string:你好🫡
$KV set /types/slash string:a/b/c
$KV get /types/empty /types/space /types/unicode /types/slash

echo "=== nil ==="
$KV set /types/nothing nil:
$KV get /types/nothing

$KV deltree /types

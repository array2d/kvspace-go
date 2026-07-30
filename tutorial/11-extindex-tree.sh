#!/bin/bash
# expected:
# === tree showext true ===
# /ext
# ├── z	42
# ├── x/
# │   └── z	99
# └── a	1
# === tree showext false ===
# /ext
# ├── z	42
# └── x/
#     └── z	99
# └── …/base/
# /end

set -e
KV="$HOME/.local/bin/kvspace"
$KV deltree /ext/
$KV deltree /base/

$KV set /base/ index:
$KV set /base/a int:1
$KV extindex /ext/ /base/
$KV set /ext/z int:42
$KV set /ext/x/ index:
$KV set /ext/x/z int:99

echo "=== tree showext true ==="
$KV tree /ext/

echo "=== tree showext false ==="
$KV tree --showext=false /ext/








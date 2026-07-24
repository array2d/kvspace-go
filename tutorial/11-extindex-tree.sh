#!/bin/bash
# expected:
# === tree showext true ===
# /ext
# =/base/
# ├── z	int64:42
# ├── x/
# │   └── z	int64:99
# └── a	int64:1
# === tree showext false ===
# /ext
# =/base/
# ├── z	int64:42
# └── x/
#     └── z	int64:99
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

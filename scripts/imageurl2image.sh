#!/usr/bin/env bash
# dataurl2img — convert a data URL to an image file
# Usage: dataurl2img "data:image/png;base64,..." [output.png]
set -euo pipefail

input="${1:-$(cat)}"
mime="${input%%;*}" # data:image/png
ext="${mime#data:}" # image/png
ext="${ext#*/}"     # png
out="${2:-qr.$ext}"
b64="${input#*,}" # strip everything up to the comma

echo "$b64" | base64 -d >"$out"
echo "→ $out"

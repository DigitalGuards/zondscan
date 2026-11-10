#!/usr/bin/env bash
# Generates the ShorScan favicon and app icon set in public/brand/shorscan/
# from the mark in app/components/ShorScanMark.tsx (same geometry, flat colours).
# Needs rsvg-convert and ImageMagick (convert). Run from ExplorerFrontend/:
#   bash scripts/build-shorscan-icons.sh
set -euo pipefail

OUT="public/brand/shorscan"
BG="#0b0d13"
mkdir -p "$OUT"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# The viewBox crops the 6.48 unit margin of the mark to $PAD so small icons
# keep as many pixels as possible on the blocks.
svg() { # <pad> <size>
  local pad="$1" size="$2" light="#e9ecf2" amber="#ffa729" body=""
  local r
  for xy in "16.82 6.48" "27.16 6.48" "37.50 6.48" "6.48 16.82" "47.84 16.82" \
            "16.82 37.50" "37.50 37.50" "6.48 47.84" "16.82 47.84" "37.50 47.84" "47.84 47.84"; do
    set -- $xy
    body+="<rect x=\"$1\" y=\"$2\" width=\"9.68\" height=\"9.68\" rx=\"1.4\" fill=\"$light\"/>"
  done
  for xy in "6.48 27.16" "47.84 27.16"; do
    set -- $xy
    body+="<rect x=\"$1\" y=\"$2\" width=\"9.68\" height=\"9.68\" rx=\"1.4\" fill=\"$amber\"/>"
  done
  body+="<rect x=\"16.82\" y=\"30.57\" width=\"30.36\" height=\"2.86\" rx=\"1.2\" fill=\"$amber\"/>"
  local vb; vb=$(awk -v p="$pad" 'BEGIN{printf "%.2f %.2f %.2f %.2f", 6.48-p, 6.48-p, 51.04+2*p, 51.04+2*p}')
  echo "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"$size\" height=\"$size\" viewBox=\"$vb\"><rect x=\"-100\" y=\"-100\" width=\"300\" height=\"300\" fill=\"$BG\"/>$body</svg>"
}

render() { # <pad> <size> <file>
  svg "$1" "$2" > "$tmp/m.svg"
  rsvg-convert -w "$2" -h "$2" "$tmp/m.svg" -o "$3"
}

render 1.5 16 "$OUT/favicon-16x16.png"
render 2.5 32 "$OUT/favicon-32x32.png"
render 3.5 48 "$OUT/favicon-48x48.png"
render 8 180 "$OUT/apple-touch-icon.png"
render 10 512 "$OUT/logo512.png"
convert "$OUT/favicon-16x16.png" "$OUT/favicon-32x32.png" "$OUT/favicon-48x48.png" "$OUT/favicon.ico"
ls -l "$OUT"

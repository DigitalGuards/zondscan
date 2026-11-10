#!/usr/bin/env bash
# Generates the ShorScan share card (1200x630) at public/brand/shorscan/og-image.png.
# Same layout as public/og-image.png: mark top left, wordmark, tagline, domain,
# large faint mark on the right and an amber bar along the bottom edge.
# Needs rsvg-convert and the IBM Plex Sans font. Runs from any directory:
#   bash scripts/build-shorscan-og.sh
set -euo pipefail
cd "$(dirname "$0")/.."

OUT="public/brand/shorscan/og-image.png"
LIGHT="#e9ecf2"
AMBER="#ffa729"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# Mark geometry matches app/components/ShorScanMark.tsx (64 unit box).
mark() { # <light colour> <amber colour>
  local xy body=""
  for xy in "16.82 6.48" "27.16 6.48" "37.50 6.48" "6.48 16.82" "47.84 16.82" \
            "16.82 37.50" "37.50 37.50" "6.48 47.84" "16.82 47.84" "37.50 47.84" "47.84 47.84"; do
    set -- $xy
    body+="<rect x=\"$1\" y=\"$2\" width=\"9.68\" height=\"9.68\" rx=\"1.4\" fill=\"$LIGHT_FILL\"/>"
  done
  for xy in "6.48 27.16" "47.84 27.16"; do
    set -- $xy
    body+="<rect x=\"$1\" y=\"$2\" width=\"9.68\" height=\"9.68\" rx=\"1.4\" fill=\"$AMBER_FILL\"/>"
  done
  body+="<rect x=\"16.82\" y=\"30.57\" width=\"30.36\" height=\"2.86\" rx=\"1.2\" fill=\"$AMBER_FILL\"/>"
  echo "$body"
}

LIGHT_FILL="$LIGHT" AMBER_FILL="$AMBER" small="$(mark)"
LIGHT_FILL="#16233a" AMBER_FILL="#2a2a2f" large="$(mark)"

cat > "$tmp/card.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630" viewBox="0 0 1200 630">
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="#0b0d13"/>
      <stop offset="1" stop-color="#0f1a2e"/>
    </linearGradient>
    <linearGradient id="bar" x1="0" y1="0" x2="1" y2="0">
      <stop offset="0" stop-color="$AMBER"/>
      <stop offset="1" stop-color="#4da3ff"/>
    </linearGradient>
  </defs>
  <rect width="1200" height="630" fill="url(#bg)"/>
  <svg x="700" y="40" width="560" height="560" viewBox="0 0 64 64">$large</svg>
  <svg x="94" y="104" width="112" height="112" viewBox="0 0 64 64">$small</svg>
  <text x="80" y="378" font-family="IBM Plex Sans" font-weight="700" font-size="88" fill="#f5f5f7">ShorScan</text>
  <text x="82" y="454" font-family="IBM Plex Sans" font-weight="400" font-size="32" fill="#b9bfcc">Block explorer for QRL 2.0, the post-quantum blockchain.</text>
  <text x="82" y="565" font-family="IBM Plex Sans" font-weight="600" font-size="26" fill="$AMBER">shorscan.com</text>
  <rect y="622" width="1200" height="8" fill="url(#bar)"/>
</svg>
SVG

rsvg-convert -w 1200 -h 630 "$tmp/card.svg" -o "$OUT"
ls -l "$OUT"

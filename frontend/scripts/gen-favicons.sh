#!/usr/bin/env bash
#
# Regenerate the whole favicon set, in both brand colours, from one SVG.
#
# This exists because the set drifted: favicon.svg was recoloured three times
# while the PNG and ICO fallbacks kept the value they were rasterised with in
# August, so browsers that fall back to PNG were still showing the original
# violet long after the site had gone green. A hand-run pipeline that has to be
# remembered is a pipeline that will be forgotten.
#
#   ./scripts/gen-favicons.sh
#
# rsvg-convert, not ImageMagick: ImageMagick's built-in SVG renderer flattens
# this artwork to a white square. `brew install librsvg` if it is missing.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
PUB=public
SRC=$PUB/favicon.svg

command -v rsvg-convert >/dev/null || { echo "需要 rsvg-convert：brew install librsvg"; exit 1; }
command -v magick >/dev/null || { echo "需要 ImageMagick"; exit 1; }

GREEN="#5DE31D"
VIOLET="#8E47FF"

# The sizes index.html actually references, plus the PWA and Apple ones the
# manifest points at. Nothing here is speculative — a size no document asks for
# is a file nobody fetches.
PNG_SIZES=(32 48 64 96 128 192 256 384 512)
APPLE_SIZES=(152 167 180)

render() {          # render <svg> <size> <out>
  rsvg-convert -w "$2" -h "$2" -o "$3" "$1"
}

build_set() {       # build_set <svg> <suffix>
  local svg=$1 sfx=$2
  for s in "${PNG_SIZES[@]}"; do
    render "$svg" "$s" "$PUB/favicon$sfx-${s}x${s}.png"
  done
  for s in "${APPLE_SIZES[@]}"; do
    render "$svg" "$s" "$PUB/apple-touch-icon$sfx-${s}x${s}.png"
  done
  cp "$PUB/apple-touch-icon$sfx-180x180.png" "$PUB/apple-touch-icon$sfx.png"
  # The ICO carries the small sizes only; the large ones would quadruple the
  # file for tabs that never render above 32.
  magick "$PUB/favicon$sfx-16x16.png" "$PUB/favicon$sfx-32x32.png" \
         "$PUB/favicon$sfx-48x48.png" "$PUB/favicon$sfx.ico" 2>/dev/null \
    || magick "$PUB/favicon$sfx-32x32.png" "$PUB/favicon$sfx-48x48.png" "$PUB/favicon$sfx.ico"
}

# 16 is not in PNG_SIZES (no <link> asks for it) but the ICO wants it.
render "$SRC" 16 "$PUB/favicon-16x16.png"

echo "绿色（默认）"
build_set "$SRC" ""

echo "紫色"
VSVG=$(mktemp -t favicon-violet).svg
sed "s/$GREEN/$VIOLET/gI" "$SRC" > "$VSVG"
cp "$VSVG" "$PUB/favicon-violet.svg"
render "$PUB/favicon-violet.svg" 16 "$PUB/favicon-violet-16x16.png"
build_set "$PUB/favicon-violet.svg" "-violet"
rm -f "$VSVG" "$PUB/favicon-16x16.png" "$PUB/favicon-violet-16x16.png"

echo
# Check the SVGs, not the rasters. The mark is a coloured glyph on
# transparency: flattening it to sample a pixel mixes the fill with whatever
# background the flatten used, and reports that instead. The SVG fill is the
# actual source of truth, and the PNGs are rendered from it in the same run.
echo "核对"
for f in "$SRC" "$PUB/favicon-violet.svg"; do
  printf "  %-30s %s\n" "$(basename "$f")" "$(grep -oE '#[0-9A-Fa-f]{6}' "$f" | sort -u | tr '\n' ' ')"
done
printf "  %-30s %s\n" "生成的 PNG/ICO" "$(ls "$PUB" | grep -cE '^(favicon|apple-touch-icon)' ) 个"

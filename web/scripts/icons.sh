#!/bin/sh
# Regenerates the PWA icons in public/ from the brand SVGs in ../assets/brand.
# Needs ImageMagick 7 (`magick`). Run from web/: `npm run icons` or `sh scripts/icons.sh`.
# The PNGs are committed, so this only needs re-running when the logo changes.
set -eu
cd "$(dirname "$0")/.."
brand=../assets/brand
out=public/icons
mkdir -p "$out"

render() { # render <svg> <size> <dest>
  magick -background none -density 384 "$1" -resize "$2x$2" -depth 8 -strip "$3"
}

render "$brand/leaf.svg" 192 "$out/icon-192.png"
render "$brand/leaf.svg" 512 "$out/icon-512.png"
# Maskable and Apple icons must be full-bleed (the OS applies its own mask/rounding).
render "$brand/leaf-maskable.svg" 512 "$out/maskable-512.png"
render "$brand/leaf-maskable.svg" 180 "$out/apple-touch-icon-180.png"
cp "$brand/leaf.svg" public/favicon.svg
echo "icons written to $out"

#!/bin/bash
# Build script for VS Code Web

set -euo pipefail

vscodeVersion="1.75.0"

usage() {
  cat <<EOF
Usage: $(basename "$0") [options] [OUT_DIR]

Options:
  --vscode                              Build VS Code web only
  --extension | --extensions            Build extensions only
  --all                                 Build both (default)
  -h, --help                            Show this help

Notes:
  - You can still pass OUT_DIR as a positional argument for backward compatibility.
  - Environment overrides are supported: TARGET and OUT_DIR.
Examples:
  ./build.sh --vscode ./dist
  ./build.sh --extension ./dist
  ./build.sh --all ./dist
EOF
}

# Defaults (allow env to override)
TARGET="${TARGET:-all}"
OUT_DIR="${OUT_DIR:-./dist}"

# Parse args
OUT_DIR_SET=""
TARGET_SET=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --vscode)
      if [[ -n "$TARGET_SET" ]]; then echo "Multiple targets specified" >&2; exit 2; fi
      TARGET="vscode"; TARGET_SET=1; shift ;;
    --extension|--extensions)
      if [[ -n "$TARGET_SET" ]]; then echo "Multiple targets specified" >&2; exit 2; fi
      TARGET="extensions"; TARGET_SET=1; shift ;;
    --all)
      if [[ -n "$TARGET_SET" ]]; then echo "Multiple targets specified" >&2; exit 2; fi
      TARGET="all"; TARGET_SET=1; shift ;;
    -h|--help)
      usage; exit 0;;
    --)
      shift; break;;
    -*)
      echo "Unknown option: $1" >&2; usage; exit 2;;
    *)
      # positional OUT_DIR (back-compat)
      if [[ -z "$OUT_DIR_SET" ]]; then
        OUT_DIR="$1"; OUT_DIR_SET=1; shift;
      else
        echo "Unexpected argument: $1" >&2; usage; exit 2;
      fi;;
  esac
done

# Normalize/validate
TARGET_LOWER="${TARGET,,}"
case "$TARGET_LOWER" in
  vscode|extensions|all) ;;
  *) echo "Invalid target: $TARGET (must be vscode|extensions|all)" >&2; exit 2;;
esac

if command -v realpath >/dev/null 2>&1; then
  OUT_DIR="$(realpath -m "$OUT_DIR")"
fi

mkdir -p "$OUT_DIR"

build_vscode_web() {
  # Clone vscode if not exists
  if [ ! -d "vscode" ]; then
    echo "Cloning vscode@$vscodeVersion..."
    git clone --depth 1 https://github.com/microsoft/vscode.git --branch "$vscodeVersion" vscode
  fi

  # Build vscode web version
  echo "Installing dependencies and building vscode web version..."
  (cd vscode && yarn install --frozen-lockfile && yarn gulp vscode-web-min)

  # Copy built files to output directory
  echo "Copying VS Code Web to $OUT_DIR..."
  mv ./vscode-web/* "$OUT_DIR"/ 2>/dev/null || true
  cp ./index.html "$OUT_DIR"/ 2>/dev/null || true
  echo "Build vscode web completed, output at $OUT_DIR"
}

build_extensions() {
  extensions=("remotefs")

  for ext in "${extensions[@]}"; do
    if [ ! -d "extensions/$ext" ]; then
      echo "Extension not found: $ext (skipping)"
      continue
    fi

    echo "Building extension $ext"

    ext_out_dir="$OUT_DIR/$ext"
    (cd "extensions/$ext" && yarn install --frozen-lockfile && yarn package-web --output-path ${ext_out_dir})
    if [ -f "extensions/$ext/package.json" ]; then
      cp "extensions/$ext/package.json" "$ext_out_dir/package.json"
    fi
  done
  echo "Build extensions completed, output at $OUT_DIR/extensions" 
}

case "$TARGET_LOWER" in
  vscode)
    build_vscode_web;;
  extensions)
    build_extensions;;
  all)
    build_vscode_web
    build_extensions;;
esac


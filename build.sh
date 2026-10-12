#!/usr/bin/env bash

set -euo pipefail
cd "$(dirname "$0")"
elm make src/Main.elm --output=elm.js --optimize
mkdir -p build
(cd golang-backend && go build -o ../build/ecc-crm .)
echo "Build complete. Start the app with ./build/ecc-crm"

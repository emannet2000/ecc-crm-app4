#!/usr/bin/env bash

rm -rf /home/eman/ecc-crm-app3/build/ecc-crm
set -euo pipefail
cd "$(dirname "$0")"
elm make src/Main.elm --output=elm.js --optimize
mkdir -p build
(cd golang-backend && go build -o ../build/ecc-crm .)
cd /home/eman/ecc-crm-app3/build
./ecc-crm

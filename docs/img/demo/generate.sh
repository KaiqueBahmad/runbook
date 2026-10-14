#!/bin/bash
# Regenerates docs/img/demo.gif from demo.tape and the fixture/ project.
#
# Needs vhs (https://github.com/charmbracelet/vhs) and ttyd on PATH, and go
# to build the binary the recording runs. Everything happens under
# /tmp/runbook-demo-gen, which this script wipes first and last — demo.tape
# refers to that fixed path, so it can't be changed here without also
# changing demo.tape.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
WORK=/tmp/runbook-demo-gen

rm -rf "$WORK"
mkdir -p "$WORK/bin" "$WORK/home/acme-app"
cp -r "$SCRIPT_DIR/fixture/." "$WORK/home/acme-app/"
chmod +x "$WORK/home/acme-app/services/api/run.sh"

go -C "$REPO_ROOT" build -o "$WORK/bin/runbook" ./cmd/runbook

cat > "$WORK/rcfile.bash" <<RC
PS1='\[\033[1;32m\]dev@devbox\[\033[0m\]:\[\033[1;34m\]~/acme-app\[\033[0m\]\$ '
export PS1
export PATH="$WORK/bin:\$PATH"
export HOME="$WORK/home"
cd "$WORK/home/acme-app"
clear
RC

cd "$SCRIPT_DIR"
vhs demo.tape

rm -rf "$WORK"
echo "wrote $SCRIPT_DIR/../demo.gif"

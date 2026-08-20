#!/usr/bin/env bash
# Build macwtf locally and run it on a test VM over SSH.
#
# The development machine compiles; the VM executes. Nothing macwtf installs
# ever touches the machine you are working on.
#
# Configure the target with MACWTF_VM (an ssh host alias or user@host):
#
#   export MACWTF_VM=macwtf-vm
#   scripts/vm-sync.sh validate
#   scripts/vm-sync.sh install --profile recon --dry-run
#
# With no arguments it syncs the binary and manifests without running anything.

set -euo pipefail

VM="${MACWTF_VM:-macwtf-vm}"
REMOTE_DIR="${MACWTF_VM_DIR:-~/macwtf}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$REPO_ROOT"

echo "==> building for arm64 darwin"
GOOS=darwin GOARCH=arm64 go build -trimpath -o dist/macwtf ./cmd/macwtf

echo "==> syncing to ${VM}:${REMOTE_DIR}"
ssh "$VM" "mkdir -p ${REMOTE_DIR}"
# The manifests are embedded in the binary, but syncing them too means the VM
# can test a working-tree catalogue with --manifest-dir without a rebuild.
rsync -az --delete \
	dist/macwtf \
	manifest \
	profiles \
	"${VM}:${REMOTE_DIR}/"

if [ "$#" -eq 0 ]; then
	echo "==> synced. No command given."
	exit 0
fi

echo "==> running: macwtf $*"
echo
# -t allocates a TTY so the TUI and any confirmation prompts work.
ssh -t "$VM" "cd ${REMOTE_DIR} && ./macwtf $*"

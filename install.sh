#!/usr/bin/env bash
#
# Build and install macwtf from a checkout.
#
#   git clone https://github.com/naturalstate/macWTF.git
#   cd macWTF && ./install.sh
#
# Building locally rather than downloading a binary sidesteps Gatekeeper
# entirely: a binary you compiled is never quarantined, whereas one downloaded
# through a browser is blocked until it is notarised. Until macWTF is signed
# with an Apple Developer ID, this is the honest way to ship it.

set -euo pipefail

BOLD=$'\033[1m'; DIM=$'\033[2m'; GREEN=$'\033[32m'; YELLOW=$'\033[33m'; RESET=$'\033[0m'
say()  { printf '%s\n' "$*"; }
step() { printf '%s==>%s %s\n' "$BOLD" "$RESET" "$*"; }
warn() { printf '%s%s%s\n' "$YELLOW" "$*" "$RESET"; }
dim()  { printf '%s%s%s\n' "$DIM" "$*" "$RESET"; }

PREFIX="${PREFIX:-$HOME/.local}"
BINDIR="$PREFIX/bin"

[ "$(uname -s)" = "Darwin" ] || { warn "macWTF is macOS only."; exit 1; }
if [ "$(uname -m)" != "arm64" ]; then
	warn "macWTF targets Apple Silicon. This machine is $(uname -m)."
	warn "It will build, but the catalogue assumes arm64."
fi

# Go is needed to build. It is an ordinary Homebrew formula, so offer it rather
# than sending the user away to install it and come back.
if ! command -v go >/dev/null 2>&1; then
	for p in /opt/homebrew/bin /usr/local/bin; do
		[ -x "$p/go" ] && export PATH="$p:$PATH" && break
	done
fi

if ! command -v go >/dev/null 2>&1; then
	step "Go is required to build macwtf, and is not installed."
	if command -v brew >/dev/null 2>&1 || [ -x /opt/homebrew/bin/brew ]; then
		[ -x /opt/homebrew/bin/brew ] && export PATH="/opt/homebrew/bin:$PATH"
		printf 'Install it with Homebrew now? [y/N] '
		read -r reply
		case "$reply" in
			[yY]*) brew install go ;;
			*) warn "Install Go and re-run: brew install go"; exit 1 ;;
		esac
	else
		warn "Homebrew is not installed either."
		say  "Install Homebrew first:"
		say  '    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"'
		say  "then: brew install go"
		exit 1
	fi
fi

step "Building with $(go version | awk '{print $3}')"
mkdir -p dist
# -trimpath keeps local paths out of the binary.
go build -trimpath -ldflags "-s -w" -o dist/macwtf ./cmd/macwtf

step "Installing to $BINDIR"
mkdir -p "$BINDIR"
install -m 0755 dist/macwtf "$BINDIR/macwtf"

say ""
printf '%s%s✓ macwtf installed to %s%s\n' "$BOLD" "$GREEN" "$BINDIR/macwtf" "$RESET"

case ":$PATH:" in
	*":$BINDIR:"*) say ""; say "Run: ${BOLD}macwtf${RESET}" ;;
	*)
		say ""
		warn "$BINDIR is not on your PATH."
		case "$(basename "${SHELL:-/bin/zsh}")" in
			bash) profile="$HOME/.bash_profile" ;;
			fish) profile="$HOME/.config/fish/config.fish" ;;
			*)    profile="$HOME/.zprofile" ;;
		esac
		say "Add it:"
		say "    echo 'export PATH=\"$BINDIR:\$PATH\"' >> $profile"
		say ""
		say "Or run it directly: ${BOLD}$BINDIR/macwtf${RESET}"
		;;
esac

say ""
dim "Start with:  macwtf doctor    check prerequisites"
dim "             macwtf           browse the catalogue"

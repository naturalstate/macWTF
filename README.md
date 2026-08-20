<div align="center">

# macWTF

**The tooling macOS leaves out — installed properly, in one pass.**

Pentesting, InfoSec, dev and utility tooling, through whichever backend each tool actually needs.

<br>

[![License](https://img.shields.io/badge/license-MIT-2ea44f?style=for-the-badge)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev)
[![Platform](https://img.shields.io/badge/macOS-Apple_Silicon-000000?style=for-the-badge&logo=apple&logoColor=white)](https://support.apple.com/en-us/HT211814)
[![Status](https://img.shields.io/badge/status-alpha-orange?style=for-the-badge)](#roadmap)

[![Catalogue](https://img.shields.io/badge/catalogue-24_tools-6E56CF?style=flat-square)](#the-catalogue)
[![Categories](https://img.shields.io/badge/categories-6-6E56CF?style=flat-square)](#the-catalogue)
[![Profiles](https://img.shields.io/badge/profiles-4-6E56CF?style=flat-square)](#profiles)
[![Backends](https://img.shields.io/badge/backends-brew_·_cask-FBB040?style=flat-square)](#backends)
[![Dry run](https://img.shields.io/badge/dry--run-first_class-0969DA?style=flat-square)](#dry-run)

<sub>Part of the WTF family · <a href="https://github.com/naturalstate/KaliWTF">KaliWTF</a> · macWTF · WindowsWTF · AndroidWTF</sub>

</div>

---

```console
$ macwtf install --profile recon --dry-run

  nmap          brew install --formula nmap
  masscan       brew install --formula masscan
  rustscan      brew install --formula rustscan
  ffuf          brew install --formula ffuf

  skipped
  aircrack-ng   linux-only — capture needs monitor mode; use the lab bridge
```

You pick categories. macWTF resolves them into an ordered plan, installs through the right backend for each tool, and then tells you — in one consolidated report — every manual step macOS still requires you to do by hand.

---

## Why this exists

You can get most of a toolkit onto a Mac with a Brewfile gist. What a Brewfile cannot do is the part that actually costs you an afternoon.

### Gatekeeper blocks half the security tooling

Reverse engineering and RF tools are overwhelmingly unsigned or ad-hoc signed — the people writing a disassembler are not paying Apple $99/year to notarize it. macOS tags anything downloaded by a quarantine-aware app with `com.apple.quarantine`, and Gatekeeper then refuses to launch it behind a dialog whose only real button is **Move to Trash**.

macWTF knows which tools carry that problem, and offers to clear it:

```bash
xattr -d -r com.apple.quarantine /Applications/Cutter.app
```

Never silently. That command waives a malware check on a specific binary, so it is opt-in and stated plainly before it runs.

### TCC permissions cannot be scripted at all

Full Disk Access, Screen Recording, Accessibility, Input Monitoring — without MDM, no installer on earth can grant these. A human has to click them.

So macWTF collects every permission its run requires and prints one numbered checklist at the end, naming the exact System Settings pane per tool, with a deep link. **This is the highest-value thing the project does.** Everything else is plumbing around it.

### Some of the Linux toolkit is a lie on macOS

Wireless monitor mode on the internal card has been dead since Big Sur, and no third-party adapter has working monitor-mode drivers on Apple Silicon. `aircrack-ng`, `kismet`, `hcxdumptool` and Responder install perfectly and then cannot do the job.

Marking them `linux_only` and routing them to the [lab bridge](#the-lab-bridge) is more useful than letting you discover it mid-engagement.

### Homebrew is not enough

A large share of the catalogue installs via `pipx`, `go`, `cargo`, `mas`, or a direct release download. Some entries are not packages at all — they are `defaults write` calls. Designing for that from day one is why the backend layer is an interface, not a Brewfile generator with things bolted on.

---

## Install

> **Alpha.** No binary releases yet. Build from source:

```bash
git clone https://github.com/naturalstate/macWTF.git
cd macWTF
go build -o macwtf ./cmd/macwtf
./macwtf validate
```

Requires Go 1.26+ and macOS on Apple Silicon.

---

## Usage

| Command | What it does |
|---|---|
| `macwtf` | Launch the TUI |
| `macwtf validate` | Schema and referential integrity checks, fully offline |
| `macwtf check` | Verify every manifest package name still resolves upstream |
| `macwtf list` | List tools by category |
| `macwtf list --profiles` | List available profiles |
| `macwtf install --profile pentest` | Install a whole profile |
| `macwtf install --category sdr` | Install one category |
| `macwtf install --tool nmap` | Install a single tool and its dependencies |
| `macwtf install ... --dry-run` | Print every command, execute nothing |
| `macwtf status` | What is installed, per `state.toml` |
| `macwtf remove --tool nmap` | Remove a tool |
| `macwtf export` | Dump the current selection as a profile TOML |

### Dry run

`--dry-run` is not an afterthought bolted on for safety theatre. Backends **build plans; they never execute** — every backend returns a list of commands, and a separate executor decides whether to spawn anything.

That means dry-run is not a parallel code path that can drift out of sync with a real install. It is the same plan, with execution declined.

---

## The catalogue

**The catalogue is the product. The engine is plumbing.**

Adding a tool is a TOML edit, never a code change. If installing something new would require touching Go source, the schema is missing a field — and the fix is to extend the schema, not to special-case the tool.

```toml
[[tool]]
id          = "cutter"
name        = "Cutter"
description = "Reverse engineering platform powered by Rizin"
category    = "sec-reversing"
backend     = "cask"
package     = "cutter"
app_path    = "/Applications/Cutter.app"
quarantine_strip = true
verify_cmd  = "test -d '/Applications/Cutter.app'"
license     = "free"
notes       = "Upstream ships ad-hoc signed builds that are not notarized."
```

Manifests are parsed strictly. A one-character typo in a key name is a hard failure, not a silently ignored field:

```console
$ macwtf validate
error: manifest/sec-reversing.toml: unknown field(s)
26| app_path = "/Applications/Cutter.app"
27| quarantine_stripp = true
  | ~~~~~~~~~~~~~~~~~ unknown field
```

That typo would otherwise silently disable a security-relevant flag.

### Current coverage

| Category | Tools | Notes |
|---|---|---|
| `cli` | ripgrep, fd, bat, jq, eza | Modern userland, safe baseline |
| `sec-recon` | nmap, masscan, rustscan, ffuf, gobuster | All fully functional on macOS |
| `sec-web` | burp-suite, caido, mitmproxy, sqlmap | Intercepting proxies |
| `sec-network` | wireshark, tshark, bettercap, aircrack-ng | Conflicts and a linux-only entry |
| `sec-reversing` | ghidra, cutter, imhex | Where quarantine bites |
| `utilities` | rectangle, alt-tab, maccy | Where TCC bites |

Seeded from [`macwtf-catalogue.md`](macwtf-catalogue.md), which drafts the full ~400-entry target across 20 categories.

### Flags

| Flag | Meaning |
|---|---|
| `[Q]` | Needs a quarantine strip to launch |
| `[T]` | Needs a TCC permission granted by hand |
| `[R]` | Needs Rosetta 2 |
| `[!]` | Linux-only or crippled on macOS |

---

## Backends

Homebrew is *a* backend, not *the* backend.

| Backend | Status | For |
|---|---|---|
| `brew` | Implemented | Formulae — CLI tools and libraries |
| `cask` | Implemented | GUI apps and pre-built binaries |
| `mas` | Planned | Mac App Store |
| `pipx` | Planned | Isolated Python CLIs |
| `cargo` · `go` · `npm` · `gem` | Planned | Language package managers |
| `curl` | Planned | Direct binary and release downloads |
| `git` | Planned | Clone-and-build, wordlist repos |
| `defaults` | Planned | macOS system preference writes |

---

## Profiles

Profiles compose — they include other profiles — and every selection stays individually toggleable in the TUI.

| Profile | Contents |
|---|---|
| **Baseline** | Modern CLI userland and desktop essentials |
| **Recon** | Baseline + scanning, enumeration, content discovery |
| **Web Hacking** | Baseline + intercepting proxies, injection testing |
| **Desktop** | Baseline + window management and clipboard |

Planned: Pentest, Blue Team, Cloud, SDR & RF, Hardware, Dev, Everything.

---

## The lab bridge

Rather than pretending macOS is a complete offensive platform, the `lab-bridge` category installs a hypervisor or container runtime (OrbStack, UTM, Lima), pulls a Kali ARM64 image, sorts out USB passthrough for Proxmark, HackRF and wireless adapters, and offers to stage [KaliWTF](https://github.com/naturalstate/KaliWTF) inside the guest.

A clean seam between two projects, not a limitation being hidden.

---

## Principles

- **Tool ids are permanent.** Renaming one breaks existing user state files.
- **Never hardcode `/opt/homebrew` or `/usr/local`.** Query `brew --prefix`.
- **Every install is idempotent.** Re-running a profile on a configured machine is a no-op, not a reinstall.
- **Failures are non-fatal.** Log, continue, report at the end. One dead cask must not abort a 60-tool run.
- **Nothing privileged happens silently.** `sudo`, quarantine stripping and security-setting changes are always surfaced and confirmed.
- **`validate` makes no network calls.** It must work on a plane.
- **Wordlists go to `~/.local/share/macwtf/`.** `/opt` is a Linux convention that clutters macOS.

---

## Contributing

Adding a tool means editing one TOML file and nothing else.

1. Add a `[[tool]]` block to the right file in `manifest/`.
2. Run `macwtf validate` — it will catch a bad category, a dangling dependency, a typo'd key, or a conflict that does not resolve.
3. Run `macwtf install --tool <id> --dry-run` and read the commands.
4. Open a PR.

If your tool cannot be expressed without a code change, that is a schema gap worth raising as an issue.

Package names rot — casks get renamed, formulae get deprecated, tools move between the two. Four of the first twenty entries had already drifted from the draft catalogue. `macwtf check` exists to catch that automatically.

---

## Roadmap

- [x] Manifest schema, strict parsing, offline validation
- [x] Catalogue seeded across six categories
- [ ] Backend interface with brew and cask, `--dry-run`
- [ ] Real install path with state tracking in `~/.macwtf/state.toml`
- [ ] Quarantine consent flow and the consolidated TCC report
- [ ] TUI: profile picker and category tree
- [ ] Remaining backends: mas, pipx, cargo, go, curl, git, defaults
- [ ] `macwtf check` in CI, on a schedule
- [ ] Lab bridge
- [ ] Full ~400-entry catalogue

---

## Non-goals

Cross-platform support · Intel Macs · Nix-style declarative purity · replacing Homebrew · managing your dotfiles · silent privilege escalation.

Sibling projects handle other platforms. Each picks the language that fits its target — they share a name, a brand and a manifest schema, but no code.

---

## License

MIT

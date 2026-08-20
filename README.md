# macWTF

**Automates installing the pentesting, InfoSec, dev, and utility tooling that doesn't ship with macOS.**

You pick categories. macWTF installs them through whichever backend each tool actually needs — Homebrew formulae, casks, Mac App Store, pipx, cargo, go, direct binary downloads — and then tells you, in one consolidated report, every manual step macOS still requires you to do by hand.

Apple Silicon only. macOS 14+.

```bash
macwtf                              # launch the TUI
macwtf install --profile pentest    # or go straight to it
```

---

## Why this exists

You can get most of a pentest toolkit onto a Mac with a Brewfile gist. What you can't get from a Brewfile is the part that actually costs you an afternoon:

- **Half the security tooling is unsigned.** Ghidra, Cutter, Maltego, Burp, SDR++ — Gatekeeper blocks them on first launch with a dialog whose only button is *Move to Trash*. macWTF knows which tools need `xattr -d com.apple.quarantine` and asks you before doing it.
- **TCC permissions cannot be scripted.** Full Disk Access, Screen Recording, Accessibility, Input Monitoring — without MDM, a human has to click these. Wireshark, Karabiner, AltTab, Rectangle, and bandwhich all need them. macWTF collects every one during the run and prints a numbered checklist at the end naming the exact System Settings pane per tool.
- **A meaningful chunk of the Linux toolkit is a lie on macOS.** Monitor mode on the internal wireless card has been dead since Big Sur. `aircrack-ng`, `kismet`, `hcxdumptool`, and Responder are marked as such and routed to the lab bridge instead of installing and letting you find out.
- **Homebrew is not enough.** A large share of the catalogue installs via pipx, go, cargo, mas, or a direct release download. Some entries aren't packages at all — they're `defaults write` calls.

The end-of-run manual-steps report is the single highest-value thing here. Everything else is plumbing around it.

## Design

**The tool registry is data. The execution engine is code. They never mix.**

Adding a tool is a TOML edit, never a code change. If a new tool would require touching Go source, the manifest schema is missing a field, and the fix is to extend the schema — not to special-case the tool.

```toml
[[tool]]
id          = "burp-suite"
name        = "Burp Suite Community"
description = "Web proxy and application security testing platform"
category    = "sec-web"
backend     = "cask"
package     = "burp-suite"
quarantine_strip = true
tcc_permissions  = []
verify_cmd  = "test -d '/Applications/Burp Suite Community Edition.app'"
license     = "free"
```

Two consequences worth stating plainly:

1. **Homebrew is a backend, not the backend.** The backend layer is an interface with many implementations from day one — `brew`, `cask`, `mas`, `pipx`, `cargo`, `go`, `curl`, `git`, `defaults`. This is not a Brewfile generator with things bolted on.
2. **Backends produce plans; an executor runs them.** Every backend returns a list of commands rather than executing anything itself. `--dry-run` is therefore not a separate code path — it's the executor declining to spawn. It cannot silently drift out of sync with what a real install does.

## Commands

```
macwtf                          # launch TUI
macwtf validate                 # schema + referential integrity, offline
macwtf check                    # verify every manifest package name still resolves
macwtf install --profile pentest
macwtf install --category sdr
macwtf install --tool nmap
macwtf install ... --dry-run    # print every command, execute nothing
macwtf status                   # what's installed, per state.toml
macwtf remove --tool nmap
macwtf export                   # dump current selection as a profile TOML
```

`--dry-run` is the primary debugging affordance, not an afterthought.

## Profiles

| Profile | Contents |
|---|---|
| **Baseline** | Terminal, shell, CLI quality-of-life, browsers, password manager, editor, system tweaks |
| **Pentest** | Baseline + recon + web + passwords + network + reporting + lab bridge |
| **Blue Team** | Baseline + Objective-See suite + Santa + forensics + Wireshark + cloud posture |
| **Web Hacking** | Baseline + web + recon subset + API tooling |
| **Cloud** | Baseline + cloud CLIs + cloud security + containers + IaC |
| **SDR & RF** | Baseline + SDR/RF + hardware + serial |
| **Hardware** | Baseline + serial + esptool + PlatformIO + KiCad + logic analyzer |
| **Dev** | Baseline + editors + languages + containers + databases |

Profiles compose — they can include other profiles — and every selection stays individually toggleable in the TUI.

## The lab bridge

Rather than pretending macOS is a complete offensive platform, the `lab-bridge` category installs a hypervisor or container runtime (OrbStack, UTM, Lima), pulls a Kali ARM64 image, sorts out USB passthrough for Proxmark/HackRF/wireless adapters, and offers to stage [KaliWTF](https://github.com/naturalstate/KaliWTF) inside the guest.

This is a clean seam between two projects, not a limitation being hidden.

## Principles

- Tool `id` values are permanent. Renaming one breaks user state files.
- Never hardcode `/opt/homebrew` or `/usr/local`. Query `brew --prefix`.
- Every install is idempotent. Re-running a profile on a configured machine is a no-op.
- Failures are non-fatal. Log, continue, and report everything at the end. One dead cask must not abort a 60-tool run.
- Nothing privileged happens silently. `sudo`, quarantine stripping, and security-setting changes are surfaced and confirmed.
- Wordlists and payloads go to `~/.local/share/macwtf/`, not `/opt`.
- `validate` makes no network calls.

## Non-goals

Cross-platform support · Intel Macs · Nix-style declarative purity · replacing Homebrew · managing your dotfiles · silent privilege escalation.

## Status

Early. Building toward an MVP: manifest schema and validation, brew + cask backends, dry-run, real installs with state tracking, and the quarantine/TCC report. TUI and the remaining backends follow. The catalogue is being seeded from [`macwtf-catalogue.md`](macwtf-catalogue.md).

## Family

Platform-native setup tools sharing a name, a brand, and a manifest schema — but zero code. Each picks the language that fits its platform.

**KaliWTF** · **macWTF** · **WindowsWTF** · **AndroidWTF**

## License

MIT

# MacWTF: Tool Catalogue

Draft catalogue to seed the manifest. Backend notation:
- `brew` = Homebrew formula
- `cask` = Homebrew cask (GUI app)
- `mas` = Mac App Store (via `mas` CLI)
- `pipx` / `cargo` / `go` / `npm` = language package managers
- `curl` = direct binary/release download
- `defaults` = macOS system preference write
- `manual` = requires user interaction (download, license, TCC approval)

Flags: **[Q]** needs quarantine strip · **[T]** needs TCC permission · **[R]** needs Rosetta · **[!]** Linux-only or crippled on macOS

---

## 0. Bootstrap (always runs, not optional)

| Tool | Backend | Notes |
|---|---|---|
| Xcode Command Line Tools | manual | `xcode-select --install`, prerequisite for everything |
| Homebrew | curl | Detect existing install, don't clobber |
| Rosetta 2 | manual | `softwareupdate --install-rosetta`, only if user picks an x86 cask |
| mas | brew | Mac App Store CLI |
| pipx | brew | Isolated Python CLI installs |
| uv | brew | Fast Python package/venv manager |

---

## 1. Terminal (single-select, always installed)

| Tool | Backend | Notes |
|---|---|---|
| **Tabby** | cask | Default recommendation. Config sync, split panes, SSH profile manager |
| Ghostty | cask | Native Swift, GPU accelerated, extremely fast |
| iTerm2 | cask | The long-standing default, deepest feature set |
| WezTerm | cask | Lua config, multiplexing built in |
| Kitty | cask | GPU accelerated, kittens ecosystem |
| Alacritty | cask | Minimal, fastest cold start |
| Warp | cask | AI-assisted, account required |
| Hyper | cask | Electron, plugin ecosystem |
| Terminal.app | builtin | Escape hatch, no install |

**Post-install:** set as default handler, install chosen Nerd Font, apply a theme, import a starter config.

---

## 2. Shell & Prompt

### Shells
| Tool | Backend | Notes |
|---|---|---|
| zsh | builtin | macOS default since Catalina |
| bash 5 | brew | macOS ships bash 3.2 from 2007, worth upgrading |
| fish | brew | Sane defaults, autosuggestions out of the box |
| nushell | brew | Structured data pipelines |

### Frameworks (single-select per shell)
| Tool | Backend | Notes |
|---|---|---|
| **Oh My Zsh** | curl | The obvious default, huge plugin library |
| Prezto | git | Lighter, faster than OMZ |
| zinit | curl | Fastest loader, turbo mode |
| antidote | brew | Modern OMZ alternative |
| Oh My Fish / fisher | curl | For fish users |

### Prompts
| Tool | Backend | Notes |
|---|---|---|
| Starship | brew | Cross-shell, config in one TOML file |
| Powerlevel10k | git | Zsh only, configuration wizard |
| oh-my-posh | brew | Cross-shell, cross-platform (shares config with WindowsWTF) |

### Zsh plugins
`zsh-autosuggestions` · `zsh-syntax-highlighting` · `zsh-history-substring-search` · `fzf-tab` · `zsh-you-should-use` · `zsh-vi-mode`

### Nerd Fonts
`font-meslo-lg-nerd-font` · `font-jetbrains-mono-nerd-font` · `font-hack-nerd-font` · `font-fira-code-nerd-font` · `font-caskaydia-cove-nerd-font` · `font-iosevka-nerd-font` · `font-symbols-only-nerd-font`

---

## 3. CLI Quality of Life

### Modern replacements
`eza` (ls) · `bat` (cat) · `fd` (find) · `ripgrep` (grep) · `sd` (sed) · `dust` (du) · `duf` (df) · `procs` (ps) · `btop` (top) · `zoxide` (cd) · `delta` (diff) · `hyperfine` (time) · `choose` (cut)

### Navigation & search
`fzf` · `atuin` (shell history sync) · `broot` · `yazi` (TUI file manager) · `mc` (midnight commander) · `tldr` · `navi` (cheatsheets)

### GNU coreutils
macOS ships BSD variants that break most Linux-oriented scripts. Install and optionally prepend to PATH:
`coreutils` · `gnu-sed` · `gawk` · `findutils` · `grep` · `gnu-tar` · `moreutils` · `parallel` · `watch`

### Data wrangling
`jq` · `yq` · `gron` · `fx` · `miller` · `csvkit` · `xsv` · `dasel`

### Network CLI
`curl` · `wget` · `aria2` · `httpie` · `xh` · `mtr` · `iperf3` · `speedtest-cli` · `dog` (dig) · `gping` · `bandwhich` **[T]**

### Multiplexers & editors
`tmux` · `zellij` · `screen` · `neovim` · `vim` · `micro` · `helix` · `nano`

### Git
`git` · `gh` · `glab` · `lazygit` · `git-delta` · `git-lfs` · `tig` · `gitui` · `pre-commit` · `git-filter-repo` · `gitleaks`

### System monitoring
`htop` · `btop` · `glances` · `ncdu` · `smartmontools` · `stats` (cask, menu bar)

### Misc
`direnv` · `just` · `entr` · `mise` (runtime version manager) · `pv` · `tree` · `rsync` · `age` · `sops` · `gnupg` · `pinentry-mac` · `openssl` · `sshpass` · `tesseract` · `ffmpeg` · `imagemagick` · `pandoc` · `qpdf` · `poppler` · `exiftool`

---

## 4. Browsers

### Chromium family
| Tool | Backend | Notes |
|---|---|---|
| Google Chrome | cask | The baseline, needed for testing |
| Chromium | cask | Open source build |
| Ungoogled Chromium | cask | Google integration stripped out |
| Brave | cask | Built-in blocking, Tor windows |
| Vivaldi | cask | Heavily customizable, built-in tools |
| Opera | cask | |
| Opera GX | cask | |
| Thorium | curl | Performance-patched Chromium |
| Arc | cask | |
| Microsoft Edge | cask | Needed for Entra/M365 testing scenarios |

### Firefox family
| Tool | Backend | Notes |
|---|---|---|
| Firefox | cask | |
| Firefox Developer Edition | cask | |
| LibreWolf | cask | Hardened, telemetry stripped, uBlock preloaded |
| Waterfox | cask | |
| Zen Browser | cask | Arc-like UX on Firefox |
| Floorp | cask | |
| Mullvad Browser | cask | Tor Browser hardening without the Tor network |
| Tor Browser | cask | |

### Testing / proxy-friendly
Consider a "pentest browser profile" post-install: a dedicated Firefox or Chromium profile with proxy pre-pointed at Burp/Caido on 8080, Burp CA cert imported, and FoxyProxy, Wappalyzer, Cookie-Editor, HackBar, User-Agent Switcher installed.

---

## 5. VPN, Privacy & Network Security

| Tool | Backend | Notes |
|---|---|---|
| Tailscale | cask | Mesh VPN |
| **Twingate** | cask | ZTNA client |
| Mullvad VPN | cask | |
| Proton VPN | cask | |
| NordVPN | cask | |
| Windscribe | cask | |
| IVPN | cask | |
| WireGuard | mas | Official client |
| `wireguard-tools` | brew | CLI |
| OpenVPN Connect | cask | |
| Tunnelblick | cask | |
| Cloudflare WARP | cask | |
| **LuLu** | cask | Objective-See outbound firewall, free |
| Little Snitch | cask | Paid, deeper than LuLu |
| Murus / Vallum | cask | pf frontends |
| Pi-hole / AdGuard Home | manual | Points at homelab, config only |
| NextDNS | cask | |

---

## 6. Remote Access

| Tool | Backend | Notes |
|---|---|---|
| **RustDesk** | cask | Self-hostable relay, open source |
| Chrome Remote Desktop | manual | Requires Chrome + host installer + TCC **[T]** |
| AnyDesk | cask | |
| TeamViewer | cask | |
| Parsec | cask | Low latency, good for lab GUI work |
| Jump Desktop | mas | |
| Windows App (ex-Microsoft Remote Desktop) | mas | RDP |
| RealVNC Viewer | cask | |
| Screens | mas | |
| Moonlight | cask | |
| **Termius** | cask | SSH client with sync |
| Royal TSX | cask | Multi-protocol connection manager |
| SecureCRT | cask | Paid |
| `sshuttle` | brew | Poor man's VPN over SSH |
| `mosh` | brew | Roaming-tolerant SSH |
| `autossh` | brew | Persistent tunnels |

---

## 7. Development

### Editors & IDEs
| Tool | Backend | Notes |
|---|---|---|
| **Visual Studio Code** | cask | |
| VSCodium | cask | Telemetry-free VS Code |
| Cursor | cask | |
| Zed | cask | |
| Sublime Text | cask | |
| JetBrains Toolbox | cask | Gateway to PyCharm/IntelliJ/etc |
| Neovim + LazyVim | brew + git | Config bootstrap |
| Xcode | mas | Large, optional |

### Containers & virtualization
| Tool | Backend | Notes |
|---|---|---|
| **OrbStack** | cask | Fast, low overhead, best Docker experience on Apple Silicon |
| Docker Desktop | cask | |
| Colima | brew | CLI-only Docker/Lima |
| Lima | brew | Linux VMs from CLI |
| Podman Desktop | cask | |
| Rancher Desktop | cask | |
| **UTM** | cask | QEMU frontend, free |
| VMware Fusion | manual | Free for personal, requires Broadcom account |
| Parallels Desktop | cask | Paid, best Windows-on-ARM experience |
| Vagrant | cask | |
| `qemu` | brew | |

### Languages & runtimes
`python@3.13` · `pyenv` · `uv` · `go` · `rustup-init` · `node` · `fnm` · `bun` · `deno` · `ruby` · `rbenv` · `temurin` (cask) · `dotnet-sdk` (cask) · `php` · `perl` · `lua`

### Infra & cloud tooling
`terraform` · `opentofu` · `ansible` · `pulumi` · `kubectl` · `k9s` · `helm` · `kubectx` · `stern` · `awscli` · `azure-cli` · `google-cloud-sdk` · `doctl` · `flyctl` · `packer` · `vault` · `consul`

### API & database
| Tool | Backend | Notes |
|---|---|---|
| Postman | cask | |
| Insomnia | cask | |
| Bruno | cask | Local-first, no account |
| Hoppscotch | cask | |
| TablePlus | cask | |
| DBeaver Community | cask | |
| Beekeeper Studio | cask | |
| `pgcli` / `mycli` | pipx | |
| Redis Insight | cask | |

---

## 8. Security: Recon & OSINT

| Tool | Backend | Notes |
|---|---|---|
| nmap | brew | |
| masscan | brew | |
| rustscan | brew | |
| naabu | go | |
| subfinder | go | |
| amass | brew | |
| assetfinder | go | |
| dnsx | go | |
| httpx | go | Note: conflicts with Python `httpx`, alias it |
| katana | go | |
| nuclei | brew | Plus template sync |
| gau | go | |
| waybackurls | go | |
| gobuster | brew | |
| ffuf | brew | |
| feroxbuster | brew | |
| dirsearch | pipx | |
| whatweb | brew | |
| wafw00f | pipx | |
| theHarvester | pipx | |
| SpiderFoot | pipx | |
| recon-ng | pipx | |
| Maltego | cask | **[Q]** |
| sherlock | pipx | |
| holehe | pipx | |
| maigret | pipx | |
| dnsrecon | pipx | |
| massdns | brew | |
| puredns | go | |
| shuffledns | go | |
| asnmap | go | |
| tlsx | go | |
| cero | go | |
| BBOT | pipx | |
| EyeWitness | pipx | Needs headless Chrome |
| gowitness | go | |
| `whois` / `dig` / `host` | builtin | |

---

## 9. Security: Web Application

| Tool | Backend | Notes |
|---|---|---|
| **Burp Suite Community** | cask | **[Q]** |
| Burp Suite Professional | manual | License required |
| **Caido** | cask | Rust-based, lighter than Burp |
| OWASP ZAP | cask | **[Q]** |
| mitmproxy | brew | |
| Proxyman | cask | Native macOS, excellent UI |
| Charles Proxy | cask | |
| sqlmap | brew | |
| commix | pipx | |
| XSStrike | pipx | |
| dalfox | go | |
| arjun | pipx | |
| ParamSpider | pipx | |
| jwt-tool | pipx | |
| wpscan | brew | |
| nikto | brew | |
| testssl.sh | brew | |
| sslscan | brew | |
| sslyze | pipx | |
| hakrawler | go | |
| kiterunner | go | |
| graphql-cop / clairvoyance / InQL | pipx | GraphQL testing |
| Postman/Bruno | cask | API testing, see Dev section |
| `wappalyzer` CLI | npm | |

**Post-install:** generate Burp CA cert and offer to import into the pentest browser profile and system keychain.

---

## 10. Security: Passwords & Cracking

| Tool | Backend | Notes |
|---|---|---|
| hashcat | brew | Metal GPU support on Apple Silicon is real but uneven |
| john-jumbo | brew | |
| hydra | brew | |
| medusa | brew | |
| ncrack | brew | |
| cewl | brew | |
| crunch | brew | |
| name-that-hash | pipx | |
| hashid | pipx | |
| haiti | brew | Hash identifier |
| CyberChef | cask | Offline desktop build |
| SecLists | git | Clone to `/opt/seclists`, export `$SECLISTS` |
| rockyou / PayloadsAllTheThings | git | Wordlist bundle |
| `mentalist` | cask | Wordlist generator GUI |

---

## 11. Security: Network & Internal

| Tool | Backend | Notes |
|---|---|---|
| Wireshark | cask | **[T]** needs ChmodBPF, prompt for it |
| `tshark` / `tcpdump` | brew | |
| bettercap | brew | **[T]** |
| ettercap | brew | |
| arp-scan | brew | |
| netcat / `ncat` / `socat` | brew | |
| impacket | pipx | |
| **NetExec** (nxc) | pipx | Successor to CrackMapExec |
| smbmap | pipx | |
| enum4linux-ng | pipx | |
| ldapdomaindump | pipx | |
| kerbrute | go | |
| evil-winrm | gem | |
| BloodHound CE | cask/docker | Plus `neo4j` |
| SharpHound collectors | curl | Store in `/opt`, for transfer to targets |
| chisel | brew | |
| ligolo-ng | go | |
| proxychains-ng | brew | Note: SIP breaks it for system binaries |
| frp | brew | |
| Responder | pipx | **[!]** Needs Linux, route to VM |
| hcxdumptool / hcxtools | brew | **[!]** Capture is Linux-only |
| aircrack-ng | brew | **[!]** No monitor mode on macOS internal card |
| kismet | brew | **[!]** Needs external adapter + Linux for most modes |
| Angry IP Scanner | cask | |
| LanScan / iNet Network Scanner | mas | Native alternatives |

---

## 12. Security: Cloud & Container

`scoutsuite` (pipx) · `prowler` (pipx) · `pacu` (pipx) · `cloudsplaining` (pipx) · `cloudfox` (go) · `ROADrecon` (pipx, Azure) · `AzureHound` (curl) · `GCPBucketBrute` (pipx) · `trivy` (brew) · `grype` (brew) · `syft` (brew) · `checkov` (pipx) · `tfsec` (brew) · `kube-bench` (brew) · `kubescape` (brew) · `steampipe` (brew) · `dive` (brew) · `hadolint` (brew)

---

## 13. Security: Mobile

| Tool | Backend | Notes |
|---|---|---|
| android-platform-tools | cask | adb, fastboot |
| Android Studio | cask | Emulator, large |
| scrcpy | brew | Screen mirror/control |
| apktool | brew | |
| jadx | brew | |
| dex2jar | brew | |
| frida / frida-tools | pipx | |
| objection | pipx | |
| MobSF | docker | |
| libimobiledevice | brew | iOS device interaction |
| ideviceinstaller | brew | |
| ios-deploy | brew | |
| class-dump | brew | |
| Apktool GUI / Bytecode Viewer | curl | **[Q]** |
| `apkleaks` | pipx | |

---

## 14. Security: Reverse Engineering & Forensics

| Tool | Backend | Notes |
|---|---|---|
| Ghidra | cask | **[Q]**, needs JDK |
| radare2 | brew | |
| rizin | brew | |
| Cutter | cask | **[Q]** |
| Binary Ninja | manual | Paid |
| Hopper Disassembler | cask | Paid, macOS-native |
| IDA Free | manual | |
| binwalk | brew | |
| `yara` | brew | |
| capa | pipx | |
| floss | pipx | |
| Detect It Easy | cask | |
| ImHex | cask | |
| Hex Fiend | cask | Native macOS hex editor |
| 010 Editor | cask | Paid |
| volatility3 | pipx | |
| sleuthkit | brew | |
| Autopsy | manual | |
| foremost / testdisk / photorec | brew | |
| bulk_extractor | brew | |
| `exiftool` | brew | |
| `lldb` / `otool` / `codesign` / `nm` | builtin | Xcode CLT |
| `gdb` | brew | Codesigning hassle on macOS |

---

## 15. Security: macOS-Specific Defense

The Objective-See suite is the single highest-value bundle here.

| Tool | Backend | Notes |
|---|---|---|
| LuLu | cask | Outbound firewall |
| BlockBlock | cask | Persistence monitoring |
| KnockKnock | cask | Persistent item scanner |
| TaskExplorer | cask | Process inspector |
| RansomWhere? | cask | Encryption behavior monitor |
| OverSight | cask | Mic/camera access alerts |
| DoNotDisturb | cask | Lid-open detection |
| Netiquette | cask | Network connection monitor |
| Santa | cask | Google binary authorization |
| Pareto Security | cask | Config posture checks |
| Suspicious Package | cask | Inspect .pkg before install |
| KextViewr / What's Your Sign | cask | |

---

## 16. SDR, RF & Hardware

| Tool | Backend | Notes |
|---|---|---|
| GNU Radio | brew | Large build |
| gqrx | cask | |
| SDR++ | curl | **[Q]** |
| CubicSDR | cask | |
| SDRangel | cask | |
| `rtl-sdr` | brew | |
| `hackrf` | brew | |
| `airspy` | brew | |
| `limesuite` | brew | |
| `soapysdr` + modules | brew | |
| `rtl_433` | brew | |
| `dump1090-mutability` | brew | ADS-B |
| `multimon-ng` | brew | |
| Universal Radio Hacker (urh) | pipx | |
| `inspectrum` | brew | |
| SigDigger | curl | |
| **Proxmark3 client** | brew (tap: RfidResearchGroup) | Build from source for RDV4 |
| ChameleonUltraGUI | curl | **[Q]** |
| qFlipper | cask | |
| `mfoc` / `mfcuk` / `libnfc` | brew | MIFARE attacks |
| `esptool` | pipx | |
| `esphome` | pipx | |
| arduino-cli | brew | |
| Arduino IDE | cask | |
| PlatformIO Core | pipx | |
| Thonny | cask | MicroPython |
| `minicom` / `picocom` | brew | Serial |
| CoolTerm | cask | Serial GUI |
| Serial | cask | Paid, best-in-class |
| `sigrok-cli` + PulseView | brew/cask | Logic analyzer |
| Saleae Logic 2 | cask | |
| KiCad | cask | |
| Fritzing | cask | |
| OpenSCAD | cask | |
| OrcaSlicer / PrusaSlicer / Bambu Studio | cask | 3D printing |
| `avrdude` | brew | |
| `openocd` | brew | |
| WiGLE upload helper | script | CSV processing for wardrive data |

---

## 17. Reporting & Documentation

| Tool | Backend | Notes |
|---|---|---|
| Obsidian | cask | |
| Notion | cask | |
| Joplin | cask | |
| Logseq | cask | |
| Typora | cask | |
| Pandoc | brew | |
| BasicTeX / MacTeX | cask | LaTeX for report pipelines |
| Draw.io | cask | |
| Excalidraw | cask | |
| OmniGraffle | cask | Paid |
| Figma | cask | |
| Microsoft Office | cask | Report deliverables |
| LibreOffice | cask | |
| SysReptor | docker | Self-hosted pentest reporting |
| Ghostwriter | docker | |
| Dradis | docker | |

### Screenshots & capture
CleanShot X (cask) · Shottr (cask, free) · Kap (cask) · OBS (cask) · LICEcap (cask) · Loom (cask)

---

## 18. Utilities & Desktop Quality of Life

### Window management & launchers
Rectangle (cask) · AltTab (cask) **[T]** · Raycast (cask) · Alfred (cask) · Hammerspoon (cask) **[T]** · Karabiner-Elements (cask) **[T]** · BetterTouchTool (cask) · Yabai + skhd (brew, needs SIP partially disabled)

### Menu bar
Stats (cask) · Ice (cask, free Bartender alternative) · Hidden Bar (mas) · Maccy (cask, clipboard) · MonitorControl (cask) · Mos (cask, scroll fix) · Itsycal (cask)

### Files & storage
The Unarchiver (mas) · Keka (cask) · Cyberduck (cask) · ForkLift (cask) · Transmit (cask) · Syncthing (brew) · rclone (brew) · LocalSend (cask) · DaisyDisk (cask) · Pearcleaner (cask, free AppCleaner alternative) · AppCleaner (cask) · OnyX (cask)

### Media
IINA (cask) · VLC (cask) · HandBrake (cask) · ImageOptim (cask) · yt-dlp (brew) · Audacity (cask)

### Credentials
1Password (cask) · Bitwarden (cask) · KeePassXC (cask) · Proton Pass (cask) · Yubico Authenticator (cask) · `ykman` (brew) · Secretive (cask, SSH keys in Secure Enclave) · `gnupg` + `pinentry-mac` (brew)

---

## 19. macOS System Tweaks (`defaults` category)

Opt-in toggles, each individually selectable and each with a documented revert:

- Show hidden files in Finder
- Show all file extensions
- Show path bar and status bar in Finder
- Default Finder view to list, search current folder by default
- Disable `.DS_Store` on network and USB volumes
- Faster key repeat, shorter delay until repeat
- Disable press-and-hold for accents (enables key repeat in vim)
- Disable automatic capitalization, smart quotes, smart dashes (breaks pasted code)
- Dock: autohide, no delay, faster animation, remove recent apps
- Screenshot location to `~/Screenshots`, format PNG, no drop shadow
- Disable Gatekeeper prompts for a specific directory (scoped, not global)
- Enable firewall + stealth mode
- Require password immediately after sleep
- Disable Spotlight indexing on lab volumes
- Show battery percentage
- Expand save and print dialogs by default
- Disable "Are you sure you want to open this application" for known-good dirs
- Enable Touch ID for `sudo` (`/etc/pam.d/sudo_local`)

---

## 20. Lab Bridge (the KaliWTF handoff)

For the Linux-only half of the toolkit. This is what makes MacWTF honest about its limits.

1. Install a hypervisor or container runtime (OrbStack, UTM, Lima, or VMware Fusion)
2. Pull Kali ARM64 (container image or installer ISO for Fusion)
3. Configure USB passthrough guidance for Proxmark, HackRF, and wireless adapters
4. Offer to fetch and stage KaliWTF inside the guest
5. Set up shared folders, clipboard, and an SSH config entry for the VM

Also worth including: a `docker-compose` bundle for the self-hosted services that pair well with this (SysReptor, BloodHound CE + Neo4j, MobSF, Ghostwriter).

---

## Profile Definitions (starting point)

| Profile | Includes |
|---|---|
| **Baseline** | Bootstrap, terminal, shell, CLI QoL, browsers (Chrome + Firefox), password manager, Rectangle, Raycast, VS Code, system tweaks |
| **Pentest** | Baseline + Recon + Web + Passwords + Network + Lab Bridge + Reporting + pentest browser profile |
| **Blue Team** | Baseline + Objective-See suite + Santa + forensics + Wireshark + cloud posture tools |
| **SDR & RF** | Baseline + SDR/RF + hardware + serial tooling |
| **Web Hacking** | Baseline + Web + Recon (subset) + browsers + API tools |
| **Cloud** | Baseline + cloud CLIs + cloud security + containers + IaC |
| **Hardware / Embedded** | Baseline + serial + esptool + PlatformIO + KiCad + slicers + logic analyzer |
| **Dev** | Baseline + editors + languages + containers + databases + API tools |
| **Everything** | All of the above, with a confirmation prompt about disk usage |

---

## Open Design Questions

1. **Dotfile strategy.** Does MacWTF manage dotfiles (chezmoi/stow), or just install packages and leave config alone? Managing them is higher value and higher risk of clobbering existing setups.
2. **Uninstall path.** Track installs in state so `macwtf remove --profile sdr` works, or leave removal to the user?
3. **Manifest sharing with KaliWTF.** Same TOML schema across both projects even with separate repos and separate code, so a tool's metadata can be authored once?
4. **Wordlist and payload staging.** `/opt` is the Linux convention; on macOS `/usr/local/share` or `~/.local/share` is cleaner. Pick one and be consistent.
5. **License-gated tools.** Burp Pro, Binary Ninja, Parallels. Install the shell and prompt for license, or exclude entirely?

#!/usr/bin/env python3
"""Convert the draft catalogue markdown into manifest TOML entries.

Existing hand-written entries win. Those carry verified package names, real
notes and platform blocks that a guess cannot reproduce, so this only adds
tools that are not already in the manifests.

Package names are guessed from the display name and will be wrong for some
entries. That is expected and is what `macwtf check` is for: import, check,
fix the list it gives you.
"""
import os, re, sys, glob

DRAFT = sys.argv[1] if len(sys.argv) > 1 else "macwtf-catalogue.md"
OUTDIR = sys.argv[2] if len(sys.argv) > 2 else "manifest"

# Draft heading -> manifest file. Anything unmapped is skipped rather than
# dumped into a catch-all, so a new draft section is a visible gap.
CATEGORY = {
    "Bootstrap": "bootstrap",
    "Terminal": "terminal",
    "Shell & Prompt": "shell",
    "CLI Quality of Life": "cli",
    "Browsers": "browsers",
    "VPN, Privacy & Network Security": "vpn",
    "Remote Access": "remote",
    "Development": "dev",
    "Security: Recon & OSINT": "sec-recon",
    "Security: Web Application": "sec-web",
    "Security: Passwords & Cracking": "sec-passwords",
    "Security: Network & Internal": "sec-network",
    "Security: Cloud & Container": "sec-cloud",
    "Security: Mobile": "sec-mobile",
    "Security: Reverse Engineering & Forensics": "sec-reversing",
    "Security: macOS-Specific Defense": "sec-macos",
    "SDR, RF & Hardware": "sdr",
    "Reporting & Documentation": "reporting",
    "Utilities & Desktop Quality of Life": "utilities",
}

MACOS_BACKENDS = {"brew","cask","mas","pipx","cargo","go","npm","gem",
                  "curl","git","defaults","builtin","manual"}
# Backends macWTF cannot execute; entries using them are imported without a
# macos block so they stay in the shared catalogue but out of this platform's.
UNSUPPORTED = {"docker","script"}

FLAGS = {"[Q]":"quarantine","[T]":"tcc","[R]":"rosetta","[!]":"linuxonly"}

def clean(s):
    return re.sub(r"\s+"," ", s.replace("**","").replace("`","")).strip()

def slug(s):
    s = re.sub(r"[^a-z0-9]+","-", s.lower()).strip("-")
    return re.sub(r"-+","-",s) or "x"

def esc(s):
    return s.replace("\\","\\\\").replace('"','\\"')

def existing_ids():
    ids = set()
    for f in glob.glob(os.path.join(OUTDIR,"*.toml")):
        for m in re.finditer(r'^id\s*=\s*"([^"]+)"', open(f).read(), re.M):
            ids.add(m.group(1))
    return ids

def parse():
    out, category, sub = [], None, None
    for line in open(DRAFT, encoding="utf-8"):
        s = line.strip()

        m = re.match(r"^##\s+(?!#)(.*)$", s)
        if m:
            t = re.sub(r"^\d+\.\s*","", clean(m.group(1)))
            t = re.sub(r"\s*\(.*\)$","", t)
            category, sub = CATEGORY.get(t), None
            continue
        m = re.match(r"^###\s+(.*)$", s)
        if m:
            sub = clean(m.group(1)); continue
        if not category:
            continue

        if s.startswith("|") and s.count("|") >= 3:
            cells = [c.strip() for c in s.strip("|").split("|")]
            if len(cells) < 2 or set(cells[0]) <= set("-: ") or cells[0].lower() in ("tool","profile"):
                continue
            out.append((category, clean(cells[0]), clean(cells[1]),
                        clean(cells[2]) if len(cells) > 2 else ""))
        elif "·" in s and not s.startswith(("#","|",">")):
            for part in s.split("·"):
                part = part.strip()
                if not part: continue
                note = re.search(r"\(([^)]*)\)", part)
                nm = clean(re.sub(r"\([^)]*\)","",part))
                if not nm or len(nm) > 48: continue
                hint = clean(note.group(1)) if note else ""
                be = "brew"
                for b in MACOS_BACKENDS:
                    if re.search(rf"\b{b}\b", hint.lower()): be = b
                if sub and "font" in sub.lower(): be = "cask"
                out.append((category, nm, be, hint))
    return out

def backend_of(raw, name):
    raw = raw.lower()
    for b in sorted(MACOS_BACKENDS | UNSUPPORTED, key=len, reverse=True):
        if re.search(rf"\b{re.escape(b)}\b", raw):
            return b
    return "brew"

def pkg_of(name, backend):
    # Take the first alternative from "a / b / c", drop parentheticals.
    n = re.sub(r"\(.*?\)","", name.split("/")[0]).strip()
    n = n.lower()
    n = re.sub(r"[^a-z0-9.@+ -]+","", n)
    n = re.sub(r"[\s_]+","-", n).strip("-")
    return n

def flags_of(text):
    found = []
    for marker, key in FLAGS.items():
        if marker in text:
            found.append(key)
    return found, re.sub(r"\[[QTR!]\]","",text).strip()

def main():
    have = existing_ids()
    rows = parse()
    per_cat, seen, skipped = {}, set(have), []

    for cat, name, rawbe, notes in rows:
        flags, notes = flags_of(notes)
        nflags, name = flags_of(name)
        flags = set(flags) | set(nflags)

        tid = slug(re.sub(r"\(.*?\)","",name.split("/")[0]))
        if not tid or tid in seen:
            continue
        seen.add(tid)

        be = backend_of(rawbe, name)
        lic = "paid" if "paid" in (notes+rawbe).lower() or "license required" in notes.lower() else \
              "freemium" if "account required" in notes.lower() else "free"

        per_cat.setdefault(cat, []).append(dict(
            id=tid, name=name, cat=cat, backend=be, pkg=pkg_of(name, be),
            notes=notes, flags=flags, license=lic,
            unsupported=(be in UNSUPPORTED) or ("linuxonly" in flags),
        ))

    total = 0
    for cat, tools in sorted(per_cat.items()):
        path = os.path.join(OUTDIR, f"{cat}.toml")
        new = not os.path.exists(path)
        with open(path, "a", encoding="utf-8") as f:
            if new:
                f.write(f"# {cat}\n#\n"
                        "# Imported from the draft catalogue. Package names are guesses and\n"
                        "# some will be wrong; `macwtf check` finds them.\n")
            for t in tools:
                total += 1
                f.write(f'\n[[tool]]\nid = "{t["id"]}"\n')
                f.write(f'name = "{esc(t["name"])}"\n')
                desc = t["notes"] or t["name"]
                f.write(f'description = "{esc(desc[:150])}"\n')
                f.write(f'category = "{cat}"\n')
                f.write(f'license = "{t["license"]}"\n')
                if t["unsupported"]:
                    # No macos block: stays in the shared catalogue, absent
                    # from this platform's, exactly like aircrack-ng.
                    reason = "needs Linux" if "linuxonly" in t["flags"] else f'{t["backend"]} backend not implemented'
                    f.write(f'notes = "Not in the macOS catalogue: {reason}."\n')
                    f.write(f'[tool.kali]\nbackend = "apt"\npackage = "{t["pkg"]}"\n')
                    continue
                f.write(f'[tool.macos]\nbackend = "{t["backend"]}"\n')
                f.write(f'package = "{t["pkg"]}"\n')
                if "quarantine" in t["flags"]:
                    f.write(f'app_path = "/Applications/{esc(t["name"].split("/")[0].strip())}.app"\n')
                    f.write('quarantine_strip = true\n')
                if "tcc" in t["flags"]:
                    f.write('tcc_permissions = ["accessibility"]\n')
                if "rosetta" in t["flags"]:
                    f.write('requires_rosetta = true\narch = ["x86_64"]\n')
                f.write('unverified = true\n')

    print(f"imported {total} new tools into {len(per_cat)} files "
          f"(kept {len(have)} existing)")

main()

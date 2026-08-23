#!/usr/bin/env python3
"""Apply `macwtf check --json` results back to the manifests.

Three actions, all mechanical:
  ok / deprecated -> clear `unverified`, the name resolves
  wrong-type      -> swap brew <-> cask, which check already diagnosed
  missing         -> leave `unverified` set; a human has to find the real name

Nothing is invented here. Anything check could not decide stays flagged, so the
flag is a shrinking to-do list rather than a thing to remember to look at.
"""
import json, re, sys, collections

results = json.load(open(sys.argv[1] if len(sys.argv) > 1 else "/tmp/check.json"))
by_file = collections.defaultdict(list)
for r in results:
    by_file[r["file"]].append(r)

stats = collections.Counter()

for path, rows in by_file.items():
    text = open(path, encoding="utf-8").read()
    # Split on entry boundaries so edits stay inside the right tool.
    blocks = re.split(r"(?=^\[\[tool\]\])", text, flags=re.M)

    index = {}
    for i, b in enumerate(blocks):
        m = re.search(r'^id\s*=\s*"([^"]+)"', b, re.M)
        if m:
            index[m.group(1)] = i

    for r in rows:
        i = index.get(r["id"])
        if i is None:
            continue
        b = blocks[i]
        v = r["verdict"]

        if v in ("ok", "deprecated"):
            new = re.sub(r"^unverified = true\n", "", b, flags=re.M)
            if new != b:
                stats["verified"] += 1

            # Fill in the upstream description where ours is a placeholder.
            # The importer used the draft's terse note, or the tool's own name
            # when there was nothing, and neither describes anything. A
            # hand-written description always wins: those were considered,
            # whereas the registry one-liner is merely accurate.
            desc = (r.get("description") or "").strip().rstrip(".")
            if desc:
                m = re.search(r'^description = "([^"]*)"$', new, re.M)
                if m:
                    current = m.group(1)
                    name_m = re.search(r'^name = "([^"]*)"$', new, re.M)
                    name = name_m.group(1) if name_m else ""
                    placeholder = (
                        current == "" or current == name
                        or current.lower() == name.lower()
                        or len(current) < 12
                    )
                    if placeholder:
                        esc = desc.replace("\\", "\\\\").replace('"', '\\"')
                        new = new[:m.start()] + f'description = "{esc}"' + new[m.end():]
                        stats["described"] += 1
            blocks[i] = new

        elif v == "wrong-type":
            want = "cask" if "is a cask" in r["detail"] else "brew"
            new = re.sub(r'^backend = "(brew|cask)"$', f'backend = "{want}"', b, flags=re.M)
            new = re.sub(r"^unverified = true\n", "", new, flags=re.M)
            if new != b:
                stats["retyped"] += 1
            blocks[i] = new

        elif v in ("missing", "disabled"):
            stats["still-unverified"] += 1

    open(path, "w", encoding="utf-8").write("".join(blocks))

for k, n in sorted(stats.items()):
    print(f"  {k:18s} {n}")

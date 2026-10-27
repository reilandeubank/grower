"""Print release payloads (JSON lines) for each version in CHANGELOG.md, oldest first.

GitHub release notes render every newline as a line break, so the changelog's
one-sentence-per-line continuations are joined back into single lines.
"""
import json
import re
import sys

repo, changelog = sys.argv[1], sys.argv[2]
src = open(changelog, encoding="utf-8").read()
sections = re.findall(r"^## (\d+\.\d+\.\d+) \(\d{4}-\d{2}-\d{2}\)\n(.*?)(?=^## |\Z)", src, re.S | re.M)
versions = [v for v, _ in sections]


def join_lines(body):
    out = []
    for line in body.strip().split("\n"):
        prev = out[-1] if out else ""
        continues_item = line.startswith("  ") and prev.strip()
        continues_para = line.strip() and prev.strip() and not line.startswith("- ") and not prev.startswith("- ")
        if continues_item or continues_para:
            out[-1] = prev + " " + line.strip()
        else:
            out.append(line)
    return "\n".join(out).strip()


for i in reversed(range(len(sections))):
    version, body = sections[i]
    tag = f"v{version}"
    footer = [f"**Image:** `ghcr.io/{repo}:{version}`"]
    if i + 1 < len(versions):
        footer.append(f"**Full changelog:** https://github.com/{repo}/compare/v{versions[i + 1]}...{tag}")
    else:
        footer.append(f"**Commits:** https://github.com/{repo}/commits/{tag}")
    print(json.dumps({
        "tag_name": tag,
        "name": tag,
        "body": join_lines(body) + "\n\n" + "  \n".join(footer),
        "make_latest": "false",
    }))

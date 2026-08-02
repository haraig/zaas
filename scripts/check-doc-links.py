#!/usr/bin/env python3
"""Verify that internal documentation anchors actually resolve.

Checks three things:

  1. In-file links     - [text](#anchor)
  2. Cross-file links  - [text](other.md#anchor), including that other.md exists
  3. runbook_url labels in deploy/prometheus.rules.yaml

Anchors are generated with GitHub's algorithm, because GitHub is the only thing
that renders these files - docs/ is not served by the website.

External http(s) links are ignored: this checks anchors, not link liveness.

Usage: scripts/check-doc-links.py [--verbose]
Exits non-zero if any link is broken.
"""

import difflib
import re
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
RULES_FILE = REPO_ROOT / "deploy" / "prometheus.rules.yaml"

# ``## Heading`` at the start of a line, ignoring fenced code blocks.
HEADING_RE = re.compile(r"^(#{1,6})\s+(.*?)\s*#*\s*$")
FENCE_RE = re.compile(r"^\s*(```|~~~)")
# ``[text](target)`` - target captured, nested brackets not supported (not used here).
LINK_RE = re.compile(r"\[[^\]]*\]\(([^)]+)\)")
RUNBOOK_URL_RE = re.compile(r"runbook_url:\s*\S*?runbook\.md(#[^\s\"']*)")


def github_anchor(heading: str) -> str:
    """Reproduce GitHub's heading-to-anchor conversion.

    Lowercase, strip anything that is not a word character, whitespace or a
    hyphen, then turn runs of whitespace into single hyphens. Inline markdown
    (links, code, emphasis) is reduced to its text first.
    """
    text = heading
    text = re.sub(r"\[([^\]]*)\]\([^)]*\)", r"\1", text)  # [text](url) -> text
    text = text.replace("`", "")
    text = re.sub(r"[*_]{1,3}(.+?)[*_]{1,3}", r"\1", text)  # bold / italic
    text = text.replace("<", "").replace(">", "")
    text = text.lower()
    text = re.sub(r"[^\w\s-]", "", text)
    return re.sub(r"\s+", "-", text.strip())


def collect_anchors(path: Path) -> set[str]:
    """Every anchor GitHub would generate for this file, duplicates suffixed."""
    anchors: set[str] = set()
    seen: dict[str, int] = {}
    in_fence = False

    for line in path.read_text(encoding="utf-8").splitlines():
        if FENCE_RE.match(line):
            in_fence = not in_fence
            continue
        if in_fence:
            continue

        match = HEADING_RE.match(line)
        if not match:
            continue

        base = github_anchor(match.group(2))
        if not base:
            continue

        # GitHub suffixes repeated headings: foo, foo-1, foo-2, ...
        count = seen.get(base, 0)
        seen[base] = count + 1
        anchors.add(base if count == 0 else f"{base}-{count}")

    return anchors


def iter_links(path: Path):
    """Yield (line_number, target) for every markdown link outside code fences."""
    in_fence = False
    for lineno, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        if FENCE_RE.match(line):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        for target in LINK_RE.findall(line):
            yield lineno, target.strip()


def suggest(anchor: str, available: set[str]) -> str:
    close = difflib.get_close_matches(anchor, sorted(available), n=1, cutoff=0.4)
    return f"  did you mean: #{close[0]}" if close else ""


def markdown_files() -> list[Path]:
    files = sorted((REPO_ROOT / "docs").rglob("*.md"))
    files += sorted(REPO_ROOT.glob("*.md"))
    return files


def main() -> int:
    verbose = "--verbose" in sys.argv
    files = markdown_files()
    anchors = {path: collect_anchors(path) for path in files}
    failures: list[str] = []
    checked = 0

    for path in files:
        rel = path.relative_to(REPO_ROOT)

        for lineno, target in iter_links(path):
            if target.startswith(("http://", "https://", "mailto:")):
                continue

            file_part, _, anchor = target.partition("#")

            if not anchor:
                # Plain relative file link - just confirm the file exists.
                if file_part and not file_part.startswith("#"):
                    resolved = (path.parent / file_part).resolve()
                    if not resolved.exists():
                        checked += 1
                        failures.append(f"{rel}:{lineno}\n  {target} -> no such file")
                continue

            checked += 1

            if file_part:
                resolved = (path.parent / file_part).resolve()
                if not resolved.exists():
                    failures.append(f"{rel}:{lineno}\n  {target} -> no such file")
                    continue
                if resolved not in anchors:
                    anchors[resolved] = collect_anchors(resolved)
                available = anchors[resolved]
            else:
                resolved = path
                available = anchors[path]

            if anchor not in available:
                failures.append(
                    f"{rel}:{lineno}\n  #{anchor} -> no such heading in "
                    f"{resolved.relative_to(REPO_ROOT)}{suggest(anchor, available)}"
                )

    # runbook_url labels in the alerting rules.
    rules_checked = 0
    if RULES_FILE.exists():
        runbook = REPO_ROOT / "docs" / "reference" / "runbook.md"
        available = anchors.get(runbook) or collect_anchors(runbook)
        rel = RULES_FILE.relative_to(REPO_ROOT)

        for lineno, line in enumerate(RULES_FILE.read_text(encoding="utf-8").splitlines(), 1):
            match = RUNBOOK_URL_RE.search(line)
            if not match:
                continue
            rules_checked += 1
            anchor = match.group(1).lstrip("#")
            if anchor not in available:
                failures.append(
                    f"{rel}:{lineno}\n  #{anchor} -> no such heading in "
                    f"docs/reference/runbook.md{suggest(anchor, available)}"
                )

    if verbose:
        for path in files:
            print(f"  {path.relative_to(REPO_ROOT)}: {len(anchors[path])} anchors")

    print(f"checking {len(files)} markdown files ... {checked} anchor links")
    print(f"checking {RULES_FILE.relative_to(REPO_ROOT)} ... {rules_checked} runbook_url labels")

    if failures:
        print(f"\n{len(failures)} broken link(s):\n", file=sys.stderr)
        for failure in failures:
            print(failure, file=sys.stderr)
        return 1

    print("all internal documentation links resolve")
    return 0


if __name__ == "__main__":
    sys.exit(main())

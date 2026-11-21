#!/usr/bin/env python3
"""Check that docs cite source lines that still describe what they claim.

Docs in this repository cite code by exact `path:line`. An edit that inserts
lines above a citation does not remove the citation, so the line still exists
while the thing it points at has moved. A bare existence check therefore cannot
catch the drift, and guessing from arbitrary identifiers in the sentence would
report drift the checker cannot actually prove. So the only content assertion
is a conservative one: when a line names an environment variable in backticks
and every citation on that line points into the same file, that variable must
appear within +/-5 lines of at least one cited line. Everything else is checked
for existence and line range only.

Historical ledgers are excluded: they record positions as observed at audit
time, not the current tree.

Exit status 1 on any proven drift.
"""
import re
import sys
import pathlib

ROOT = pathlib.Path(__file__).resolve().parent.parent

# Documents describing the current tree. Historical ledgers are excluded: they
# record positions as observed at audit time, not the current tree.
EXCLUDE = {"todo.md", "docs/审计-文档一致性.md"}


def doc_files():
    out = sorted(str(p.relative_to(ROOT)) for p in ROOT.glob("docs/**/*.md"))
    out += [str(p.relative_to(ROOT)) for p in ROOT.glob("README*.md")]
    return [d for d in out if d not in EXCLUDE and ".vitepress/dist" not in d]

# `path:line` where path starts with a ./ or ../ relative link or a repo path.
CITE = re.compile(r"\b((?:\.\./)?[A-Za-z0-9_\-./]+(?:\.go|\.proto|\.json|\.mod|\.sum|\.md|\.mts|\.mjs|\.html)):(\d+)")

# Only SCREAMING_SNAKE environment variables are unambiguous enough to assert
# against: a prose word or a path component can never spell them. Other
# identifiers (CamelCase names, `README`, `SPEC`, `POST` in "POST /path")
# would produce false positives, so they are deliberately not checked.
ENV = re.compile(r"`\b((?:JWT|FABRIC|TLS|POSTGRES|GRPC)_[A-Z_]+|PORT)\b`")

WINDOW = 5
seen_cache = {}


def read_lines(rel):
    if rel in seen_cache:
        return seen_cache[rel]
    p = ROOT / rel
    data = p.read_text(encoding="utf-8", errors="replace").splitlines() if p.is_file() else None
    seen_cache[rel] = data
    return data


def resolve(rel):
    """Map a citation path as written in docs to a repo-relative path.

    Docs cite repo-root paths (`cmd/api/main.go`); `../`-prefixed links are
    relative to docs/ themselves. Returns None when no candidate exists.
    """
    base = rel[3:] if rel.startswith("../") else rel
    if (ROOT / base).is_file():
        return base
    alt = str(pathlib.Path("docs") / rel)
    if (ROOT / alt).is_file():
        return alt
    return None


def main():
    checked = drift = 0
    problems = []
    for doc in doc_files():
        text = (ROOT / doc).read_text(encoding="utf-8")
        lines = text.splitlines()
        for ln, line in enumerate(lines, 1):
            cites = [(m.group(1), int(m.group(2))) for m in CITE.finditer(line)]
            if not cites:
                continue
            env = {m.group(1) for m in ENV.finditer(line)}
            bases = {resolve(rel) for rel, _ in cites}
            # Assert identifiers only when every citation on the line points
            # into one file, so a paragraph citing several files cannot get
            # its identifiers matched against the wrong one.
            assert_env = env and None not in bases and len(bases) == 1
            windows = []
            for rel, num in cites:
                base = resolve(rel)
                if base is None:
                    continue
                src = read_lines(base)
                if src is None:
                    continue
                if num < 1 or num > len(src):
                    problems.append(f"{doc}:{ln} cites {rel}:{num} but that file has {len(src)} lines")
                    drift += 1
                    continue
                checked += 1
                lo, hi = max(1, num - WINDOW), min(len(src), num + WINDOW)
                windows.append("\n".join(src[lo - 1:hi]))
            if assert_env and windows:
                for ident in sorted(env):
                    if not any(ident in w for w in windows):
                        problems.append(
                            f"{doc}:{ln} cites "
                            f"{', '.join(f'{r}:{n}' for r, n in cites)} "
                            f"but '{ident}' is absent from every +/-{WINDOW} window"
                        )
                        drift += 1
    print(f"citations checked: {checked}, proven drift: {drift}")
    for p in problems:
        print("  DRIFT:", p)
    return 1 if drift else 0


if __name__ == "__main__":
    sys.exit(main())

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

Separately, the configuration docs claim to list every environment variable the
code reads (verified by hand once, todo.md row 128): every SCREAMING_SNAKE
`os.Getenv` literal in non-generated code must have a row in both the English
and Chinese configuration tables, and every table row must name a variable the
code actually reads.

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

# Docs outside docs/ and the root README pair. deploy/README.md, the web
# placeholder and the generated-code README are all citable, so they must stay
# in the matrix: any citation a future edit adds to them is checked like the
# rest instead of being invisible.
EXTRA_DOCS = [
    "deploy/README.md",
    "web/README.md",
    "internal/gatewaypb/gen/README.md",
]


def doc_files():
    out = sorted(str(p.relative_to(ROOT)) for p in ROOT.glob("docs/**/*.md"))
    out += [str(p.relative_to(ROOT)) for p in ROOT.glob("README*.md")]
    out += [d for d in EXTRA_DOCS if (ROOT / d).is_file()]
    return [d for d in out if d not in EXCLUDE and ".vitepress/dist" not in d]

# `path:line` or `path:start-end` where path starts with a ./ or ../ relative
# link or a repo path. Both endpoints of a range are captured so each can be
# validated; a range whose end is missing (`path:NN-`) is caught by MALFORMED.
# The path may carry a file extension or be the extensionless `Makefile`
# (checked because its target lines are cited by the build docs).
CITE = re.compile(r"\b((?:\.\./)?[A-Za-z0-9_\-./]+\.(?:go|proto|json|mod|sum|md|mts|mjs|html)|Makefile):(\d+)(?:-(\d+))?")

# `path:NN-` not followed by a digit: a truncated range like `main.go:248-`
# otherwise passes silently because CITE still matches `:248`.
MALFORMED = re.compile(r"(?:\.(?:go|proto|json|mod|sum|md|mts|mjs|html)|Makefile):\d+-(?!\d)")

# Only SCREAMING_SNAKE environment variables are unambiguous enough to assert
# against: a prose word or a path component can never spell them. Other
# identifiers (CamelCase names, `README`, `SPEC`, `POST` in "POST /path")
# would produce false positives, so they are deliberately not checked.
ENV = re.compile(r"`\b((?:JWT|FABRIC|TLS|POSTGRES|GRPC)_[A-Z_]+|PORT)\b`")

WINDOW = 5
seen_cache = {}

# todo.md is excluded as a *document* (it records positions as observed at audit
# time), but docs still point into it at exact lines, e.g. "the gap is tracked at
# todo.md:77". Such a pointer must land on a list item: hitting a heading, a
# blank line or another section's prose silently breaks it.
TODO_ANCHOR = re.compile(r"^\s*-")

# Env coverage: code os.Getenv literals must match the configuration tables.
GETENV = re.compile(r'os\.Getenv\("([A-Z][A-Z_]+)"\)')
DOC_ROW = re.compile(r"^\|\s*`([A-Z][A-Z_]+)`\s*\|", re.M)
CONFIG_DOCS = ["docs/configuration.md", "docs/zh/configuration.md"]


def go_files():
    for p in ROOT.rglob("*.go"):
        s = str(p)
        if "/gen/" in s or s.endswith("_test.go"):
            continue
        yield p


def env_coverage():
    code = set()
    for p in go_files():
        code |= set(GETENV.findall(p.read_text(encoding="utf-8", errors="replace")))
    rows = {}
    for d in CONFIG_DOCS:
        text = (ROOT / d).read_text(encoding="utf-8")
        rows[d] = set(DOC_ROW.findall(text))
    problems = []
    for d in CONFIG_DOCS:
        for name in sorted(code - rows[d]):
            problems.append(f"{name} is read by code but has no row in {d}")
        for name in sorted(rows[d] - code):
            problems.append(f"{d} documents {name} but the code never reads it")
    print(f"env coverage: {len(code)} code vars, "
          + ", ".join(f"{d}: {len(rows[d])} rows" for d in CONFIG_DOCS)
          + f", mismatches: {len(problems)}")
    return problems


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
            cites = [(m.group(1), int(m.group(2)), m.group(3)) for m in CITE.finditer(line)]
            if not cites:
                continue
            if MALFORMED.search(line):
                problems.append(f"{doc}:{ln} has a truncated range (`path:NN-`)")
                drift += 1
            env = {m.group(1) for m in ENV.finditer(line)}
            bases = {resolve(rel) for rel, _, _ in cites}
            # Assert identifiers only when every citation on the line points
            # into one file, so a paragraph citing several files cannot get
            # its identifiers matched against the wrong one.
            assert_env = env and None not in bases and len(bases) == 1
            windows = []
            for rel, num, end in cites:
                base = resolve(rel)
                if base is None:
                    continue
                src = read_lines(base)
                if src is None:
                    continue
                for n in [num] if end is None else [num, int(end)]:
                    if n < 1 or n > len(src):
                        problems.append(f"{doc}:{ln} cites {rel}:{n} but that file has {len(src)} lines")
                        drift += 1
                        continue
                    checked += 1
                    if base == "todo.md" and not TODO_ANCHOR.match(src[n - 1]):
                        problems.append(
                            f"{doc}:{ln} cites {rel}:{n} but todo.md line {n} is not a list item"
                        )
                        drift += 1
                        continue
                    lo, hi = max(1, n - WINDOW), min(len(src), n + WINDOW)
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
    env_problems = env_coverage()
    problems += env_problems
    drift += len(env_problems)
    print(f"citations checked: {checked}, proven drift: {drift}")
    for p in problems:
        print("  DRIFT:", p)
    return 1 if drift else 0


if __name__ == "__main__":
    sys.exit(main())

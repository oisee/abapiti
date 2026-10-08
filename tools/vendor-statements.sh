#!/usr/bin/env bash
# Vendor abaplint core's statement-parser closure into
# tsfront/testdata/stmts/src, verbatim at the pinned commit (see
# LICENSE.abaplint). The closure is the transitive import closure of the files
# phase 2 lowers, plus the files those need for type checking only
# (objects/program.ts, abap_file.ts, 3_structures, ...). Run from the abapiti
# root:
#   tools/vendor-statements.sh <abaplint/packages/core/src>
set -euo pipefail
src=${1:?usage: vendor-statements.sh <abaplint/packages/core/src>}
out="$(cd "$(dirname "$0")/.." && pwd)/tsfront/testdata/stmts"
[ -d "$src/abap/2_statements" ] || { echo "not an abaplint core src: $src" >&2; exit 2; }

# The roots whose whole import closure is vendored (typecheck + lower).
roots=(
  position.ts virtual_position.ts version.ts
  files/_ifile.ts files/memory_file.ts
  _iregistry.ts _imacro_references.ts
  abap/2_statements/statement_parser.ts
  abap/artifacts.ts
  abap/nodes/index.ts
)

python3 - "$src" "${roots[@]}" <<'EOF' > /tmp/vendor-stmts-list.txt
import os, re, sys
src = sys.argv[1]
os.chdir(src)
roots = sys.argv[2:]
def imports(path):
    text = open(path, encoding='utf8').read()
    out = set()
    for m in re.finditer(r'from\s+["\'](\.[^"\']+)["\']', text):
        d = os.path.dirname(path)
        r = os.path.normpath(os.path.join(d, m.group(1))) if d else m.group(1)
        for cand in (r, r + '.ts', os.path.join(r, 'index.ts')):
            if cand.startswith('..'):
                break
            if os.path.isfile(os.path.join(src, cand)):
                out.add(cand)
                break
        else:
            print(f"WARN unresolved {m.group(1)} in {path}", file=sys.stderr)
    return out
seen, todo = set(), list(roots)
while todo:
    f = todo.pop()
    if f in seen:
        continue
    seen.add(f)
    todo.extend(imports(f))
for f in sorted(seen):
    print(f)
EOF

count=$(wc -l < /tmp/vendor-stmts-list.txt)
echo "vendoring $count files into $out/src"
rm -rf "$out/src"
while IFS= read -r f; do
  mkdir -p "$out/src/$(dirname "$f")"
  cp "$src/$f" "$out/src/$f"
done < /tmp/vendor-stmts-list.txt
cp "$out/../LICENSE.abaplint" "$out/src/../LICENSE.abaplint" 2>/dev/null || cp "$(dirname "$0")/../tsfront/testdata/lexer/LICENSE.abaplint" "$out/LICENSE.abaplint"
echo done

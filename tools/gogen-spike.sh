#!/usr/bin/env bash
# Read-only input; all npm, source copies and generated output live under /tmp.
set -euo pipefail
cd "$(dirname "$0")/.."
input=${1:-/mnt/ssd-510/osg/abapiti/gogen-pilot}
scratch=$(mktemp -d /tmp/gogen-lowering.XXXXXX)
echo "Scratch (retained for inspection): $scratch"
export GOCACHE=${GOCACHE:-$HOME/.cache/go-build}
export GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false
cp -r "$input/tools" "$scratch/tools"
cp "$input/tsconfig.gogen.json" "$scratch/tsconfig.gogen.json"
printf '{"type":"module","private":true}\n' > "$scratch/package.json"
(cd "$scratch" && npm install --cache "$scratch/npm-cache" --ignore-scripts --no-audit --no-fund @abaplint/core@2.120.59 @types/node@20.19.0)
# tsfront needs explicit ambient Node types; preserve every other original option.
python3 - "$scratch" <<'PY'
import json, pathlib, sys
p = pathlib.Path(sys.argv[1]) / 'tsconfig.gogen.json'
c = json.loads(p.read_text())
c['compilerOptions']['types'] = ['node']
p.write_text(json.dumps(c, indent=2) + '\n')
PY
mkdir -p "$scratch/stub/tools/gogen"
cp "$scratch"/tools/gogen/*.ts "$scratch/stub/tools/gogen/"
cp "$scratch"/{package.json,tsconfig.gogen.json} "$scratch/stub/"
ln -s "$scratch/node_modules" "$scratch/stub/node_modules"
# NodeNext resolves .mjs imports to .d.mts (the ESM form of a .d.ts stub).
cat > "$scratch/stub/tools/gogen/frontend.d.mts" <<'TS'
import type {Program, Class, Param, Type, Argument} from './ir.ts';
export declare function omittedFactoryCall(program: Program, cls: Class, method: {name: string; params: readonly Param[]; returning: {type: Type}}): {args: Argument[]};
export declare class Unsupported extends Error { constructor(message: string); }
TS
cat > "$scratch/stub/tools/gogen/emit-go.d.mts" <<'TS'
import type {Program} from './ir.ts';
import type {Layers, SymbolCollector} from './symbols.ts';
export declare function emitGo(program: Program, pkg?: string, layers?: Layers | null, unitBuild?: boolean, symbols?: SymbolCollector): string;
TS
go build -o "$scratch/gogen-spike" ./cmd/gogen-spike
"$scratch/gogen-spike" -config "$scratch/tsconfig.gogen.json" -out "$scratch/result-ts-js"
"$scratch/gogen-spike" -config "$scratch/tsconfig.gogen.json" -js -out "$scratch/result-js"
"$scratch/gogen-spike" -config "$scratch/stub/tsconfig.gogen.json" -out "$scratch/result-stub"
cp -r cmd/gogen-spike/testdata "$scratch/probe"
"$scratch/gogen-spike" -config "$scratch/probe/tsconfig.json" -out "$scratch/result-probe"

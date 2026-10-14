# gogen frontend cache: cold, warm and one edited class

Measurement date: 2026-10-14. Base: `8ad792f50a93708a334136369c71bea496d22018`
(`origin/main`, certified 11,060 shinkansen). OSG:
`c681c9f7ee888044051ea4e5e67c13a30ecbf5c8` (#730).
Upstream abaplint: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`.

Unchanged warm frontend runs save **368.5 s (75.5%, 4.08x)** on
`abap/statements` and **605.4 s (71.1%, 3.46x)** on `rules`. Whole-harness
time falls **47.9%** and **50.6%**, respectively, but this includes Go's
independent build-cache saving (145.1 → 3.7 s and 178.0 → 4.8 s).
Peak RSS falls **62.6%** and **51.8%**.

A comment appended to one emitted rule class gives **zero initial disk hits
on both shards**. OSG's `frontend-object-cache.mjs` includes every registry
object's source hash in its version namespace (`semanticInputs`), so this
content edit invalidates all previous entries. The later closure round
reuses work from the same invocation. This pin provides strong unchanged-run
reuse, with no partial cross-run reuse after the measured content edit.
Edited frontend times return near cold: **481.5 s** for statements and
**810.4 s** for rules. Cache footprint grows from **386.4 to 772.7 MiB**
and **1,174.3 to 2,349.1 MiB**, respectively; unchanged warm runs leave
the footprint unchanged.

An mtime-only touch would leave source bytes unchanged and would not exercise
this invalidation. Edited-run totals can still benefit from Go's build cache.

## Wall time and memory

All times in seconds. RSS is GiB.

| Shard | Condition | TS lowering | ABAP emission | gogen frontend | gogen emission | Go build | Execution | Harness total | Peak RSS |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap/statements` | cold | 57.9 | 23.3 | 488.0 | 52.6 | 145.1 | 287.2 | 1070.9 | 22.6 |
| `abap/statements` | warm | 58.5 | 23.4 | 119.5 | 50.0 | 3.7 | 287.7 | 558.1 | 8.4 |
| `abap/statements` | warm-after-edit | 58.9 | 23.5 | 481.5 | 53.4 | 2.5 | 287.6 | 924.3 | 23.3 |
| `rules` | cold | 123.3 | 34.0 | 851.9 | 57.8 | 178.0 | 276.9 | 1545.2 | 28.8 |
| `rules` | warm | 126.9 | 33.8 | 246.5 | 51.3 | 4.8 | 277.8 | 763.8 | 13.9 |
| `rules` | warm-after-edit | 125.5 | 34.1 | 810.4 | 57.4 | 3.2 | 277.8 | 1331.4 | 28.8 |

## Cache reuse and disk footprint

H/M = gogen `frontendCounts.reused` / `frontendCounts.lowered` per closure
round. Round 0 begins with an empty in-memory session, so its hits are disk
hits. Round 1 can reuse in-memory or disk entries; its hits must not be
reported as cross-run disk hits. “Reused classes” counts distinct names
reused in either round, including within-run reuse on cold and edited runs.
These are class-compilation counters, not file reads or whole-program hits.

| Shard | Condition | Round 0 H/M | Round 1 H/M | Reused classes | Cache MiB on disk |
| --- | --- | ---: | ---: | ---: | ---: |
| `abap/statements` | cold | 0 / 2,730 | 1,168 / 1,564 | 1,168 | 386.4 |
| `abap/statements` | warm | 2,724 / 6 | 2,728 / 4 | 2,728 | 386.4 |
| `abap/statements` | warm-after-edit | 0 / 2,730 | 1,168 / 1,564 | 1,168 | 772.7 |
| `rules` | cold | 0 / 4,331 | 1,158 / 3,175 | 1,158 | 1174.3 |
| `rules` | warm | 4,327 / 4 | 4,331 / 2 | 4,331 | 1174.3 |
| `rules` | warm-after-edit | 0 / 4,331 | 1,158 / 3,175 | 1,158 | 2349.1 |

## Result identity

Every condition has the same complete multiset of `(title, status)` and
`(title, status, message)` for its shard. Statements: **4,962 pass, 0 fail,
0 trap, 0 not-run**. Rules: **3,149 pass, 0 fail, 0 trap, 10 not-run**;
all ten are explicit upstream `it.skip` tests. No asynchronous pending
cases or initialization failures. No test verdict is cached.

The SHA-256 of compact JSON containing the sorted `(title, status)` pairs
(including duplicates) is identical across all three conditions:

| Shard | Title/status SHA-256 |
| --- | --- |
| `abap/statements` | `8f575909715661385438b0299cc9e96be087768913a0e427d2b423aaee9a6f14` |
| `rules` | `cdd6a95ec9ee434da5c0401bc84f1a6b80e950c5f1dea3bd917a3d8e3de12556` |

Generated `zz_generated.go` is also byte-identical across each shard's
three conditions. Its SHA-256 is `8ba2a5afd939c9177dc5f66d9c5efd1ec6447ebc1f41dda6a90c5f9ca7fb1ed6`
for statements and `ab9cdd8fe72dc4575847385e74a49daa8fb6d0ef5a9643cb4ef0d481da639d41`
for rules. This explains Go's own build-cache reuse independently of the
gogen frontend cache.

## Method

TS-HA@OSGO, one CLI invocation at a time, no competing heavy process.
AMD Ryzen 5 5600G; 12 visible logical CPUs, cgroup quota 8 CPUs and memory
limit 64 GiB. Node 26.9.0, Go 1.26.0. Prepared environment from
`.cache/train-o5/env.sh`: `GOMAXPROCS=4`, `GOGC=50`, `GOMEMLIMIT=24GiB`,
Node heap limit 32 GiB, `GOGEN_NOLINE=1`, fast Go build mode.
Prepared Go build/module caches remain enabled throughout; “cold” means
only a fresh, empty `ABAPITI_OSG_CACHE`, not cold Go or filesystem caches.
Each shard has its own cache directory. Warm uses that same directory.

The base bridge does not implement `ABAPITI_OSG_ONE_BUILD`. A local-only
bridge copy honors `ABAPITI_OSG_ONE_BUILD=0` by disabling whole-program
frontend/executable snapshot reads and writes, while retaining
`ABAPITI_OSG_CACHE/gogen-frontend`. Every measured run therefore invokes
`compileProgram`, emission, `go build` and fresh test execution. OSG remains
clean and unmodified. The only tracked changes are this report and the pin.

A Go build overlay times `LowerAbaplintTests` and
`EmitAbaplintTestsOSG` plus writing its files. The existing bridge timers
measure gogen frontend, Go emission, Go build and execution. Harness total
is elapsed wall time for the complete `abapiti abaplint-tests` invocation;
it includes evidence/gap generation, startup, admission and GC beyond the
listed phase times. Closure materialization and compiler setup are outside
that total. The same materialized closure is used for all three runs.

Peak RSS is Linux `wait4().ru_maxrss` for the CLI process tree: the maximum
single-process high-water mark, not simultaneous aggregate memory. GNU
`time` is unavailable. Disk usage is allocated bytes (`du -s -B1`), MiB =
2^20 bytes; RSS GiB = 2^30 bytes. One observation per condition, no confidence
interval or claim that execution/build timing changes come from frontend reuse.

Warm-after-edit appends `* cache measurement: trivial source edit` to the
emitted ABAP rule class corresponding to
`src/rules/unused_variables.ts.UnusedVariables`, before gogen reads it.
This is a content change, rather than an mtime-only touch. The next CLI run
regenerates ABAP from the unchanged TS closure. Tests and rule semantics
are unchanged.

## Reproduction

Run from the repository root. All scratch artifacts below remain local.
The following setup uses the prepared environment and a clean OSG worktree:

```sh
. .cache/train-o5/env.sh
git switch -c docs/osg-cache-cold-warm origin/main
git config user.name 'Alice V.'
git config user.email ooisee@gmail.com
git -C .cache/open-steamgate fetch origin c681c9f7ee888044051ea4e5e67c13a30ecbf5c8
git -C .cache/open-steamgate worktree add --detach "$PWD/.cache/osg-c681c9f7" \
  c681c9f7ee888044051ea4e5e67c13a30ecbf5c8
(cd .cache/osg-c681c9f7 && npm ci && node tools/osd-libs.mjs --sync)
printf '%s\n' c681c9f7ee888044051ea4e5e67c13a30ecbf5c8 > .github/ci/osgo.ref
mkdir -p .cache/osg-cache-measure
```

Create the local measurement adapters (no tracked source edits):

```sh
cat > .cache/osg-cache-measure/instrument.py <<'PY'
from pathlib import Path
import json,re
r=Path.cwd(); m=r/'.cache/osg-cache-measure'
replacements={}
for name in ['abaplint_tests.go','abaplint_tests_osg.go']:
    source=r/'cmd/abapiti'/name
    text=source.read_text().replace('import (','import (\n "time"')
    if name=='abaplint_tests.go':
        text=text.replace('lowering, err := tsfront.LowerAbaplintTests(args[0])',
            'phaseStart := time.Now()\n lowering, err := tsfront.LowerAbaplintTests(args[0])\n'
            ' fmt.Fprintf(cmd.ErrOrStderr(), "MEASURE tsLoweringMs=%d\\n", time.Since(phaseStart).Milliseconds())')
    else:
        text=text.replace('files, err := tsfront.EmitAbaplintTestsOSG(dir, lowering)',
            'phaseStart := time.Now()\n files, err := tsfront.EmitAbaplintTestsOSG(dir, lowering)')
        text=text.replace('osg, _ := cmd.Flags()',
            'fmt.Fprintf(cmd.ErrOrStderr(), "MEASURE abapEmissionMs=%d\\n", time.Since(phaseStart).Milliseconds())\n osg, _ := cmd.Flags()')
    target=m/name; target.write_text(text); replacements[str(source)]=str(target)
(m/'overlay.json').write_text(json.dumps({'Replace':replacements}))
text=(r/'tools/abaplint-tests-osg.mjs').read_text()
text=re.sub(r"from '\./([^']+)'",lambda v:"from '"+(r/'tools'/v[1]).as_uri()+"'",text)
text=text.replace("new URL('../.github/ci/osgo.ref',import.meta.url)",
                  "new URL('"+(r/'.github/ci/osgo.ref').as_uri()+"')")
text=text.replace("new URL('./", "new URL('"+(r/'tools').as_uri()+"/")
text=text.replace('if (cacheRoot) {',"if (cacheRoot && process.env.ABAPITI_OSG_ONE_BUILD!=='0') {")
text=text.replace('const front=cacheRoot ?',"const front=cacheRoot && process.env.ABAPITI_OSG_ONE_BUILD!=='0' ?")
text=text.replace('if(cacheRoot)writeSnapshot',"if(cacheRoot && process.env.ABAPITI_OSG_ONE_BUILD!=='0')writeSnapshot")
text=text.replace('const timingMs=',
    "if(process.env.CACHE_EDIT_FILE) { const file=join(input,process.env.CACHE_EDIT_FILE); "
    "writeFileSync(file,readFileSync(file,'utf8')+'\\n* cache measurement: trivial source edit\\n'); }\nconst timingMs=")
text=text.replace('registry=program.reg;',
    'registry=program.reg;\n   writeFileSync(join(out,`frontend-round-${round}.json`),JSON.stringify(program.frontendCounts,null,2));')
(m/'bridge.mjs').write_text(text)
PY
python3 .cache/osg-cache-measure/instrument.py
go build -overlay .cache/osg-cache-measure/overlay.json \
  -o .cache/osg-cache-measure/abapiti ./cmd/abapiti
```

Materialize once, before timing:

```sh
cat > .cache/osg-cache-measure/materialize.mjs <<'JS'
import {materializeRegistryClosure} from '../../tools/registry-closure.mjs';
import {discoverShards} from '../../tools/abaplint-tests-all.mjs';
const root=process.cwd(), upstream=`${root}/.cache/abaplint`;
for(const name of ['abap/statements','rules']) {
 const shard=discoverShards(upstream).shards.find(s=>s.name===name);
 materializeRegistryClosure(`${root}/.cache/osg-cache-measure/${shard.id}-closure`,upstream,{roots:shard.roots,runtime:`${root}/tsfront/testdata/abaplint-tests/runtime`});
}
JS
node .cache/osg-cache-measure/materialize.mjs
```

Use a fresh child for each peak-RSS observation:

```sh
cat > .cache/osg-cache-measure/time.py <<'PY'
import os,sys,time
args=sys.argv[2:]; out=sys.argv[1]
pid=os.fork()
if pid==0: os.execvpe(args[0],args,os.environ)
_,status,usage=os.wait4(pid,0)
with open(out,'w') as f:f.write('Maximum resident set size (kbytes): '+str(usage.ru_maxrss)+'\n')
sys.exit(os.waitstatus_to_exitcode(status))
PY
```

Run the six conditions serially; the two `*-cache` directories must not exist
before starting this script. It preserves raw results, round counters, logs,
phase times, RSS and cache sizes under the measurement directory:

```sh
cat > .cache/osg-cache-measure/measure.py <<'PY'
import subprocess, json, pathlib, time, os, re, hashlib
root=pathlib.Path.cwd(); base=root/'.cache/osg-cache-measure'
records=[]
env=dict(os.environ,ABAPITI_OSG_HOME=str(root/'.cache/osg-c681c9f7'),ABAPITI_OSG_ONE_BUILD='0')
for shard in ['abap-statements','rules']:
 cache=base/(shard+'-cache'); cache.mkdir()
 for kind in ['cold','warm','warm-after-edit']:
  label=shard+'-'+kind; out=base/label; runenv=dict(env,ABAPITI_OSG_CACHE=str(cache))
  if kind=='warm-after-edit':
   # Use a real emitted rule class, identified from the previous run's name map.
   names=json.loads((base/(shard+'-cold')/'abap/names.json').read_text())
   files=sorted((base/(shard+'-cold')/'abap').glob('*.clas.abap'))
   chosen=next((p for p in files if names.get(p.name.split('.')[0].upper(),{}).get('id','')=='src/rules/unused_variables.ts.UnusedVariables'),None)
   if chosen is None: raise RuntimeError('no emitted unused_variables rule class')
   runenv['CACHE_EDIT_FILE']=chosen.name
  command=[str(base/'abapiti'),'abaplint-tests',str(base/(shard+'-closure')),'-o',str(out),'--target','osg','--osg-home',env['ABAPITI_OSG_HOME'],'--osg-runner',str(base/'bridge.mjs')]
  print('START',label,time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime()),flush=True)
  start=time.monotonic()
  with (base/(label+'.stdout.json')).open('w') as stdout, (base/(label+'.stderr.log')).open('w') as stderr:
   proc=subprocess.run(['python3',str(base/'time.py'),str(base/(label+'.rss')),*command],env=runenv,stdout=stdout,stderr=stderr)
  elapsed=time.monotonic()-start
  record={'shard':shard,'kind':kind,'seconds':elapsed,'exit':proc.returncode,'cacheDiskBytes':int(subprocess.check_output(['du','-s','-B1',str(cache)]).split()[0]),'cacheApparentBytes':int(subprocess.check_output(['du','-sb',str(cache)]).split()[0])}
  record['phaseMs']={k:int(v) for k,v in re.findall(r'MEASURE (\w+)=(\d+)',(base/(label+'.stderr.log')).read_text())}
  record['rssKiB']=int(re.search(r'Maximum resident set size \(kbytes\):\s*(\d+)',(base/(label+'.rss')).read_text())[1])
  if 'CACHE_EDIT_FILE' in runenv: record['editedFile']=runenv['CACHE_EDIT_FILE']
  if (out/'osg/timing.json').exists():record['osg']=json.loads((out/'osg/timing.json').read_text())
  record['rounds']=[json.loads(p.read_text()) for p in sorted((out/'osg').glob('frontend-round-*.json'))]
  records.append(record);(base/'records.json').write_text(json.dumps(records,indent=2)+'\n')
  print('DONE',label,round(elapsed,3),'exit',proc.returncode,'rssKiB',record['rssKiB'],flush=True)
  if not (base/(label+'.stdout.json')).stat().st_size or proc.returncode not in [0,1]: raise RuntimeError('measurement failed')
PY
python3 .cache/osg-cache-measure/measure.py \
  > .cache/osg-cache-measure/progress.log 2>&1
```

Compare complete title/status/message multisets and generated Go bytes;
this also writes the digest and compact measurements used in the tables:

```sh
cat > .cache/osg-cache-measure/analyze.py <<'PY'
import json,hashlib,collections,pathlib
base=pathlib.Path(__file__).resolve().parent
records=json.loads((base/'records.json').read_text())
summary=[]
for row in records:
 label=row['shard']+'-'+row['kind']; result=json.loads((base/(label+'.stdout.json')).read_text())
 tuples=sorted((r['title'],r['status']) for r in result['results'])
 digest=hashlib.sha256(json.dumps(tuples,ensure_ascii=False,separators=(',',':')).encode()).hexdigest()
 full=sorted((r['title'],r['status'],r.get('message','')) for r in result['results'])
 cold=json.loads((base/(row['shard']+'-cold.stdout.json')).read_text())
 assert tuples==sorted((r['title'],r['status']) for r in cold['results'])
 assert full==sorted((r['title'],r['status'],r.get('message','')) for r in cold['results'])
 assert not any(r['status']=='failed' for r in result['results'])
 generated=base/label/'osg/go/cmd/abapitests/zz_generated.go'
 coldgo=base/(row['shard']+'-cold')/'osg/go/cmd/abapitests/zz_generated.go'
 assert generated.read_bytes()==coldgo.read_bytes()
 rounds=row['rounds']; reused=set().union(*(set(x['reusedClasses']) for x in rounds))
 item={'label':label,'total':round(row['seconds'],3),'phases':row['phaseMs']|row['osg']['timingMs'],
       'rounds':[(x['reused'],x['lowered']) for x in rounds], 'uniqueReused':len(reused),
       'cacheMiB':row['cacheDiskBytes']/2**20,'peakGiB':row['rssKiB']/2**20,
       'nodePeakGiB':row['osg']['nodePeakRSSKiB']/2**20,
       'counts':dict(collections.Counter(r['status'] for r in result['results'])),
       'titleStatusSha256':digest,'generatedGoSha256':hashlib.sha256(generated.read_bytes()).hexdigest()}
 summary.append(item)
(base/'analysis.json').write_text(json.dumps(summary,indent=2)+'\n')
for item in summary: print(json.dumps(item))
PY
python3 .cache/osg-cache-measure/analyze.py
```

## Control: no cache dir

Measured 2026-10-14 at stoker's request: two serial, fresh-process
`abap/statements` samples using TS-HA@OSGO, the same materialized closure,
instrumented CLI, local bridge, prepared environment and clean OSG
`c681c9f7` worktree as above. `ABAPITI_OSG_ONE_BUILD=0` remained set;
`ABAPITI_OSG_CACHE` was **unset**, rather than pointing at an empty directory.
No frontend cache directory was supplied or created by either control.
Each sample used a new output directory. Prepared Go build/module and
filesystem caches remained enabled, matching the earlier method; “cold”
here means no cross-run gogen frontend or whole-program snapshot reuse.
No other heavy workload ran during either sample; process checks showed
only the benchmark consuming substantial CPU. Sample starts were
15:51:22 UTC and 16:01:37 UTC, with the second starting after the first exited.

All times in seconds. Peak RSS uses the same `wait4` definition and is GiB.

| Shard | Condition | TS lowering | ABAP emission | gogen frontend | gogen emission | Go build | Execution | Harness total | Peak RSS |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap/statements` | no cache dir, cold sample 1 | 59.0 | 23.3 | 179.2 | 48.3 | 2.4 | 287.0 | 615.7 | 11.6 |
| `abap/statements` | no cache dir, cold sample 2 | 59.2 | 23.5 | 184.1 | 48.6 | 2.4 | 286.5 | 619.6 | 10.4 |

**The frontend is ~184 s, not ~488 s.** The two-sample mean is **181.6 s**,
compared with **488.0 s** for the earlier cold run with cache recording
enabled. This control supports cache-recording-path overhead as the cause
of the earlier frontend slowdown, rather than a persistent host/regression
slowdown. It does not isolate serialization, hashing or disk I/O costs
within that path. Go's independent build-cache reuse explains the short
Go builds and must not be attributed to disabling the frontend cache.

Both controls exited 0 with **4,962 pass, 0 fail, 0 trap, 0 not-run**.
Complete `(title, status, message)` multisets and generated Go bytes match
the original statements cold run exactly, retaining the title/status and
generated-Go SHA-256 digests listed above. Both controls report round 0
H/M **0 / 2,730**, round 1 H/M **1,146 / 1,586**; the latter is within-run
in-memory reuse, with no disk hits. Bridge snapshot counters report
`frontend: disabled` and `binary: disabled`. Its `gogenFrontend: unsupported`
label also covers the no-`cacheDir` case in this instrument; it does not
indicate that this OSG pin lacks cache support.

Raw evidence remains local under `.cache/osg-cache-measure/`:
`abap-statements-control-no-cache-{1,2}` output directories, matching
`.stdout.json`, `.stderr.log` and `.rss` files, plus `control-records.json`,
`control-analysis.json` and `control-progress.log`. The local `control.py`
adapter reuses `time.py`, `abapiti` and `bridge.mjs`, removes
`ABAPITI_OSG_CACHE` and `CACHE_EDIT_FILE` from each child environment,
asserts fresh output directories, and runs samples 1 and 2 sequentially.
Reproduce with fresh output names using:

```sh
. .cache/train-o5/env.sh
unset ABAPITI_OSG_CACHE CACHE_EDIT_FILE
python3 .cache/osg-cache-measure/control.py \
  > .cache/osg-cache-measure/control-progress.log 2>&1
python3 .cache/osg-cache-measure/control-analyze.py
```

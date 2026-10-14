# TS-HG@Go versus TS-HA@OSGO

Measured on 2026-10-14 at ABAPITI `338a5385dae4cef5d6124fe3f8f7ae96335de320`, OSG
`f83dc5c3e05bfaea71268fd6ad0e9ff88532cce6` (the base's `.github/ci/osgo.ref`),
and upstream abaplint `577f875ebec44cfaf64841cfe71c8ab8dc32622e`.
Node 26.9.0; Go 1.26.0, linux/amd64; TypeScript 6.0.3 for the independent
oracle. The OSG checkout has transpiler 2.14.3 and parser 2.120.73; the
prepared `TRANSPILER` checkout is `ee5c2cd1a45aaac6be503a46fc903ce95a946877`.
One AMD Ryzen 5 5600G machine, 6 cores / 12 logical CPUs, cgroup CPU quota
8 CPUs and memory limit 64 GiB. No machine identifier is included.

Each shard/target has two accepted samples, run sequentially. The median
of two samples is their arithmetic mean. Ratios divide the OSG median
by the Go median. Per-test time divides execution by **passed tests**.
Pass counts refer to the complete binary workload, including borrowed tests
(syntax executes six borrowed passing cases); Node-pending tests remain not-run.
Execution includes registration and every executed case; execution/pass is
an amortized measure of the complete shard workload.

The same external `meter` executable uses `CLOCK_MONOTONIC` and `wait4`
for both targets' harness totals, generated-module Go builds, and binary
execution. The Go executable is rerun alone after its harness finishes;
its complete raw results must match the harness binary output before ownership
filtering. OSG execution is the sum of
metered binary invocations inside `resumeRunner`, including any restarts.
This excludes driver-side JSON/checkpoint processing; the broader
`osgEvidence.host.timingMs.run` is preserved in the raw samples below.
The OSG gogen frontend/emission phases come from its retained evidence.
A Go compiler overlay, stored only under `.cache`, times the original
`LowerAbaplintTests` and target emitter calls with monotonic `time.Now` /
`time.Since`. No tracked product source was edited.

All runs source `.cache/train-o5/env.sh`: `GOMAXPROCS=4`, `GOGC=50`,
`GOMEMLIMIT=24GiB`, Node heap limit 32768 MiB, `GOGEN_NOLINE=1`, and
`GOFLAGS="-buildvcs=false -gcflags=all=-dwarf=false '-ldflags=-s -w'"`.
The scheduler additionally supplies `-p=2 -buildvcs=false`. Both targets
reuse the prepared Go/npm caches. OSG uses `ABAPITI_OSG_ONE_BUILD=0`,
no `ABAPITI_OSG_CACHE`, and its default `fast` build flags:
`go build -p 1 -trimpath -ldflags='-s -w'
-gcflags='osg/gogen/cmd/abapitests=-N -l -dwarf=false'`.
Thus OSG frontend/binary snapshots are cold; Go's ordinary compiler cache
is retained, as requested. The emitted Go target uses its normal build flags.
OSG fast mode disables optimization and inlining for the generated package;
the Go target uses normal optimization. Execution ratios apply to these flags.

## Measurements

All phase and total times are seconds; execution/pass is milliseconds.
Generation total = target emission + gogen frontend + gogen emission.
Build/test RSS shows GiB / MiB respectively. These are medians of
per-run peak RSS; OSG's run-level test peak is the maximum across its
binary invocations. `wait4` build RSS measures the largest process in the
waited build tree, not the sum of simultaneous processes.

| Shard | Target | Pass / fail / trap / not-run | TS frontend/lowering | Go/ABAP emission | Gogen frontend + emission | Generation total | Go build | Execution only | ms/pass | Harness total | Build/test peak RSS (GiB/MiB) | OSG/Go execution | OSG/Go total |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap/lexer` | `go` | 200 / 0 / 0 / 0 | 49.487 | 4.935 | — | 4.935 | 36.699 | 0.018719 | 0.093593 | 103.553 | 3.076 / 19.82 | — | — |
| `abap/lexer` | `osg` | 200 / 0 / 0 / 0 | 50.062 | 21.008 | 145.760 + 18.157 | 184.924 | 116.061 | 0.054664 | 0.273319 | 376.861 | 8.719 / 83.77 | 2.92× | 3.64× |
| `abap/statements` | `go` | 4962 / 0 / 0 / 0 | 59.053 | 5.697 | — | 5.697 | 42.304 | 48.124327 | 9.698575 | 178.156 | 3.431 / 109.39 | — | — |
| `abap/statements` | `osg` | 4962 / 0 / 0 / 0 | 59.111 | 23.194 | 183.570 + 49.207 | 255.971 | 148.380 | 291.612287 | 58.769103 | 793.752 | 11.064 / 182.97 | 6.06× | 4.46× |
| `abap/syntax` | `go` | 1369 / 0 / 0 / 10 | 93.530 | 6.335 | — | 6.335 | 50.468 | 22.202352 | 16.217934 | 190.766 | 4.222 / 66.35 | — | — |
| `abap/syntax` | `osg` | 1369 / 0 / 0 / 10 | 93.718 | 33.434 | 238.300 + 28.879 | 300.613 | 159.658 | 116.483064 | 85.086241 | 709.002 | 11.861 / 224.51 | 5.25× | 3.72× |
| `rules` | `go` | 3149 / 0 / 0 / 10 | 126.001 | 7.070 | — | 7.070 | 57.348 | 53.151631 | 16.878892 | 267.434 | 4.758 / 153.86 | — | — |
| `rules` | `osg` | 3149 / 0 / 0 / 10 | 123.595 | 33.620 | 282.678 + 50.773 | 367.070 | 179.626 | 277.890846 | 88.247331 | 993.136 | 13.462 / 342.90 | 5.23× | 3.71× |

All title/status multisets match across targets. Each retained Node oracle is
covered exactly, including documented borrowed-title exclusions in the owned
reports. Full `(title, status, message)` results are identical between each
target’s repeats. All sixteen samples were rerun at the certified base, one
command at a time; no earlier-base measurements are included.

Harness totals include oracle compilation/execution, ABAPITI compiler build, closure preparation, evidence/file writes, OSG gap measurement, admission waits, release, and reporting. They need not equal the listed phase sums. The Go isolated rerun is outside its harness total. No OSG cache hits occurred.

## Raw accepted samples

Each line lists seconds and run-level peak RSS. `hostRun` is the broader OSG harness execution phase, while `execution` is binary-process wall time used in the table and ratios. Local evidence is retained under `.cache/speed-338a538`; no scratch identifiers or absolute temporary paths are committed.

```json
{"shard":"abap/lexer","target":"go","repeat":1,"lower":49.549723714,"emit":4.928241104,"gogenFront":0.0,"gogenEmit":0.0,"codegen":4.928241104,"build":36.613626893,"execution":0.019005349,"hostRun":0.0,"total":103.529279023,"passed":200,"failed":0,"trapped":0,"registered":200,"notRun":0,"asyncPending":0,"processes":1,"harnessExit":0,"buildPeakRSSKiB":3237180,"testPeakRSSKiB":20040}
{"shard":"abap/lexer","target":"go","repeat":2,"lower":49.424013593,"emit":4.9419378,"gogenFront":0.0,"gogenEmit":0.0,"codegen":4.9419378,"build":36.783810997,"execution":0.018432046,"hostRun":0.0,"total":103.576373595,"passed":200,"failed":0,"trapped":0,"registered":200,"notRun":0,"asyncPending":0,"processes":1,"harnessExit":0,"buildPeakRSSKiB":3212924,"testPeakRSSKiB":20552}
{"shard":"abap/lexer","target":"osg","repeat":1,"lower":50.200947614,"emit":20.940630538,"gogenFront":150.814,"gogenEmit":18.558,"codegen":190.312630538,"build":115.804177829,"execution":0.054926404,"hostRun":0.098,"total":382.091157049,"passed":200,"failed":0,"trapped":0,"registered":200,"notRun":0,"asyncPending":0,"processes":1,"harnessExit":0,"buildPeakRSSKiB":9287272,"testPeakRSSKiB":85828}
{"shard":"abap/lexer","target":"osg","repeat":2,"lower":49.922192425,"emit":21.074424036,"gogenFront":140.705,"gogenEmit":17.755,"codegen":179.534424036,"build":116.318239433,"execution":0.054401247,"hostRun":0.098,"total":371.630375738,"passed":200,"failed":0,"trapped":0,"registered":200,"notRun":0,"asyncPending":0,"processes":1,"harnessExit":0,"buildPeakRSSKiB":8998380,"testPeakRSSKiB":85732}
{"shard":"abap/statements","target":"go","repeat":1,"lower":59.170546173,"emit":5.702658244,"gogenFront":0.0,"gogenEmit":0.0,"codegen":5.702658244,"build":42.395616647,"execution":48.19800214,"hostRun":0.0,"total":178.837007204,"passed":4962,"failed":0,"trapped":0,"registered":4962,"notRun":0,"asyncPending":0,"processes":1,"harnessExit":0,"buildPeakRSSKiB":3598608,"testPeakRSSKiB":112836}
{"shard":"abap/statements","target":"go","repeat":2,"lower":58.935009062,"emit":5.691736344,"gogenFront":0.0,"gogenEmit":0.0,"codegen":5.691736344,"build":42.212026277,"execution":48.050652241,"hostRun":0.0,"total":177.474193283,"passed":4962,"failed":0,"trapped":0,"registered":4962,"notRun":0,"asyncPending":0,"processes":1,"harnessExit":0,"buildPeakRSSKiB":3596496,"testPeakRSSKiB":111196}
{"shard":"abap/statements","target":"osg","repeat":1,"lower":59.132360269,"emit":23.1144246,"gogenFront":181.431,"gogenEmit":49.188,"codegen":253.7334246,"build":148.761625547,"execution":293.039949533,"hostRun":293.121,"total":792.945031047,"passed":4962,"failed":0,"trapped":0,"registered":4962,"notRun":0,"asyncPending":0,"processes":1,"harnessExit":0,"buildPeakRSSKiB":11635716,"testPeakRSSKiB":187388}
{"shard":"abap/statements","target":"osg","repeat":2,"lower":59.089071108,"emit":23.273292757,"gogenFront":185.709,"gogenEmit":49.226,"codegen":258.208292757,"build":147.998561933,"execution":290.184623983,"hostRun":290.264,"total":794.558529473,"passed":4962,"failed":0,"trapped":0,"registered":4962,"notRun":0,"asyncPending":0,"processes":1,"harnessExit":0,"buildPeakRSSKiB":11567548,"testPeakRSSKiB":187332}
{"shard":"abap/syntax","target":"go","repeat":1,"lower":92.681701753,"emit":6.325903013,"gogenFront":0.0,"gogenEmit":0.0,"codegen":6.325903013,"build":50.474932081,"execution":22.104201099,"hostRun":0.0,"total":189.833105244,"passed":1369,"failed":0,"trapped":0,"registered":1379,"notRun":10,"asyncPending":0,"processes":1,"harnessExit":0,"buildPeakRSSKiB":4444720,"testPeakRSSKiB":68764}
{"shard":"abap/syntax","target":"go","repeat":2,"lower":94.377343609,"emit":6.343992528,"gogenFront":0.0,"gogenEmit":0.0,"codegen":6.343992528,"build":50.460576485,"execution":22.300502146,"hostRun":0.0,"total":191.699300161,"passed":1369,"failed":0,"trapped":0,"registered":1379,"notRun":10,"asyncPending":0,"processes":1,"harnessExit":0,"buildPeakRSSKiB":4409800,"testPeakRSSKiB":67120}
{"shard":"abap/syntax","target":"osg","repeat":1,"lower":93.309244484,"emit":33.422570904,"gogenFront":236.252,"gogenEmit":28.867,"codegen":298.541570904,"build":158.9632611,"execution":116.137424541,"hostRun":116.2,"total":705.462679355,"passed":1369,"failed":0,"trapped":0,"registered":1379,"notRun":10,"asyncPending":0,"processes":1,"harnessExit":0,"buildPeakRSSKiB":12382348,"testPeakRSSKiB":229720}
{"shard":"abap/syntax","target":"osg","repeat":2,"lower":94.12760237,"emit":33.445254475,"gogenFront":240.349,"gogenEmit":28.89,"codegen":302.684254475,"build":160.35193899,"execution":116.828702675,"hostRun":116.891,"total":712.54060178,"passed":1369,"failed":0,"trapped":0,"registered":1379,"notRun":10,"asyncPending":0,"processes":1,"harnessExit":0,"buildPeakRSSKiB":12491292,"testPeakRSSKiB":230072}
{"shard":"rules","target":"go","repeat":1,"lower":127.844692812,"emit":7.075572031,"gogenFront":0.0,"gogenEmit":0.0,"codegen":7.075572031,"build":57.581573317,"execution":53.36549183,"hostRun":0.0,"total":269.658558621,"passed":3149,"failed":0,"trapped":0,"registered":3159,"notRun":10,"asyncPending":0,"processes":1,"harnessExit":0,"buildPeakRSSKiB":4998596,"testPeakRSSKiB":156808}
{"shard":"rules","target":"go","repeat":2,"lower":124.156404895,"emit":7.065322704,"gogenFront":0.0,"gogenEmit":0.0,"codegen":7.065322704,"build":57.11473177,"execution":52.937769317,"hostRun":0.0,"total":265.208746125,"passed":3149,"failed":0,"trapped":0,"registered":3159,"notRun":10,"asyncPending":0,"processes":1,"harnessExit":0,"buildPeakRSSKiB":4979476,"testPeakRSSKiB":158288}
{"shard":"rules","target":"osg","repeat":1,"lower":123.484962905,"emit":33.522539889,"gogenFront":285.7,"gogenEmit":49.865,"codegen":369.087539889,"build":178.956877528,"execution":277.656993021,"hostRun":277.719,"total":993.507658761,"passed":3149,"failed":0,"trapped":0,"registered":3159,"notRun":10,"asyncPending":0,"processes":1,"harnessExit":0,"buildPeakRSSKiB":14236828,"testPeakRSSKiB":352540}
{"shard":"rules","target":"osg","repeat":2,"lower":123.704564721,"emit":33.717128511,"gogenFront":279.655,"gogenEmit":51.68,"codegen":365.052128511,"build":180.295509372,"execution":278.124698159,"hostRun":278.191,"total":992.763345831,"passed":3149,"failed":0,"trapped":0,"registered":3159,"notRun":10,"asyncPending":0,"processes":1,"harnessExit":0,"buildPeakRSSKiB":13995464,"testPeakRSSKiB":349724}
```

## Exact commands

Run from `/work` with the prepared caches. This recreates the worktree at the measured integration tip; an existing worktree must first be removed with `git worktree remove /work/.speed`. Instrument files live under `.cache`.

```sh
git fetch origin
git worktree add -f --detach /work/.speed origin/integ/shinkansen
cd /work/.speed
test "$(git rev-parse HEAD)" = 338a5385dae4cef5d6124fe3f8f7ae96335de320
. /work/.cache/train-o5/env.sh
test "$(cat .github/ci/osgo.ref)" = "$(git -C /work/.cache/osg-f83dc5c3 rev-parse HEAD)"
mkdir -p /work/.cache/speed-338a538/bin
cat > /work/.cache/speed-338a538/meter.c <<'SPEED_EOF'
#define _GNU_SOURCE
#include <sys/wait.h>
#include <sys/resource.h>
#include <time.h>
#include <unistd.h>
#include <stdio.h>
#include <stdlib.h>
#include <errno.h>
int main(int argc,char **argv) {
 if(argc<3)return 125;
 struct timespec a,b; struct rusage r; int status;
 clock_gettime(CLOCK_MONOTONIC,&a);
 pid_t p=fork(); if(p==0){execvp(argv[2],argv+2);perror("execvp");_exit(127);} if(p<0)return 125;
 while(wait4(p,&status,0,&r)<0){if(errno!=EINTR)return 125;}
 clock_gettime(CLOCK_MONOTONIC,&b);
 double s=(b.tv_sec-a.tv_sec)+(b.tv_nsec-a.tv_nsec)/1e9;
 const char *log=getenv("SPEED_LOG"); FILE *f=log?fopen(log,"a"):NULL;
 if(f){fprintf(f,"{\"kind\":\"%s\",\"seconds\":%.9f,\"rssKiB\":%ld,\"status\":%d}\n",argv[1],s,r.ru_maxrss,status);fclose(f);}
 if(WIFSIGNALED(status))return 128+WTERMSIG(status);
 return WEXITSTATUS(status);
}
SPEED_EOF
cat > /work/.cache/speed-338a538/prepare.py <<'SPEED_EOF'
import pathlib,json
root=pathlib.Path('/work/.speed'); m=pathlib.Path('/work/.cache/speed-338a538')
replace={}
for rel in ['cmd/abapiti/abaplint_tests.go','cmd/abapiti/abaplint_tests_osg.go']:
 s=(root/rel).read_text(); s=s.replace('import (','import (\n "time"',1)
 if rel.endswith('abaplint_tests.go'):
  s=s.replace('"os/exec"','"os/exec"\n "os"')
  s=s.replace('lowering, err := tsfront.LowerAbaplintTests(args[0])','speedStart := time.Now()\n lowering, err := tsfront.LowerAbaplintTests(args[0])\n speedPhase("tsLower", speedStart)')
  s=s.replace('files, err := tsfront.EmitAbaplintTests(args[0], lowering)','speedStart = time.Now()\n files, err := tsfront.EmitAbaplintTests(args[0], lowering)\n speedPhase("goEmit", speedStart)')
  s+='''\nfunc speedPhase(kind string, start time.Time) {\n elapsed := time.Since(start).Seconds()\n if f, err := os.OpenFile(os.Getenv("SPEED_LOG"), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600); err == nil {\n fmt.Fprintf(f, "{\\"kind\\":\\"%s\\",\\"seconds\\":%.9f}\\n",kind,elapsed); f.Close()\n }\n}\n'''
 else:
  s=s.replace('files, err := tsfront.EmitAbaplintTestsOSG(dir, lowering)','speedStart := time.Now()\n files, err := tsfront.EmitAbaplintTestsOSG(dir, lowering)\n speedPhase("abapEmit", speedStart)')
 dest=m/pathlib.Path(rel).name; dest.write_text(s);replace[str(root/rel)]=str(dest)
(m/'overlay.json').write_text(json.dumps({'Replace':replace}))
SPEED_EOF
cat > /work/.cache/speed-338a538/bin/go <<'SPEED_EOF'
#!/bin/sh
m=/work/.cache/speed-338a538
if [ "$1" = build ]; then
 case "$*" in
  *./cmd/abapiti*) shift; exec "$m/meter" compilerBuild /usr/local/go/bin/go build -overlay="$m/overlay.json" "$@" ;;
  *) exec "$m/meter" goBuild /usr/local/go/bin/go "$@" ;;
 esac
fi
exec /usr/local/go/bin/go "$@"
SPEED_EOF
cat > /work/.cache/speed-338a538/preload.mjs <<'SPEED_EOF'
import cp from 'node:child_process';
import {syncBuiltinESMExports} from 'node:module';
import {promisify} from 'node:util';
import {writeFileSync} from 'node:fs';
const original=cp.spawnSync;
cp.spawnSync=function(command,args,options){
 if(String(command).endsWith('/abaplint-tests-osg')) return original('/work/.cache/speed-338a538/meter',['osgTest',command,...args],options);
 return original(command,args,options);
};
const originalExec=cp.execFile, originalAsync=promisify(originalExec);
function capture(command,stdout){
 if(String(command).endsWith('/abapiti') && process.env.SPEED_RAW && stdout) writeFileSync(process.env.SPEED_RAW,stdout);
}
cp.execFile=function(...args){return originalExec(...args);};
cp.execFile[promisify.custom]=(...args)=>originalAsync(...args).then(result=>{
 capture(args[0],result.stdout);return result;
},error=>{capture(args[0],error.stdout);throw error;});
syncBuiltinESMExports();
SPEED_EOF
cat > /work/.cache/speed-338a538/run.py <<'SPEED_EOF'
import os,pathlib,subprocess,json,re,time
m=pathlib.Path('/work/.cache/speed-338a538');root=pathlib.Path('/work/.speed')
base=os.environ.copy()
base.update(ABAPITI_OSG_HOME='/work/.cache/osg-f83dc5c3',ABAPITI_TESTS_WORKERS='1',ABAPITI_TESTS_KEEP_ARTIFACTS='1',ABAPITI_OSG_ONE_BUILD='0',ABAPITI_TESTS_SCRATCH=str(m/'scratch'))
for k in ['ABAPITI_OSG_CACHE','ABAPITI_OSG_BUILD_ONLY','ABAPITI_TESTS_COMPILER','ABAPITI_TESTS_COMPILER_COMMIT','ABAPITI_TESTS_FILES','ABAPITI_TESTS_FIRST','ABAPITI_OSG_LOCK','ABAPITI_OSG_MEMORY_MAX','ABAPITI_ASSUME_INT']:base.pop(k,None)
base['PATH']=str(m/'bin')+':'+base['PATH']
base['NODE_OPTIONS']=base.get('NODE_OPTIONS','')+' --import='+str(m/'preload.mjs')
rows=[]
for shard in ['abap/lexer','abap/statements','abap/syntax','rules']:
 for repeat in [1,2]:
  for target in ['go','osg']:
   label=shard.replace('/','-')+'-'+target+'-'+str(repeat); d=m/label;d.mkdir(exist_ok=True)
   env=base|dict(ABAPITI_TESTS_SHARDS=shard,ABAPITI_TESTS_TARGET=target,SPEED_LOG=str(d/'meter.ndjson'),SPEED_RAW=str(d/'raw.json'))
   if (d/'row.json').exists():
    rows.append(json.loads((d/'row.json').read_text()));continue
   if (d/'meter.ndjson').exists(): raise RuntimeError('Incomplete sample must be archived before rerun: '+label)
   print('START',label,time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime()),flush=True)
   with (d/'stdout.log').open('w') as out,(d/'stderr.log').open('w') as err:
    p=subprocess.run([str(m/'meter'),'total','node','tools/abaplint-tests-all.mjs','/work/.cache/abaplint',str(d/'summary.json')],cwd=root,env=env,stdout=out,stderr=err)
   log=(d/'stderr.log').read_text(); found=re.search(r'all-shard evidence retained: (.+)',log)
   if not found or not (d/'raw.json').exists():
    print('FAILED',label,p.returncode,log[-6000:],flush=True);raise SystemExit(1)
   scratch=pathlib.Path(found.group(1));id=shard.replace('/','-')
   raw=json.loads((d/'raw.json').read_text())
   report=json.loads((scratch/('go-'+id+'.json')).read_text()) if (scratch/('go-'+id+'.json')).exists() else raw
   aborted=p.returncode!=0
   if aborted and not raw['failed']:raise RuntimeError('Non-assertion harness failure')
   evidence=None
   if target=='osg':
    evidence=report.get('osgEvidence') or dict(host=json.loads((scratch/(id+'-out')/'osg/timing.json').read_text()))
   if target=='go':
    go=scratch/(id+'-out')/'go';binary=go/'abaplint-tests-go'
    with (d/'execution.json').open('w') as out,(d/'execution.stderr').open('w') as err:
     p=subprocess.run([str(m/'meter'),'goTest',str(binary)],cwd=go,env=env,stdout=out,stderr=err)
    rerun=json.loads((d/'execution.json').read_text())
    if rerun['results']!=raw['results']:raise RuntimeError('Go isolated execution changed results')
   readings=[json.loads(line) for line in (d/'meter.ndjson').read_text().splitlines()]
   row=dict(shard=shard,target=target,repeat=repeat,scratch=str(scratch),readings=readings,
    summary=json.loads((d/'summary.json').read_text()) if (d/'summary.json').exists() else {'shards':[{'name':shard,'registered':len(raw['results']),**{k:raw[k] for k in ['passed','failed','trapped','notRun','asyncPending']}}]},
    osgEvidence=evidence,passed=raw['passed'],registered=len(raw['results']),harnessExit=int(aborted),rawFile=str(d/'raw.json'))
   (d/'row.json').write_text(json.dumps(row,indent=2))
   rows.append(row);(m/'rows.json').write_text(json.dumps(rows,indent=2))
   print('DONE',label,'total',next(x['seconds'] for x in readings if x['kind']=='total'),'passed',row['passed'],'registered',row['registered'],flush=True)
SPEED_EOF
cc -O2 -o /work/.cache/speed-338a538/meter /work/.cache/speed-338a538/meter.c
python3 /work/.cache/speed-338a538/prepare.py
chmod +x /work/.cache/speed-338a538/bin/go
/usr/local/go/bin/go build -overlay=/work/.cache/speed-338a538/overlay.json \
  -o /work/.cache/speed-338a538/compiler ./cmd/abapiti
python3 /work/.cache/speed-338a538/run.py
# Resume with the same environment and run.py; complete rows are skipped.
# Archive an incomplete sample directory before rerunning it.
```

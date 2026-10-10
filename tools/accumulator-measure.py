#!/usr/bin/env python3
"""Compare already generated off/on Go targets, kit output, GC and array sites.

Usage: accumulator-measure.py OUTPUT_ROOT KIT CORE
OUTPUT_ROOT contains off/go and on/go from abapiti abaplint --target go.
Run under flock /tmp/abapiti-heavy.lock.
"""
import json,re,statistics,subprocess,pathlib,csv,argparse
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('output_root');parser.add_argument('kit');parser.add_argument('core')
parser.add_argument('--reuse-profiles',action='store_true',help='reuse existing allocation certificates; rerun interleaved timings')
args=parser.parse_args()
work,kit,core=(pathlib.Path(p).resolve() for p in [args.output_root,args.kit,args.core])
rows=[]
def run(args,**kw):
 p=subprocess.run(args,text=True,capture_output=True,**kw)
 if p.returncode: raise RuntimeError(str(args)+'\n'+p.stderr[-8000:])
 return p

def gc_share(binary,path):
 raw=run(['go','tool','pprof','-raw',str(binary),str(path)]).stdout
 samples,locations=raw.split('Locations\n',1);gc=set();cur=None
 for line in locations.split('Mappings\n')[0].splitlines():
  m=re.match(r'\s*(\d+):',line)
  if m:cur=int(m[1])
  if re.search(r'runtime\.(gcBgMarkWorker|gcAssistAlloc|gcStart|gcMarkDone|gcMarkTermination|sweepone|bgsweep|bgscavenge|scavenge)',line):gc.add(cur)
 total=collected=0
 for line in samples.splitlines():
  m=re.match(r'\s*\d+\s+(\d+):\s+([\d ]+)',line)
  if m:
   n=int(m[1]);total+=n
   if any(int(v) in gc for v in m[2].split()):collected+=n
 if not total:raise RuntimeError('empty profile')
 return collected/total
def timed(mode,variant,repeat):
 directory=work/mode/'go';binary=directory/'zabaplint-go'
 input='zabapgit_standalone.prog.abap' if variant=='clean' else 'seeded/zabapgit_standalone.prog.abap'
 prefix=work/f'{mode}-{variant}-{repeat+1}';profile=prefix.with_suffix('.cpu')
 p=run([str(binary),'--file',input,'--config','abaplint.json','--deps','deps.txt','--metrics','--cpu-profile',str(profile)],cwd=kit)
 prefix.with_suffix('.stdout').write_text(p.stdout);prefix.with_suffix('.stderr').write_text(p.stderr)
 expected=(kit/f'expected-{variant}.txt').read_bytes()
 if p.stdout.encode()!=expected:raise RuntimeError(f'{mode} {variant}: output mismatch')
 metric=next(json.loads(line) for line in p.stderr.splitlines() if line.startswith('{'))
 row={'flag':mode,'input':variant,'repeat':repeat+1,**metric,'gc_sample_share':gc_share(binary,profile)}
 rows.append(row);print(json.dumps(row),flush=True)
# Keep profiling builds outside the timed comparisons. Alternate pair order
# to reduce order/temperature drift while retaining exactly three samples.
for variant in ['clean','seeded']:
 for repeat in range(3):
  for mode in (['off','on'] if repeat%2==0 else ['on','off']):timed(mode,variant,repeat)
if not args.reuse_profiles:
 for mode in ['off','on']:
  directory=work/mode/'go'
  run(['go','build','-tags','profile_sites','-o','zabaplint-go-profile','.'],cwd=directory)
  for variant in ['clean','seeded']:
   input='zabapgit_standalone.prog.abap' if variant=='clean' else 'seeded/zabapgit_standalone.prog.abap'
   p=run([str(directory/'zabaplint-go-profile'),'--file',input,'--config','abaplint.json','--deps','deps.txt','--profile-sites',str(work/f'{mode}-{variant}-sites.json')],cwd=kit)
   if p.stdout.encode()!=(kit/f'expected-{variant}.txt').read_bytes():raise RuntimeError('profiled output mismatch')
(work/'timings.json').write_text(json.dumps(rows,indent=2)+'\n')
with (work/'timings.csv').open('w') as f:
 w=csv.writer(f);w.writerow(['flag','input','median_check_seconds','median_gc_sample_share'])
 for mode in ['off','on']:
  for variant in ['clean','seeded']:
   group=[r for r in rows if r['flag']==mode and r['input']==variant]
   w.writerow([mode,variant,statistics.median(r['Seconds'] for r in group),statistics.median(r['gc_sample_share'] for r in group)])
for variant in ['clean','seeded']:
 before=json.loads((work/f'off-{variant}-sites.json').read_text());after=json.loads((work/f'on-{variant}-sites.json').read_text())
 if before['schema']!='site-profile/1' or after['schema']!='site-profile/1' or before['input_sha256']!=after['input_sha256']:raise RuntimeError('certificate input/schema mismatch')
 off=before['sites'];on=after['sites']
 alloc=[]
 for site in sorted(off.keys()|on.keys()):
  if ('combi.ts' in site or '_combi.ts' in site) and '|new|' in site:
   before=off.get(site,{}).get('allocations',0);after=on.get(site,{}).get('allocations',0)
   alloc.append([site,before,after,after-before])
 with (work/f'allocations-{variant}.csv').open('w') as f:
  w=csv.writer(f);w.writerow(['site','off','on','delta']);w.writerows(alloc)
 print(variant,'combinator new',sum(r[1] for r in alloc),sum(r[2] for r in alloc),flush=True)
# Separate arrays from class allocations using each original TypeScript site.
site_map={s['site_id']:s for s in json.loads((work/'off/sites.json').read_text())['sites']}
def array_site(site):
 meta=site_map.get(site,{})
 source=meta.get('source','')
 m=re.match(r'(.*):(\d+):(\d+)$',source)
 if not m:return False
 path,line,col=m.groups();path=path[path.find('src/'):] if 'src/' in path else path
 file=core/path
 if not file.exists():return False
 text=file.read_text().splitlines()[int(line)-1]
 # Source columns are UTF-16. Array literal allocations start at '['.
 tail=text.encode('utf-16-le')[2*(int(col)-1):].decode('utf-16-le')
 return tail.startswith('[') or tail.startswith('new Array')
for variant in ['clean','seeded']:
 before=json.loads((work/f'off-{variant}-sites.json').read_text());after=json.loads((work/f'on-{variant}-sites.json').read_text())
 if before['schema']!='site-profile/1' or after['schema']!='site-profile/1' or before['input_sha256']!=after['input_sha256']:raise RuntimeError('certificate input/schema mismatch')
 off=before['sites'];on=after['sites']
 detail=[];groups={}
 for site in sorted(off.keys()|on.keys()):
  if ('combi.ts' in site or '_combi.ts' in site) and '|new|' in site and array_site(site):
   before=off.get(site,{}).get('allocations',0);after=on.get(site,{}).get('allocations',0)
   meta=site_map[site];method=meta['method'];detail.append([site,meta['source'],method,before,after,after-before])
   total=groups.setdefault(method,[0,0]);total[0]+=before;total[1]+=after
 with (work/f'array-allocations-{variant}.csv').open('w') as f:
  w=csv.writer(f);w.writerow(['site','source','method','off','on','delta']);w.writerows(detail)
 with (work/f'array-methods-{variant}.csv').open('w') as f:
  w=csv.writer(f);w.writerow(['method','off','on','delta']);w.writerows([method,*counts,counts[1]-counts[0]] for method,counts in sorted(groups.items()))
 print(variant,'combinator arrays',sum(r[3] for r in detail),sum(r[4] for r in detail),flush=True)

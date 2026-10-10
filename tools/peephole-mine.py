#!/usr/bin/env python3
"""Analysis only. Consume stmt-patterns -peephole; never rewrite ABAP/HIR."""
import argparse
import collections
import csv
import hashlib
import gzip
import json
from pathlib import Path
import re

TEMP = re.compile(r"\bt\d+\b", re.I)
ASSIGN = re.compile(r"^(?:DATA\(\s*(t\d+)\s*\)|(t\d+))\s*(=|\?=)\s*(.*)\.$", re.I)
DECL = re.compile(r"^DATA (t\d+) TYPE (.*)\.$", re.I)
BARRIER = re.compile(r"^(?:ELSE|ENDIF|ENDLOOP|ENDWHILE|ENDDO|TRY|CATCH|CLEANUP|ENDTRY|CASE|WHEN|RETURN|EXIT|CONTINUE|CHECK|RAISE)\b", re.I)

def masked(s):
    # Keep template interpolations visible for def/use, mask literal payloads.
    s = re.sub(r"'(?:''|[^'])*'|`(?:``|[^`])*`", "", s)
    return re.sub(r"\|(?:\\.|[^|])*\|", lambda m: " ".join(re.findall(r"\{([^{}]*)\}", m[0])), s)

def hot(m):
    ts = m['ts']
    name = ts.rsplit('.', 1)[-1]
    if '/combi.ts.' in ts:
        return name in ('run', 'run_one')
    if ts.endswith('.Lexer.process') or ts.endswith('.Lexer.add'):
        return True
    if '.LexerStream.' in ts or '.Result.' in ts:
        return name != 'constructor'
    if '.StatementParser.' in ts:
        return name.startswith(('parse', 'run', 'find'))
    if any('.'+c+'.' in ts for c in ('ABAPFileInformation', 'CurrentScope', 'SpaghettiScope')):
        return name.lower().startswith(('get', 'find', 'lookup', 'resolve', 'has', 'is', 'exists'))
    return False

def csvwrite(path, rows, fields):
    with path.open('w', newline='') as f:
        w = csv.DictWriter(f, fieldnames=fields)
        w.writeheader()
        w.writerows({k:r.get(k,'') for k in fields} for r in rows)

def md(s):
    return str(s).replace('&','&amp;').replace('<','&lt;').replace('>','&gt;').replace('|','&#124;').replace('`','&#96;').replace('\n',' ')

COMMON = ('Require one definition and one value use on all paths, including handlers; '
          'no intervening write to RHS dependencies or alias; no escaped local, closure capture, '
          'GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type '
          'and conversion; evaluate RHS exactly once at the next evaluated effect, preserving '
          'operand and exception order; same basic block and loop execution domain. '
          'Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof.')

def classify(rhs, use, op):
    u = rhs.upper()
    if op == '?=':
        return 'downcast', 'ABAP-only', 'Replace declaration + t ?= x + single use with explicit CAST target( x ) at that use; retain a typed temporary when syntax requires it.', 'REF TO / ?= form is created by ABAP emission; runtime cast failure must stay at the same evaluated effect.'
    if u.startswith('CAST '):
        return 'cast', 'ABAP-only', 'Inline CAST target( x ) into the single argument/receiver/value use; remove its dead temp store.', 'This candidate targets the emitted ABAP CAST/receiver syntax, not a proven redundant HIR cast.'
    if u.startswith('CONV '):
        return 'conv', 'ABAP-only', 'Inline the same explicitly typed CONV into the single use; remove its dead temp store. Do not erase CONV without a separate type proof.', 'Explicit CONV and its ABAP conversion semantics only appear after emission.'
    if u.startswith('VALUE '):
        return 'value', 'ABAP-only', 'Inline the same explicitly typed VALUE constructor into the single use; retain constructor type instead of relying on # inference.', 'VALUE constructor syntax and ABAP inferred types are emission artifacts.'
    if u.startswith('XSDBOOL(') and use.upper().startswith('IF '):
        return 'bool', 'ABAP-only', 'Replace t = xsdbool( condition ); IF t = abap_true with IF condition.', 'ABAP xsdbool/abap_true forwarding is an emitted representation of a boolean.'
    if '|' in rhs:
        return 'template', 'ABAP-only', 'Inline the string template into its single use with the same string conversion and formatting.', 'String templates and their conversion/formatting rules are ABAP emission syntax.'
    return 'copy', 'HIR', 'Substitute the defining expression at its single use and delete the dead definition; keep any declaration needed for scope/type.', 'Single-use copy propagation is expressible as HIR -> HIR in Grace and benefits TS-HG@Go.'

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--statements', default='.local/peephole/export/statements.json')
    ap.add_argument('--input', default='out')
    ap.add_argument('--output', default='docs/history/2026-10-10-peephole-mining')
    args = ap.parse_args()
    methods = json.loads(Path(args.statements).read_text())
    out = Path(args.output); out.mkdir(parents=True, exist_ok=True)
    grams, candidates = {}, {}
    sites = []
    for m in methods:
        ss = m['statements'] or []
        h = hot(m)
        # Forwarding wrappers have no declared source; keep them in raw census,
        # exclude them from the rewrite shortlist.
        counts = collections.Counter(t.lower() for s in ss for t in TEMP.findall(masked(s['text'])))
        defs = collections.Counter()
        for s in ss:
            a = ASSIGN.match(s['text']); d = DECL.match(s['text'])
            if a: defs[(a[1] or a[2]).lower()] += 1
            elif d: counts[d[1].lower()] -= 1
        for n in (2,3,4):
            for i in range(len(ss)-n+1):
                w = ss[i:i+n]
                key = '\n'.join(s['normalized'] for s in w)
                loop = all(s['depth'] > 0 for s in w)
                weight = (4 if h else 1) * (4 if loop else 1)
                loc = f"{m['class']}=>{m['method']}:{w[0]['line']}"
                ex = '\n'.join(s['text'] for s in w)
                row = grams.setdefault(key, dict(n=n, pattern=key, sites=0, in_loop=0, hot_sites=0, hot_weight=0, example_location=loc, ts=m['ts'], ts_source=m['source'], example=ex))
                row['sites'] += 1; row['in_loop'] += int(loop); row['hot_sites'] += int(h); row['hot_weight'] += weight
        if not m['source']:
            continue
        for i, s in enumerate(ss):
            a = ASSIGN.match(s['text'])
            if not a: continue
            v, op, rhs = (a[1] or a[2]).lower(), a[3], a[4]
            if defs[v] != 1 or counts[v] != 2 or v in [t.lower() for t in TEMP.findall(masked(rhs))]:
                continue
            # Read immediately, allowing only non-executable declarations between.
            j = i+1
            while j < len(ss) and DECL.match(ss[j]['text']) and j-i < 3: j += 1
            if j >= len(ss) or j-i > 3: continue
            use = ss[j]['text']
            if BARRIER.match(use) or ss[j]['depth'] != s['depth']: continue
            if sum(t.lower()==v for t in TEMP.findall(masked(use))) != 1: continue
            if use.upper().startswith(('LOOP ', 'WHILE ', 'DO ', 'ELSEIF ')): continue
            start = i
            # Include the reference declaration for a ?= syntax candidate.
            if op == '?=' and i and DECL.match(ss[i-1]['text']) and DECL.match(ss[i-1]['text'])[1].lower()==v:
                start = i-1
            w = ss[start:j+1]
            if not 2 <= len(w) <= 4: continue
            family, tag, replacement, justification = classify(rhs, use, op)
            key = '\n'.join(x['normalized'] for x in w)
            loc = f"{m['class']}=>{m['method']}:{w[0]['line']}"
            loop = all(x['depth']>0 for x in w)
            weight = (4 if h else 1)*(4 if loop else 1)
            row = candidates.setdefault(key, dict(pattern=key,n=len(w),family=family,tag=tag,sites=0,in_loop=0,hot_sites=0,hot_weight=0,saved_per_exec=1,replacement=replacement,side_conditions=COMMON,tag_justification=justification,example_location=loc,ts=m['ts'],ts_source=m['source'],example='\n'.join(x['text'] for x in w)))
            row['sites'] += 1; row['in_loop'] += int(loop); row['hot_sites'] += int(h); row['hot_weight'] += weight
            sites.append(dict(pattern=key,example_location=loc,ts=m['ts'],ts_source=m['source'],in_loop=int(loop),hot=int(h),weight=weight,example='\n'.join(x['text'] for x in w)))
    ranked = sorted(candidates.values(), key=lambda r:(-r['hot_weight']*r['saved_per_exec'],-r['sites'],r['pattern']))
    for i,r in enumerate(ranked,1):
        r['rule_id'] = f'PM{i:04d}'
        r['in_loop_share'] = f"{r['in_loop']/r['sites']:.4f}"
    fields = ['rule_id','n','family','pattern','tag','sites','in_loop','in_loop_share','hot_sites','hot_weight','saved_per_exec','replacement','side_conditions','tag_justification','example_location','ts','ts_source','example']
    csvwrite(out/'candidates.csv',ranked,fields)
    raw = sorted(grams.values(),key=lambda r:(-r['hot_weight'],-r['sites'],r['pattern']))
    csvwrite(out/'ngrams.csv',raw,['n','pattern','sites','in_loop','hot_sites','hot_weight','example_location','ts','ts_source','example'])
    ids = {r['pattern']:r['rule_id'] for r in ranked}
    for s in sites: s['rule_id'] = ids[s['pattern']]
    csvwrite(out/'occurrences.csv',sites,['rule_id','example_location','ts','ts_source','in_loop','hot','weight','example'])
    # Retain one concrete/normalized entry per lexical statement, without
    # checking a large intermediate JSON file into git.
    with (out/'statements.json.gz').open('wb') as f:
        with gzip.GzipFile(fileobj=f,mode='wb',filename='',mtime=0) as g:
            g.write(Path(args.statements).read_bytes())
    ledger = []
    for r in ranked[:20]:
        ledger.append(dict(r, correctness_gate='pending',osgo_delta='pending',a4h_ns='pending',decision='pending'))
    ledgerpath = out.parent/'2026-10-10-peephole-ledger.csv'
    csvwrite(ledgerpath,ledger,['rule_id','pattern','tag','sites','in_loop','hot_weight','saved_per_exec','correctness_gate','osgo_delta','a4h_ns','decision'])
    hashes = {}
    for p in sorted(Path(args.input).glob('classes/*.clas.abap'))+[Path(args.input)/'names.json']:
        hashes[str(p.relative_to(Path(args.input)))] = hashlib.sha256(p.read_bytes()).hexdigest()
    manifest = dict(base_commit='db8173d',methods=len(methods),statements=sum(len(m['statements'] or []) for m in methods),unique_ngrams=len(raw),candidate_patterns=len(ranked),candidate_sites=len(sites),weight='sum((4 if hot else 1) * (4 if all statements in loop else 1)); rank = weight * saved_per_exec, then sites desc, pattern asc',files=hashes)
    (out/'manifest.json').write_text(json.dumps(manifest,indent=2,sort_keys=True)+'\n')
    b = ['# Peephole mining on main after #80/#81\n',
         f"Base db8173d; {len(methods)} emitted class methods, {manifest['statements']} lexical statements, {len(raw)} distinct n=2..4 windows. {len(ranked)} candidate shapes, {len(sites)} syntactically eligible sites. No optimizer changes or runtime measurements.\n",
         'Ranking: `sum((hot ? 4 : 1) * (in_loop ? 4 : 1)) * saved_per_exec`; ties use sites descending then pattern ascending. In-loop means every statement in the window has lexical loop depth > 0. Nested depth is not multiplied again. Hot methods: all combi.ts run/run_one; Lexer.process/add; LexerStream and Result except constructors; StatementParser parse*/run*/find*; ABAPFileInformation, CurrentScope and SpaghettiScope get*/find*/lookup*/resolve*/has*/is*/exists*. Case-insensitive lookup prefixes. This is a prioritization heuristic, not dynamic profile data.\n',
         'Raw windows may cross control boundaries, overlap, include unreachable traps and generated forwarding methods; candidates require a declared TS source, one textual temp definition and one textual value use, equal loop depth and adjacent use with only plain DATA declarations between. Each shape is a separate candidate specialization; shared families can share a rule. Safety gates remain pending. Overlaps and alternative specializations must not be added as independent savings. HIR rows go to Grace on HIR so every backend benefits.\n',
         'Normalization is exactly stmt-patterns: joined multiline/chained lexical statements, temp identities -> TMP, hashed names -> NAME, numeric literals -> NUM, quoted strings/templates -> LIT. It is intentionally lossy: TMP does not imply identity, and templates hide interpolation in normalized text. Concrete token scans retain simple interpolation uses; guards must use full typed def/use and control-flow facts. A chained statement remains one entry, never split at commas in constructor/call syntax.\n',
         'A site saves **one executed store/assignment** if its guard succeeds; plain DATA declarations are not counted as runtime savings. Repeated iterations do not make an ordinary DATA declaration reset a local. No #80 initialization removals or #81 quoted numeric removal is proposed again.\n',
         'Artifacts: [full candidate ranking](candidates.csv), [all n-grams](ngrams.csv), [eligible occurrences](occurrences.csv), [normalized statement entries](statements.json.gz), [input fingerprints](manifest.json), [A4H fixtures](A4H.md). Regenerate with the commands in cmd/stmt-patterns/README.md, then `python3 tools/peephole-bench.py`.\n',
         '| ID | Tag | Family | Sites | In loop | Share | Hot weight | Saved/exec | Normalized pattern |\n|---|---|---|---:|---:|---:|---:|---:|---|']
    for r in ranked[:20]:
        b.append(f"| {r['rule_id']} | {r['tag']} | {r['family']} | {r['sites']} | {r['in_loop']} | {r['in_loop_share']} | {r['hot_weight']} | 1 | <code>{md(r['pattern'])}</code> |")
    b.append('\n## Candidate details\n')
    for r in ranked[:20]:
        b.extend([f"### {r['rule_id']} — {r['tag']}\n",f"`{r['example_location']}`; TS `{r['ts']}` (`{r['ts_source']}`).\n",'```abap\n'+r['example']+'\n```\n',r['replacement']+' Saves 1 statement per successful execution.\n', 'Soundness: '+r['side_conditions']+'\n','Tag: '+r['tag_justification']+'\n'])
    (out/'README.md').write_text('\n'.join(b))
    (out.parent/'2026-10-10-peephole-ledger.md').write_text('# Peephole ledger — 2026-10-10\n\nAnalysis baseline: db8173d, after #80 and #81. [Top 20 and full methodology](2026-10-10-peephole-mining/README.md); [ledger CSV](2026-10-10-peephole-ledger.csv).\n\nEach row starts pending. Promote only after typed correctness proofs and positive/negative guard tests, OSGO differential equality (record corpus/hash and delta), and A4H median ns/execution before/after. Record decision with reason. HIR rows belong in Grace HIR -> HIR for TS-HG@Go too. ABAP-only rows belong in pure method-local window rules with side conditions, iterated to a bounded fixed point; require decreasing store count, deterministic rule order and idempotence.\n\nColumns: rule id, pattern, tag, sites, in-loop, hot weight, stmts saved/exec, correctness gate (pending), OSGO delta (pending), A4H ns (pending), decision (pending). Counts are lexical, overlapping, conditional opportunities. No correctness or performance gate has been claimed.\n')
    print(json.dumps({k:v for k,v in manifest.items() if k!='files'},indent=2))
    for r in ranked[:20]: print(r['rule_id'],r['family'],r['tag'],r['sites'],r['in_loop'],r['hot_weight'],repr(r['pattern']))

if __name__ == '__main__': main()

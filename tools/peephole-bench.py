#!/usr/bin/env python3
"""Emit runnable A4H micro-benchmark fixtures for the top-20 ABAP-only rows."""
import csv
from pathlib import Path

ROOT = Path('docs/history/2026-10-10-peephole-mining')
N = 5_000_000
FIXTURES = {
    'cast': ('DATA(t1) = CAST lcl_probe( source ).\nsink = lcl_probe=>consume( t1 ).', 'sink = lcl_probe=>consume( CAST lcl_probe( source ) ).'),
    'downcast': ('DATA t1 TYPE REF TO lcl_probe.\nt1 ?= source.\nsink = lcl_probe=>consume( t1 ).', 'sink = lcl_probe=>consume( CAST lcl_probe( source ) ).'),
    'conv': ('DATA(t1) = CONV string( input ).\ntext_sink = t1.', 'text_sink = CONV string( input ).'),
    'value': ('DATA(t1) = VALUE ty_row( low = sy-index sign = \'I\' option = \'EQ\' ).\nrow_sink = t1.', 'row_sink = VALUE ty_row( low = sy-index sign = \'I\' option = \'EQ\' ).'),
    'bool': ('DATA(t1) = xsdbool( sy-index > 0 ).\nIF t1 = abap_true.\n  sink = sy-index.\nENDIF.', 'IF sy-index > 0.\n  sink = sy-index.\nENDIF.'),
    'template': ('DATA(t1) = |{ sy-index }|.\ntext_sink = t1.', 'text_sink = |{ sy-index }|.'),
    'box-copy': ('DATA t1 TYPE REF TO object.\nt1 = source.\nbox->oval = t1.', 'box->oval = source.'),
    'cast-copy': ('DATA t1 TYPE REF TO object.\nt1 = source.\nDATA t2 TYPE REF TO lcl_probe.\nt2 ?= t1.', 'DATA t2 TYPE REF TO lcl_probe.\nt2 ?= source.'),
    'conv-copy': ('DATA(t1) = input.\ntext_sink = CONV string( t1 ).', 'text_sink = CONV string( input ).'),
    'cast-assign': ('DATA(t1) = CAST lcl_probe( source ).\nref_sink = t1.', 'ref_sink = CAST lcl_probe( source ).'),
    'cast-field': ('DATA(t1) = CAST lcl_probe( source ).\nsink = t1->payload.', 'sink = CAST lcl_probe( source )->payload.'),
    'cast-call': ('DATA(t1) = CAST lcl_probe( source ).\nsink = t1->read( ).', 'sink = CAST lcl_probe( source )->read( ).'),
    'cast-initial': ('DATA(t1) = CAST lcl_probe( source ).\nIF t1 IS INITIAL.\n  sink = 0.\nELSE.\n  sink = sy-index.\nENDIF.', 'IF CAST lcl_probe( source ) IS INITIAL.\n  sink = 0.\nELSE.\n  sink = sy-index.\nENDIF.'),
    'cast-bound': ('DATA(t1) = CAST lcl_probe( source ).\nbool_sink = xsdbool( t1 IS BOUND ).', 'bool_sink = xsdbool( CAST lcl_probe( source ) IS BOUND ).'),
    'conv-bool': ('DATA(t1) = CONV abap_bool( abap_true ).\nIF t1 = abap_true.\n  sink = sy-index.\nENDIF.', 'IF CONV abap_bool( abap_true ) = abap_true.\n  sink = sy-index.\nENDIF.'),
    'conv-compare': ('DATA(t1) = CONV string( |abcdef| ).\nbool_sink = xsdbool( t1 = input ).', 'bool_sink = xsdbool( CONV string( |abcdef| ) = input ).'),
    'cast-copy-decl-ref': ('DATA t1 TYPE REF TO object.\nt1 = source.\nDATA unused TYPE REF TO lcl_probe.\nDATA(t3) = CAST lcl_probe( t1 ).', 'DATA unused TYPE REF TO lcl_probe.\nDATA(t3) = CAST lcl_probe( source ).'),
    'cast-copy-decl-string': ('DATA t1 TYPE REF TO object.\nt1 = source.\nDATA unused TYPE string.\nDATA(t3) = CAST lcl_probe( t1 ).', 'DATA unused TYPE string.\nDATA(t3) = CAST lcl_probe( source ).'),
}
FIXTURES['bool'] = ('DATA(t1) = xsdbool( source IS BOUND ).\nIF t1 = abap_true.\n  sink = sy-index.\nENDIF.', 'IF source IS BOUND.\n  sink = sy-index.\nENDIF.')
FIXTURES['downcast'] = ('DATA t1 TYPE REF TO lcl_probe.\nt1 ?= source.\nref_sink = t1.', 'ref_sink = CAST lcl_probe( source ).')

PREFIX = '''REPORT zpeephole_bench.
CLASS lcl_probe DEFINITION FINAL.
  PUBLIC SECTION.
    DATA payload TYPE i VALUE 7.
    CLASS-METHODS consume IMPORTING obj TYPE REF TO lcl_probe RETURNING VALUE(result) TYPE i.
    METHODS read RETURNING VALUE(result) TYPE i.
ENDCLASS.
CLASS lcl_box DEFINITION FINAL.
  PUBLIC SECTION.
    DATA oval TYPE REF TO object.
ENDCLASS.
CLASS lcl_box IMPLEMENTATION.
ENDCLASS.
CLASS lcl_probe IMPLEMENTATION.
  METHOD consume.
    result = obj->payload.
  ENDMETHOD.
  METHOD read.
    result = payload.
  ENDMETHOD.
ENDCLASS.
TYPES: BEGIN OF ty_row,
         low TYPE i,
         sign TYPE c LENGTH 1,
         option TYPE c LENGTH 2,
       END OF ty_row.
DATA source TYPE REF TO object.
DATA box TYPE REF TO lcl_box.
DATA input TYPE c LENGTH 12 VALUE 'abcdef'.
DATA sink TYPE i.
DATA ref_sink TYPE REF TO lcl_probe.
DATA bool_sink TYPE abap_bool.
DATA text_sink TYPE string.
DATA row_sink TYPE ty_row.
DATA begin_us TYPE i.
DATA end_us TYPE i.
DATA elapsed_us TYPE int8.
source = NEW lcl_probe( ).
box = NEW lcl_box( ).
'''

def bench_family(row):
    pattern = row['pattern']
    if row['family'] == 'cast':
        use = pattern.splitlines()[-1]
        if use == 'TMP = TMP.': return 'cast-assign'
        if 'IF TMP IS INITIAL' in use: return 'cast-initial'
        if 'XSDBOOL( TMP IS BOUND' in use: return 'cast-bound'
        if '->NAME( ' in use: return 'cast-call'
        if '->' in use: return 'cast-field'
        return 'cast'
    if row['family'] == 'conv':
        if 'CONV ABAP_BOOL( ABAP_TRUE )' in pattern: return 'conv-bool'
        if 'XSDBOOL( TMP = TMP )' in pattern: return 'conv-compare'
    if row['family'] != 'abap-copy': return row['family']
    if '->OVAL' in pattern: return 'box-copy'
    if 'CONV ' in pattern: return 'conv-copy'
    if 'CAST ' in pattern:
        return 'cast-copy-decl-string' if 'DATA TMP TYPE STRING.' in pattern else 'cast-copy-decl-ref'
    return 'cast-copy'

def main():
    rows = list(csv.DictReader((ROOT/'candidates.csv').open()))[:20]
    dest = ROOT/'a4h'; dest.mkdir(exist_ok=True)
    groups = {}
    for r in rows:
        if r['tag']=='ABAP-only': groups.setdefault(bench_family(r), []).append(r['rule_id'])
    text = ['# A4H micro-benchmark specifications\n',
            f'Loop count: **{N:,}** per timed arm. Run each report before/after separately on the same A4H kernel, warm both arms once, then alternate order for seven samples; record median elapsed_us * 1000 / {N} in ns/execution. Compare scalar and reference sinks between arms (ASSERT in each report); retain observable output. No numbers have been measured. The reports use representative local types and exercise the store-elimination mechanism; instantiate each candidate\'s actual type and receiver/argument position before deciding it is profitable. Same result type and conversion are mandatory. Shared fixtures are explicitly mapped below; no workload call counts or OSGO equality are inferred.\n',
            'Local parser/type-check evidence: [a4h-syntax.json](a4h-syntax.json). Recheck with `npm ci --ignore-scripts --cache "$HOME/.cache/abapiti-npm-cache"` then `node tools/peephole-check-benches.cjs`. The repository lock selects @abaplint/core 2.118.0 and the check targets v758 (matching the existing A4H benchmark context). Kernel activation and measurements remain pending.\n',
            'Syntax-check/import each report as a temporary local program; the name is zpeephole_bench in each independent file. Reports run the same loop twice, with the first loop warming that arm; only the second is timed. Pure DATA declarations are outside timed loops except inline declarations under test. The class, allocation, inputs and timer overhead are identical in both arms. Check bound and incompatible-reference exceptions separately outside timing for CAST/?= guards; handlers observing removed locals invalidate the rule.\n']
    for family, ids in groups.items():
        before, after = FIXTURES[family]
        text += [f"## {', '.join(ids)} — {family}\n",f'Loop count: {N:,}. Savings if guard holds: one executed store per iteration.\n','Before:\n```abap\n'+before+'\n```\n','After:\n```abap\n'+after+'\n```\n']
        for label, body in [('before',before),('after',after)]:
            timed = '\n'.join('  '+line for line in body.splitlines())
            # Inline DATA is method/program scoped; reuse it for the warm+timed
            # outer loop rather than redeclaring the same name in two DO bodies.
            program = PREFIX + f'''DO 2 TIMES.
  GET RUN TIME FIELD begin_us.
  DO {N} TIMES.
{timed}
  ENDDO.
  GET RUN TIME FIELD end_us.
  elapsed_us = CONV int8( end_us ) - CONV int8( begin_us ).
ENDDO.
'''
            checks = {
                'cast':'ASSERT sink = 7.', 'downcast':'ASSERT sink = 7.',
                'conv':"ASSERT text_sink = CONV string( input ).",
                'value':f"ASSERT row_sink-low = {N}.\nASSERT row_sink-sign = 'I'.\nASSERT row_sink-option = 'EQ'.",
                'bool':f'ASSERT sink = {N}.',
                'template':f'ASSERT text_sink = |{N}|.',
                'box-copy':'ASSERT box->oval = source.',
                'cast-copy':'ASSERT t2 = source.',
                'conv-copy':'ASSERT text_sink = CONV string( input ).',
                'cast-assign':'ASSERT ref_sink = source.',
                'cast-field':'ASSERT sink = 7.',
                'cast-call':'ASSERT sink = 7.',
                'cast-initial':f'ASSERT sink = {N}.',
                'cast-bound':'ASSERT bool_sink = abap_true.',
                'conv-bool':f'ASSERT sink = {N}.',
                'conv-compare':'ASSERT bool_sink = abap_true.',
                'cast-copy-decl-ref':'ASSERT t3 = source.',
                'cast-copy-decl-string':'ASSERT t3 = source.',
            }
            checks['downcast'] = 'ASSERT ref_sink = source.'
            program += checks[family]+"\nWRITE: / elapsed_us, / sink, / text_sink, / row_sink-low.\n"
            (dest/f'{family}-{label}.prog.abap').write_text(program)
        text.append(f'Ready reports: [before](a4h/{family}-before.prog.abap), [after](a4h/{family}-after.prog.abap).\n')
    (ROOT/'A4H.md').write_text('\n'.join(text))
    with (ROOT/'bench-map.csv').open('w',newline='') as f:
        w=csv.writer(f); w.writerow(['rule_id','family','loop_count','before','after','status'])
        for r in rows:
            if r['tag']=='ABAP-only':
                family=bench_family(r); w.writerow([r['rule_id'],family,N,f'a4h/{family}-before.prog.abap',f'a4h/{family}-after.prog.abap','pending A4H syntax check and measurement'])

if __name__=='__main__': main()

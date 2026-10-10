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
}

PREFIX = '''REPORT zpeephole_bench.
CLASS lcl_probe DEFINITION FINAL.
  PUBLIC SECTION.
    DATA payload TYPE i VALUE 7.
    CLASS-METHODS consume IMPORTING obj TYPE REF TO lcl_probe RETURNING VALUE(result) TYPE i.
ENDCLASS.
CLASS lcl_probe IMPLEMENTATION.
  METHOD consume.
    result = obj->payload.
  ENDMETHOD.
ENDCLASS.
TYPES: BEGIN OF ty_row,
         low TYPE i,
         sign TYPE c LENGTH 1,
         option TYPE c LENGTH 2,
       END OF ty_row.
DATA source TYPE REF TO object.
DATA input TYPE c LENGTH 12 VALUE 'abcdef'.
DATA sink TYPE i.
DATA text_sink TYPE string.
DATA row_sink TYPE ty_row.
DATA begin_us TYPE i.
DATA end_us TYPE i.
DATA elapsed_us TYPE int8.
source = NEW lcl_probe( ).
'''

def main():
    rows = list(csv.DictReader((ROOT/'candidates.csv').open()))[:20]
    dest = ROOT/'a4h'; dest.mkdir(exist_ok=True)
    groups = {}
    for r in rows:
        if r['tag']=='ABAP-only': groups.setdefault(r['family'], []).append(r['rule_id'])
    text = ['# A4H micro-benchmark specifications\n',
            f'Loop count: **{N:,}** per timed arm. Run each report before/after separately on the same A4H kernel, warm both arms once, then alternate order for seven samples; record median elapsed_us * 1000 / {N} in ns/execution. Compare all three sinks between arms (ASSERT in each report); retain observable output. No numbers have been measured. The reports use representative local types and exercise the store-elimination mechanism; instantiate each candidate\'s actual type and receiver/argument position before deciding it is profitable. Same result type and conversion are mandatory. Shared fixtures are explicitly mapped below; no workload call counts or OSGO equality are inferred.\n',
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
            }
            program += checks[family]+"\nWRITE: / elapsed_us, / sink, / text_sink, / row_sink-low.\n"
            (dest/f'{family}-{label}.prog.abap').write_text(program)
        text.append(f'Ready reports: [before](a4h/{family}-before.prog.abap), [after](a4h/{family}-after.prog.abap).\n')
    (ROOT/'A4H.md').write_text('\n'.join(text))
    with (ROOT/'bench-map.csv').open('w',newline='') as f:
        w=csv.writer(f); w.writerow(['rule_id','family','loop_count','before','after','status'])
        for r in rows:
            if r['tag']=='ABAP-only':
                family=r['family']; w.writerow([r['rule_id'],family,N,f'a4h/{family}-before.prog.abap',f'a4h/{family}-after.prog.abap','pending A4H syntax check and measurement'])

if __name__=='__main__': main()

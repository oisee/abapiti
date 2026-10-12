# Batch O remaining Node-active tests

Frozen semantic source: `a827974`, based on `f78b973`. Original Node source pin:
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`.

23 remaining = 20 traps + 3 unregistered, FAIL 0, async-pending 0.
Original Node-pending cases are outside this list and the 11,060 denominator.
Exact measured messages/groups are in [final evidence](evidence/traps-o-final.json).
Original title/source-site audit is preserved from [train 11](train-11-remaining.md);
no source test or title is edited. The constructor case advances only to the
existing batch N helper refusal, so is not counted as a pass.

1. **abap/structures — structure combi, complex1 complex1 match**
   - Boundary: Unregistered, batch N.
   - Original source sites: `test/abap/structures/_combi.ts:284:3`, `test/abap/structures/_combi.ts:276:1`.

2. **rules — test line_length rule "hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello" should have 1 issue(s)**
   - Boundary: Unregistered, batch N.
   - Original source sites: `test/rules/_utils.ts:60:9`, `test/rules/_utils.ts:39:11`, `test/rules/_utils.ts:36:3`, `test/rules/line_length.ts:17:9`.

3. **rules — test line_length rule "WRITE: / 'abc'." should have 0 issue(s)**
   - Boundary: Unregistered, batch N.
   - Original source sites: `test/rules/_utils.ts:60:9`, `test/rules/_utils.ts:39:11`, `test/rules/_utils.ts:36:3`, `test/rules/line_length.ts:17:9`.

4. **lsp — LSP, help first character = DO**
   - Boundary: DumpScope ToPrimitive, batch N.
   - Original source sites: `test/lsp/help.ts:9:3`, `test/lsp/help.ts:7:1`.

5. **lsp — LSP, help class**
   - Boundary: DumpScope ToPrimitive, batch N.
   - Original source sites: `test/lsp/help.ts:25:3`, `test/lsp/help.ts:7:1`.

6. **rules — Rule: dynpro_checks Element SDFSDF length longer than 132**
   - Boundary: Dynpros absence, Dell.
   - Original source sites: `test/rules/dynpro_checks.ts:21:3`, `test/rules/dynpro_checks.ts:19:1`.

7. **rules — Rule: dynpro_checks overlapping dynpro fields**
   - Boundary: Dynpros absence, Dell.
   - Original source sites: `test/rules/dynpro_checks.ts:197:3`, `test/rules/dynpro_checks.ts:19:1`.

8. **objects — malformed object XML ignores structured values where text is expected**
   - Boundary: MessageClass absence, Dell.
   - Original source sites: `test/objects/malformed_xml.ts:120:3`, `test/objects/malformed_xml.ts:19:1`.

9. **rules — Message rule no number**
   - Boundary: MessageClass absence, Dell.
   - Original source sites: `test/rules/msag_consistency.ts:41:3`, `test/rules/msag_consistency.ts:14:1`.

10. **root — exclude list will exclude issues based on the local exclude patterns**
   - Boundary: SpaceBeforeDot config absence, Dell.
   - Original source sites: `test/registry.ts:466:3`, `test/registry.ts:368:1`.

11. **root — severity an issue will have the severity specified in the config**
   - Boundary: SpaceBeforeDot config absence, Dell.
   - Original source sites: `test/severity.ts:30:3`, `test/severity.ts:6:1`.

12. **lsp — LSP, help CDS**
   - Boundary: Help CDS deletion, Dell; JSON order independently certified by O.
   - Original source sites: `test/lsp/help.ts:39:3`, `test/lsp/help.ts:7:1`.

13. **objects — malformed object XML ignores malformed inner object metadata**
   - Boundary: Proxy metadata absence, Dell.
   - Original source sites: `test/objects/malformed_xml.ts:71:3`, `test/objects/malformed_xml.ts:19:1`.

14. **rules — test line_only_punc rule "." should have 1 issue(s)**
   - Boundary: LineOnlyPunc conversion/absence, Dell.
   - Original source sites: `test/rules/_utils.ts:60:9`, `test/rules/_utils.ts:39:11`, `test/rules/_utils.ts:36:3`, `test/rules/line_only_punc.ts:23:9`.

15. **abap/types — Types, TYPES TYPES structure should sometimes inherit qualified name**
   - Boundary: Types qualified-name conversion/absence, Dell.
   - Original source sites: `test/abap/types/types.ts:18:3`, `test/abap/types/types.ts:17:1`.

16. **cds — CDS Parser define table entity — field annotations sit under CDSTableField**
   - Boundary: Now CDS dynamic get/getChildren helper at 3804, batch N; constructor boundary repaired by O.
   - Original source sites: `test/cds/cds_parser.ts:3725:3`, `test/cds/cds_parser.ts:7:1`.

17. **cds — CDS Parser define table entity with association/composition wrapped in CDSTableField**
   - Boundary: CDS dynamic get/getChildren helper, batch N.
   - Original source sites: `test/cds/cds_parser.ts:3782:3`, `test/cds/cds_parser.ts:7:1`.

18. **cds — CDS Parser — quoted identifiers FROM-clause alias '"=A0"' token value preserved in CDSAs → CDSName**
   - Boundary: CDS dynamic getFirstToken, batch N.
   - Original source sites: `test/cds/cds_parser.ts:3842:3`, `test/cds/cds_parser.ts:3815:1`.

19. **objects — Domain, parse main xml parser error**
   - Boundary: Domain XML conversion, Dell.
   - Original source sites: `test/objects/domain.ts:129:3`, `test/objects/domain.ts:101:1`.

20. **objects — Domain, parse main xml parser error, valid xml**
   - Boundary: Domain XML conversion, Dell.
   - Original source sites: `test/objects/domain.ts:138:3`, `test/objects/domain.ts:101:1`.

21. **objects — View, parse XML database view with 2 append views**
   - Boundary: View XML conversion, Dell.
   - Original source sites: `test/objects/view.ts:97:3`, `test/objects/view.ts:7:1`.

22. **pretty_printer — Remove sequential blanks, no config REPORT zfoo.**
   - Boundary: Pretty-printer deletion, Dell.
   - Original source sites: `test/pretty_printer/pretty_printer.ts:179:5`, `test/pretty_printer/pretty_printer.ts:178:9`, `test/pretty_printer/pretty_printer.ts:170:1`.

23. **root — exclude list will not crash with an undefined global exclude list**
   - Boundary: Registry deletion, Dell.
   - Original source sites: `test/registry.ts:452:3`, `test/registry.ts:368:1`.


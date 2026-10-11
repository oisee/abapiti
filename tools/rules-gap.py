#!/usr/bin/env python3
"""Count --full evidence for rules except downport, including cascades.

Usage: python3 tools/rules-gap.py OUTPUT/evidence/registry-blocking.json
Ownership follows the fit-rules batch assignment at origin/main f834c16.
"""
import collections
import json
import re
import sys


def owner(file, line, message):
    if file in {"7bit_ascii.ts", "whitespace_end.ts"}:
        return "excluded: RegExp"
    if file == "cds_check_syntax.ts":
        return "excluded: string match" if line <= 284 else "excluded: RegExp"
    if file == "contains_tab.ts":
        return "excluded: misc-roots" if line == 51 else "excluded: string repeat"
    if file in {"function_module_recommendations.ts", "identical_conditions.ts", "remove_descriptions.ts"}:
        return "excluded: misc-roots"
    if file in {"constant_classes.ts", "no_chained_assignment.ts", "parser_missing_space.ts", "sequential_blank.ts", "unused_variables.ts"}:
        return "excluded: small builtins"
    if file in {"align_parameters.ts", "align_pseudo_comments.ts", "align_type_expressions.ts", "definitions_top.ts", "in_statement_indentation.ts", "indentation.ts", "line_break_multiple_parameters.ts"}:
        return "excluded: string repeat"
    if file == "object_naming.ts":
        if "RegExp" in message or "identifier regex " in message:
            return "excluded: RegExp"
        return "owned: indexed nominal config"
    if file in {"parser_702_chaining.ts", "short_case.ts"}:
        return "owned: primitive find"
    if file == "intf_referencing_clas.ts":
        return "owned: reference || undefined"
    if file == "uncaught_exception.ts":
        return "owned: dense forEach"
    if file == "xml_consistency.ts":
        if line <= 63:
            return "owned: XMLValidator.validate"
        if line <= 68:
            return "excluded: string match"
        if line in {118, 122}:
            return "owned: Object.entries"
        return "excluded: small builtins"
    if file in {"abapdoc.ts", "change_if_to_case.ts", "global_class.ts", "inline_data_old_versions.ts", "msag_consistency.ts", "no_prefixes.ts"}:
        return "excluded: string match"
    return "unassigned: inspect diagnostic"


def summarize(path):
    diagnostics = json.load(open(path, encoding="utf-8"))
    rules = []
    owners = collections.Counter()
    for diagnostic in diagnostics:
        match = re.search(r"(?:^|/)src/rules/([^/:]+):(\d+):\d+$", diagnostic["loc"])
        if not match or match[1] == "downport.ts":
            continue
        rules.append(diagnostic)
        owners[owner(match[1], int(match[2]), diagnostic["message"])] += 1
    return {
        "total": len(diagnostics),
        "totalRoot": sum(not d.get("cascade", False) for d in diagnostics),
        "totalCascade": sum(d.get("cascade", False) for d in diagnostics),
        "totalByMessage": dict(collections.Counter(d["message"] for d in diagnostics).most_common()),
        "rules": len(rules),
        "rulesRoot": sum(not d.get("cascade", False) for d in rules),
        "owners": dict(sorted(owners.items())),
        "rulesByMessage": dict(collections.Counter(d["message"] for d in rules).most_common()),
    }


if __name__ == "__main__":
    print(json.dumps(summarize(sys.argv[1]), indent=2))

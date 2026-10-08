import {Lexer} from "../src/abap/1_lexer/lexer";
import {TestFile} from "./test_file";
import {StatementParser} from "../src/abap/2_statements/statement_parser";
import {StructureParser} from "../src/abap/3_structures/structure_parser";
import {IABAPLexerResult} from "../src/abap/1_lexer/lexer_result";
import {IStatementResult} from "../src/abap/2_statements/statement_result";
import {IStructureResult} from "../src/abap/3_structures/structure_result";
import {StatementsDump} from "./statements_dump";
import {Release} from "../src/version";
import {StatementNode} from "../src/abap/nodes/statement_node";
import {StructureNode} from "../src/abap/nodes/structure_node";

export class StructuresDump {
  public static lastTokens: number = -1;
  public static lastStatements: number = -1;
  public static lastStructures: number = -1;
  private static references: readonly StatementNode[];
  private static nextReference: number = 0;

  public static dump(raw: string, filename: string): string {
    const lexed = StructuresDump.lex(raw, filename);
    const parsed = StructuresDump.parseStatements(lexed);
    const result = StructuresDump.parseStructures(parsed);
    return StructuresDump.fromStructure(parsed, result);
  }

  public static lex(raw: string, filename: string): IABAPLexerResult {
    return new Lexer().run(new TestFile(raw, filename));
  }

  public static parseStatements(lexed: IABAPLexerResult): IStatementResult {
    return new StatementParser(Release.v758).run([lexed], [])[0];
  }

  public static parseStructures(input: IStatementResult): IStructureResult {
    return StructureParser.run(input);
  }

  public static dumpLexed(input: IABAPLexerResult): string {
    let out = "";
    for (const token of input.tokens) {
      if (out !== "") { out = out + "\n"; }
      out = out + token.constructor.name + "|" + token.getStr() + "|" + token.getRow().toString() + "|" + token.getCol().toString();
    }
    return out;
  }

  public static dumpStatements(input: IStatementResult): string {
    return StatementsDump.fromStatements(input.statements);
  }

  public static fromStructure(input: IStatementResult, result: IStructureResult): string {
    StructuresDump.lastTokens = input.tokens.length;
    const statements = input.statements;
    StructuresDump.lastStatements = statements.length;
    StructuresDump.lastStructures = 0;
    StructuresDump.references = statements;
    StructuresDump.nextReference = 0;
    let out = input.tokens.length.toString() + "|" + statements.length.toString() + "|" + result.issues.length.toString();
    for (const issue of result.issues) {
      out = out + "\nI|" + issue.getKey() + "|" + issue.getSeverity() + "|" + issue.getFilename()
        + "|" + issue.getStart().getRow().toString() + ":" + issue.getStart().getCol().toString()
        + "|" + issue.getEnd().getRow().toString() + ":" + issue.getEnd().getCol().toString()
        + "|" + issue.getMessage();
    }
    const root = result.node;
    if (root !== undefined) {
      out = out + StructuresDump.node(root, 0);
    }
    return out;
  }

  // Successful trees visit statements in source order. A forward cursor
  // avoids a quadratic reference map in ABAP. Fall back to a full identity
  // search for reordered or repeated references, preserving the exact dump.
  private static reference(node: StatementNode): number {
    let index = StructuresDump.nextReference;
    while (index < StructuresDump.references.length) {
      if (StructuresDump.references[index] === node) {
        StructuresDump.nextReference = index + 1;
        return index;
      }
      index = index + 1;
    }
    index = 0;
    while (index < StructuresDump.references.length) {
      if (StructuresDump.references[index] === node) {
        StructuresDump.nextReference = index + 1;
        return index;
      }
      index = index + 1;
    }
    throw new Error("structure references an unknown statement");
  }

  private static node(node: StructureNode | StatementNode, depth: number): string {
    if (node instanceof StatementNode) {
      const index = StructuresDump.reference(node);
      return "\n" + depth.toString() + "|T" + index.toString() + "|" + node.get().constructor.name;
    }
    StructuresDump.lastStructures = StructuresDump.lastStructures + 1;
    let out = "\n" + depth.toString() + "|S" + node.get().constructor.name + "|" + node.getChildren().length.toString();
    for (const child of node.getChildren()) {
      out = out + StructuresDump.node(child, depth + 1);
    }
    return out;
  }
}

// abapiti's differential-test driver, see README.md. Not part of abaplint.
import {Lexer} from "../src/abap/1_lexer/lexer";
import {TestFile} from "./test_file";
import {StatementParser} from "../src/abap/2_statements/statement_parser";
import {Release} from "../src/version";
import {StatementNode} from "../src/abap/nodes/statement_node";
import {TokenNode} from "../src/abap/nodes/token_node";
import {ExpressionNode} from "../src/abap/nodes/expression_node";

export class StatementsDump {
  public static lastTokens: number = -1;
  public static lastStatements: number = -1;

  public static dump(raw: string): string {
    const file = new TestFile(raw);
    const lexed = new Lexer().run(file);
    StatementsDump.lastTokens = lexed.tokens.length;
    const parsed = new StatementParser(Release.v758).run([{file: file, tokens: lexed.tokens}], []);
    const statements = parsed[0].statements;
    StatementsDump.lastStatements = statements.length;

    let out = statements.length.toString();
    for (const statement of statements) {
      out = out + "\n" + StatementsDump.statement(statement);
    }
    return out;
  }

  private static statement(statement: StatementNode): string {
    const first = statement.getFirstToken();
    const last = statement.getLastToken();

    let colon = "-";
    const colonToken = statement.getColon();
    if (colonToken !== undefined) {
      colon = colonToken.getRow().toString() + ":" + colonToken.getCol().toString();
    }

    let tree = "";
    for (const child of statement.getChildren()) {
      if (tree !== "") {
        tree = tree + ",";
      }
      tree = tree + StatementsDump.node(child);
    }

    return statement.get().constructor.name
      + "|" + colon
      + "|" + first.getRow().toString() + ":" + first.getCol().toString()
      + "|" + last.getRow().toString() + ":" + last.getCol().toString()
      + "|" + tree;
  }

  private static node(child: TokenNode | ExpressionNode): string {
    if (child instanceof TokenNode) {
      const token = child.get();
      return "T" + token.constructor.name + "[" + token.getStr() + "]";
    }
    let inner = "";
    for (const c of child.getChildren()) {
      if (inner !== "") {
        inner = inner + ",";
      }
      inner = inner + StatementsDump.node(c);
    }
    return "E" + child.get().constructor.name + "(" + inner + ")";
  }

  public static firstDiff(a: string, b: string): number {
    const la = a.length;
    const lb = b.length;
    const n = la < lb ? la : lb;
    let i = 0;
    while (i < n) {
      if (a.charCodeAt(i) !== b.charCodeAt(i)) {
        return i;
      }
      i = i + 1;
    }
    if (la !== lb) {
      return n;
    }
    return -1;
  }
}

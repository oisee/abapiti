// abapiti's differential-test driver, see README.md. Not part of abaplint.
import {Lexer} from "../src/abap/1_lexer/lexer";
import {TestFile} from "./test_file";
import {TokenName} from "./token_name";

export class LexerDump {
  public static dump(raw: string): string {
    const lexer = new Lexer();
    const file = new TestFile(raw);
    const result = lexer.run(file);
    let out = "";
    for (const t of result.tokens) {
      if (out !== "") {
        out = out + "\n";
      }
      out = out + TokenName.tokenName(t) + "|" + t.getStr() + "|" + t.getRow().toString() + "|" + t.getCol().toString();
    }
    return out;
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

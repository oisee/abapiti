// abapiti's differential-test driver, see README.md. Not part of abaplint.
import {AbstractToken} from "../src/abap/1_lexer/tokens/abstract_token";
import {At, AtW, BracketLeft, BracketLeftW, BracketRight, BracketRightW, Dash, DashW, InstanceArrow, InstanceArrowW, ParenLeft, ParenLeftW, ParenRight, ParenRightW, Plus, PlusW, StaticArrow, StaticArrowW, WAt, WAtW, WBracketLeft, WBracketLeftW, WBracketRight, WBracketRightW, WDash, WDashW, WInstanceArrow, WInstanceArrowW, WParenLeft, WParenLeftW, WParenRight, WParenRightW, WPlus, WPlusW, WStaticArrow, WStaticArrowW} from "../src/abap/1_lexer/tokens";
import {AssociationName} from "../src/abap/1_lexer/tokens/association_name";
import {Comment} from "../src/abap/1_lexer/tokens/comment";
import {Identifier} from "../src/abap/1_lexer/tokens/identifier";
import {Pragma} from "../src/abap/1_lexer/tokens/pragma";
import {Punctuation} from "../src/abap/1_lexer/tokens/punctuation";
import {StringTemplate} from "../src/abap/1_lexer/tokens/string_template";
import {StringTemplateBegin} from "../src/abap/1_lexer/tokens/string_template_begin";
import {StringTemplateEnd} from "../src/abap/1_lexer/tokens/string_template_end";
import {StringTemplateMiddle} from "../src/abap/1_lexer/tokens/string_template_middle";
import {StringToken} from "../src/abap/1_lexer/tokens/string";

export class TokenName {
  public static tokenName(t: AbstractToken): string {
    if (t instanceof At) { return "At"; }
    if (t instanceof AtW) { return "AtW"; }
    if (t instanceof AssociationName) { return "AssociationName"; }
    if (t instanceof BracketLeft) { return "BracketLeft"; }
    if (t instanceof BracketLeftW) { return "BracketLeftW"; }
    if (t instanceof BracketRight) { return "BracketRight"; }
    if (t instanceof BracketRightW) { return "BracketRightW"; }
    if (t instanceof Comment) { return "Comment"; }
    if (t instanceof Dash) { return "Dash"; }
    if (t instanceof DashW) { return "DashW"; }
    if (t instanceof Identifier) { return "Identifier"; }
    if (t instanceof InstanceArrow) { return "InstanceArrow"; }
    if (t instanceof InstanceArrowW) { return "InstanceArrowW"; }
    if (t instanceof ParenLeft) { return "ParenLeft"; }
    if (t instanceof ParenLeftW) { return "ParenLeftW"; }
    if (t instanceof ParenRight) { return "ParenRight"; }
    if (t instanceof ParenRightW) { return "ParenRightW"; }
    if (t instanceof Plus) { return "Plus"; }
    if (t instanceof PlusW) { return "PlusW"; }
    if (t instanceof Pragma) { return "Pragma"; }
    if (t instanceof Punctuation) { return "Punctuation"; }
    if (t instanceof StaticArrow) { return "StaticArrow"; }
    if (t instanceof StaticArrowW) { return "StaticArrowW"; }
    if (t instanceof StringTemplate) { return "StringTemplate"; }
    if (t instanceof StringTemplateBegin) { return "StringTemplateBegin"; }
    if (t instanceof StringTemplateEnd) { return "StringTemplateEnd"; }
    if (t instanceof StringTemplateMiddle) { return "StringTemplateMiddle"; }
    if (t instanceof StringToken) { return "StringToken"; }
    if (t instanceof WAt) { return "WAt"; }
    if (t instanceof WAtW) { return "WAtW"; }
    if (t instanceof WBracketLeft) { return "WBracketLeft"; }
    if (t instanceof WBracketLeftW) { return "WBracketLeftW"; }
    if (t instanceof WBracketRight) { return "WBracketRight"; }
    if (t instanceof WBracketRightW) { return "WBracketRightW"; }
    if (t instanceof WDash) { return "WDash"; }
    if (t instanceof WDashW) { return "WDashW"; }
    if (t instanceof WInstanceArrow) { return "WInstanceArrow"; }
    if (t instanceof WInstanceArrowW) { return "WInstanceArrowW"; }
    if (t instanceof WParenLeft) { return "WParenLeft"; }
    if (t instanceof WParenLeftW) { return "WParenLeftW"; }
    if (t instanceof WParenRight) { return "WParenRight"; }
    if (t instanceof WParenRightW) { return "WParenRightW"; }
    if (t instanceof WPlus) { return "WPlus"; }
    if (t instanceof WPlusW) { return "WPlusW"; }
    if (t instanceof WStaticArrow) { return "WStaticArrow"; }
    if (t instanceof WStaticArrowW) { return "WStaticArrowW"; }
    return "Unknown";
  }
}

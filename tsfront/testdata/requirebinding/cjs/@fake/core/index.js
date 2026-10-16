"use strict";
class Kind {}
class CharKind extends Kind {
  constructor(length) { super(); this.length = length; }
  name() { return "C" + this.length; }
  static of(length) { return new CharKind(length); }
}
class IntKind extends Kind {
  name() { return "I"; }
}
class Token {
  constructor(value) { this.value = value; }
  text() { return "'" + this.value + "'"; }
}
exports.Kinds = {Kind, CharKind, IntKind};
exports.Token = Token;
exports.Level = {Low: "L", High: "H"};
exports.LIMIT = 3;
exports.twice = (n) => n * 2;

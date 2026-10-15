"use strict";
class Rearranger {
  run(kinds) {
    const names = [];
    for (const k of kinds) names.unshift(k.name());
    return names.join(">");
  }
}
exports.Rearranger = Rearranger;

"use strict";
exports.answer = 42;
exports.count = 0;
function bump() { exports.count++; }
exports.bump = bump;

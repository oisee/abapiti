// Match parentheses across lines, ignoring strings, escapes, and line comments.
exports.foldRanges = (text) => {
  const stack = [];
  const ranges = new Map();
  let line = 0, string = false, comment = false, escaped = false;
  for (const char of text) {
    if (char === '\n') {
      line++;
      comment = false;
      escaped = false;
      continue;
    }
    if (comment) continue;
    if (string) {
      if (escaped) escaped = false;
      else if (char === '\\') escaped = true;
      else if (char === '"') string = false;
      continue;
    }
    if (char === ';') comment = true;
    else if (char === '"') string = true;
    else if (char === '(') stack.push(line);
    else if (char === ')' && stack.length) {
      const start = stack.pop();
      // VS Code permits only one folding range starting on each line.
      if (line > start) ranges.set(start, Math.max(line, ranges.get(start) ?? line));
    }
  }
  return [...ranges].sort(([a], [b]) => a - b);
};

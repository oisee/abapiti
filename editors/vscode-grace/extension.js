const vscode = require('vscode');
const { foldRanges } = require('./folding');

exports.activate = (context) => {
  context.subscriptions.push(vscode.languages.registerFoldingRangeProvider('grace', {
    provideFoldingRanges(document) {
      return foldRanges(document.getText()).map(([start, end]) =>
        new vscode.FoldingRange(start, end));
    }
  }));
};

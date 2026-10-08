function applyOne(x: number): number { return x + 1; }
function applyTwo(x: number): number { return x + 2; }
const Edits = {
  applyOne,
  applyTwo,
};
export class Probe { public run(): number { return applyOne(1); } }
export {Edits};

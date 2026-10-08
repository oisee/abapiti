import * as Defs from "./defs";
export class Probe { public run(): number { return new Defs.Alpha().name().length + Defs.table.length; } }
export {Defs};

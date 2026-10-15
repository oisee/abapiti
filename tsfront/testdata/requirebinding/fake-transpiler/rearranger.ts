import {Kind} from "../fake-core/kinds.ts";

export class Rearranger {
  run(kinds: Kind[]): string {
    const names: string[] = [];
    for (const k of kinds) {
      names.unshift(k.name());
    }
    return names.join(">");
  }
}

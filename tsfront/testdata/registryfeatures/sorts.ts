interface SortMetadata { key: string; }
export class SortItem {
  constructor(public key: string, public id: string) {}
  public getMetadata(): SortMetadata { return {key: this.key}; }
}
export class SortFile {
  constructor(private filename: string, public id: string) {}
  public getFilename(): string { return this.filename; }
}
export class SortProbe {
  public static ruleKeys(): string {
    const first = new SortItem("a", "first");
    const second = new SortItem("a", "second");
    const underscore = new SortItem("_", "underscore");
    const rules: SortItem[] = [second, new SortItem("a_", "punctuation"), new SortItem("7bit_ascii", "digit"), first, underscore, new SortItem("__" + "proto__", "proto")];
    const alias = rules;
    const sorted = rules.sort((a, b) => a.getMetadata().key.localeCompare(b.getMetadata().key));
    let ids = "";
    for (const item of sorted) { ids += item.id + ","; }
    return `${sorted === alias}/${alias[0] === underscore}/${ids}`;
  }
  public static objectNames(): string {
    const objects: SortItem[] = [new SortItem("ZCL_TEST", "z"), new SortItem("/IWBEP/IF_TEST", "namespace"), new SortItem("A_", "underscore"), new SortItem("A/", "slash"), new SortItem("A0", "digit")];
    const sorted = objects.sort((a, b) => a.key.localeCompare(b.key));
    let ids = "";
    for (const item of sorted) { ids += item.id + ","; }
    return ids;
  }
  public static fileSequence(): string {
    const files: SortFile[] = [new SortFile("z.clas.testclasses.abap", "test"), new SortFile("z.unknown.abap", "unknown1"), new SortFile("z.clas.abap", "main"), new SortFile("z.clas.locals_def.abap", "def1"), new SortFile("x.other.abap", "unknown2"), new SortFile("z.clas.locals_imp.abap", "imp"), new SortFile("x.clas.locals_def.abap", "def2")];
    const sequence = [".clas.locals_def.abap", ".clas.locals_imp.abap", ".clas.abap", ".clas.testclasses.abap"];
    const sorted = files.slice().sort((a, b) => {
      const aValue = sequence.findIndex(s => a.getFilename().endsWith(s));
      const bValue = sequence.findIndex(s => b.getFilename().endsWith(s));
      return aValue - bValue;
    });
    let ids = "";
    for (const item of sorted) { ids += item.id + ","; }
    return `${files[0].id}/${ids}`;
  }
  public static rejected(): string[] {
    const invalid: string[] = ["é", "a"];
    return invalid.sort((a, b) => a.localeCompare(b));
  }
}

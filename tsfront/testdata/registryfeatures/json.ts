export interface GlobalSettings {
  files: any;
  skipGeneratedFunctionGroups?: boolean;
  skipIncludesWithoutMain?: boolean;
  errorOnDuplicateFilenames?: boolean;
}
export interface SyntaxSettings {
  version?: any;
  errorNamespace: string;
  globalConstants?: string[];
  ambigiousVoids?: string[];
  globalMacros?: string[];
}
export interface Dependency { url?: string; folder?: string; files: string; }
export interface ConfigGraph {
  global: GlobalSettings;
  syntax: SyntaxSettings;
  dependencies?: Dependency[];
  rules: any;
  targetRules?: any;
}
export class JSONProbe {
  public static parse(text: string): unknown { throw new Error("external JSON adapter"); }
  public static config(text: string): ConfigGraph { throw new Error("external typed JSON adapter"); }
  public static defaults(text: string): string {
    const config = JSONProbe.config(text);
    const syntax = config.syntax;
    const global = config.global;
    if (syntax.globalMacros === undefined) { syntax.globalMacros = []; }
    if (syntax.globalConstants === undefined) { syntax.globalConstants = []; }
    else { syntax.globalConstants = [...new Set(syntax.globalConstants)]; }
    if (syntax.ambigiousVoids === undefined) { syntax.ambigiousVoids = []; }
    else { syntax.ambigiousVoids = [...new Set(syntax.ambigiousVoids)]; }
    if (global.skipIncludesWithoutMain === undefined) { global.skipIncludesWithoutMain = false; }
    if (global.errorOnDuplicateFilenames === undefined) { global.errorOnDuplicateFilenames = false; }
    return `${global.files}/${syntax.version}/${syntax.errorNamespace}/${syntax.globalConstants.join(",")}/${syntax.ambigiousVoids.join(",")}/${syntax.globalMacros.length}/${global.skipIncludesWithoutMain}/${global.errorOnDuplicateFilenames}/${config.rules.unknown_rule}/${config.targetRules === null}`;
  }
  public static observe(text: string): string {
    const value: any = JSONProbe.parse(text);
    return `${value.global.files}/${value.syntax.version}/${value.rules.unknown_rule}/${value.list[1]}/${value.n === null}/${value.missing === undefined}`;
  }
}

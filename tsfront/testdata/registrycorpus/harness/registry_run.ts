import {Registry} from "../src/registry";
import {Config} from "../src/config";
import {MemoryFile} from "../src/files/memory_file";
import {Issue} from "../src/issue";
import {RulesRunner} from "../src/rules_runner";
import {ABAPObject} from "../src/objects/_abap_object";
import {SyntaxLogic} from "../src/abap/5_syntax/syntax";
import {CrossIncludeMacros} from "../src/abap/cross_include_macros";
import {FindGlobalDefinitions} from "../src/abap/5_syntax/global_definitions/find_global_definitions";

// Deployment harness, run unchanged by Node (the oracle) and by the
// translated ABAP: add the files, parse, find issues, print them in order.
// parse and report repeat the steps of Registry.parse() and
// RulesRunner.runRules() so every stage can be timed; the issues are the same.
export class RegistryRun {
  private readonly files: MemoryFile[] = [];
  private readonly dependencies: MemoryFile[] = [];
  private stages = "";

  public addFile(filename: string, raw: string): void {
    this.files.push(new MemoryFile(filename, raw));
  }

  public addDependency(filename: string, raw: string): void {
    this.dependencies.push(new MemoryFile(filename, raw));
  }

  public run(config: string): string {
    return this.report(this.parse(config));
  }

  // milliseconds per stage of the last parse and report
  public timings(): string {
    return this.stages;
  }

  public parse(config: string): Registry {
    const reg = new Registry(new Config(config));
    reg.addFiles(this.files);
    reg.addDependencies(this.dependencies);
    const conf = reg.getConfig();
    let lexer = 0;
    let statements = 0;
    let structures = 0;
    const start = Date.now();
    for (const o of reg.getObjects()) {
      const result = o.parse(conf.getRelease(), conf.getSyntaxSetttings().globalMacros, reg, conf.getLanguageVersion());
      if (result.runtimeExtra !== undefined) {
        lexer += result.runtimeExtra.lexing;
        statements += result.runtimeExtra.statements;
        structures += result.runtimeExtra.structure;
      }
    }
    const objects = Date.now();
    new CrossIncludeMacros(reg).run();
    const macros = Date.now();
    new FindGlobalDefinitions(reg).run();
    const globals = Date.now();
    this.stages = `lexer=${lexer} statements=${statements} structures=${structures} objects=${objects - start} macros=${macros - objects} globals=${globals - macros}`;
    return reg;
  }

  public report(reg: Registry): string {
    const runner = new RulesRunner(reg);
    const rules = reg.getConfig().getEnabledRules();
    const check = runner.objectsToCheck(reg.getObjects());
    const start = Date.now();
    for (const obj of check) {
      if (obj instanceof ABAPObject) {
        new SyntaxLogic(reg, obj).run();
      }
    }
    const syntax = Date.now();
    const times: number[] = [];
    for (const rule of rules) {
      const before = Date.now();
      rule.initialize(reg);
      times.push(Date.now() - before);
    }
    const found: Issue[] = [];
    for (const obj of check) {
      for (let index = 0; index < rules.length; index++) {
        const before = Date.now();
        found.push(...rules[index].run(obj));
        times[index] = times[index] + Date.now() - before;
      }
    }
    const issues = runner.excludeIssues(found);
    let ruleTimes = "";
    for (let index = 0; index < rules.length; index++) {
      ruleTimes += ` ${rules[index].getMetadata().key}=${times[index]}`;
    }
    this.stages += ` syntax=${syntax - start}${ruleTimes}`;
    let out = `${issues.length}`;
    for (const i of issues) {
      out += `\n${i.getKey()}|${i.getSeverity()}|${i.getFilename()}|${i.getStart().getRow()}:${i.getStart().getCol()}|${i.getEnd().getRow()}:${i.getEnd().getCol()}|${i.getMessage()}`;
    }
    return out;
  }
}

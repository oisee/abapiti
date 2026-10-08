import {Registry} from "../src/registry";
import {Config} from "../src/config";
import {MemoryFile} from "../src/files/memory_file";

// Deployment harness, run unchanged by Node (the oracle) and by the
// translated ABAP: add the files, parse, find issues, print them in order.
export class RegistryRun {
  private readonly files: MemoryFile[] = [];
  private readonly dependencies: MemoryFile[] = [];

  public addFile(filename: string, raw: string): void {
    this.files.push(new MemoryFile(filename, raw));
  }

  public addDependency(filename: string, raw: string): void {
    this.dependencies.push(new MemoryFile(filename, raw));
  }

  public run(config: string): string {
    const reg = new Registry(new Config(config)).addFiles(this.files).addDependencies(this.dependencies);
    reg.parse();
    const issues = reg.findIssues();
    let out = `${issues.length}`;
    for (const i of issues) {
      out += `\n${i.getKey()}|${i.getSeverity()}|${i.getFilename()}|${i.getStart().getRow()}:${i.getStart().getCol()}|${i.getEnd().getRow()}:${i.getEnd().getCol()}|${i.getMessage()}`;
    }
    return out;
  }
}

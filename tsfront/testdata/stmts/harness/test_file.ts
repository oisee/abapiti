// abapiti's differential-test driver, see README.md. Not part of abaplint.
import {IFile} from "../src/files/_ifile";

export class TestFile implements IFile {
  private readonly raw: string;
  private readonly filename: string;

  public constructor(raw: string, filename: string = "ztest.prog.abap") {
    this.raw = raw;
    this.filename = filename;
  }

  public getFilename(): string {
    return this.filename;
  }

  public getObjectType(): string | undefined {
    return "PROG";
  }

  public getObjectName(): string {
    return "ZTEST";
  }

  public getRaw(): string {
    return this.raw;
  }

  public getRawRows(): string[] {
    return this.raw.split("\n");
  }
}

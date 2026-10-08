// abapiti's differential-test driver, see README.md. Not part of abaplint.
import {IFile} from "../src/files/_ifile";

export class TestFile implements IFile {
  private readonly raw: string;

  public constructor(raw: string) {
    this.raw = raw;
  }

  public getFilename(): string {
    return "ztest.prog.abap";
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
    return [];
  }
}

import {MemoryFile} from "../src/files/memory_file";

export class MemoryFileDump {
  public static dumpSplit(raw: string, separator: string): string {
    const rows = raw.split(separator);
    let out = rows.length.toString();
    for (const row of rows) {
      out = out + "\n" + row.length.toString() + ":" + row;
    }
    return out;
  }

  public static dump(filename: string, raw: string): string {
    const file = new MemoryFile(filename, raw);
    if (!(file instanceof MemoryFile)) {
      throw new Error("MemoryFile constructor lost identity");
    }
    let out = file.getFilename() + "\n" + file.getObjectName() + "\n"
      + (file.getObjectType() ?? "<undefined>") + "\n" + file.getRaw();
    const rows = file.getRawRows();
    out = out + "\n" + rows.length.toString();
    for (const row of rows) {
      out = out + "\n" + row.length.toString() + ":" + row;
    }
    return out;
  }
}

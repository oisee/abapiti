// Preload (node --import) for the reference run of an ABAP-hosted program:
// records what the host boundary observed, for ZCL_ABAPITI_HOSTIO, which has
// no directory listing, environment or git on ABAP. Writes FS_SNAPSHOT_OUT at
// exit (format: docs/extern-modules.md, "fs-snapshot.json"). Program output
// is unchanged: every patched call returns and throws exactly as before.
import fs from 'node:fs';
import childProcess from 'node:child_process';
import path from 'node:path';
import {syncBuiltinESMExports} from 'node:module';

const out = process.env.FS_SNAPSHOT_OUT;
if (out) {
  const dirs = {}, env = {}, gitHead = {};
  const readdirSync = fs.readdirSync;
  // Only the two call shapes of the host boundary are recorded: names, and
  // {withFileTypes: true}. Every other shape passes through untouched.
  fs.readdirSync = function (dir, options) {
    const typed = options !== null && typeof options === 'object' && options.withFileTypes === true && Object.keys(options).length === 1;
    if (options !== undefined && !typed) return readdirSync.apply(this, arguments);
    const key = path.resolve(String(dir));
    let entries;
    try {
      entries = readdirSync.call(this, dir, {withFileTypes: true});
    } catch (error) {
      dirs[key] ??= {error: String(error?.message ?? error)};
      throw error;
    }
    dirs[key] ??= entries.map(e => [e.name, e.isDirectory()]);
    return typed ? entries : entries.map(e => e.name);
  };
  const execFileSync = childProcess.execFileSync;
  childProcess.execFileSync = function (file, args, options) {
    if (file !== 'git' || !Array.isArray(args) || args.length !== 2 || args[0] !== 'rev-parse' || args[1] !== 'HEAD') {
      return execFileSync.apply(this, arguments);
    }
    const key = path.resolve(String(options?.cwd ?? '.'));
    try {
      const result = execFileSync.apply(this, arguments);
      gitHead[key] ??= String(result).trim();
      return result;
    } catch (error) {
      gitHead[key] ??= null;
      throw error;
    }
  };
  syncBuiltinESMExports();
  // Values are recorded only for names the program read.
  const real = process.env;
  process.env = new Proxy(real, {
    get(target, key) {
      const value = Reflect.get(target, key);
      if (typeof key === 'string' && !Object.hasOwn(env, key)) env[key] = value === undefined ? null : String(value);
      return value;
    },
  });
  const cwd = process.cwd(), stdout = real.FS_SNAPSHOT_STDOUT;
  process.on('exit', () => {
    delete env.FS_SNAPSHOT_OUT;
    delete env.FS_SNAPSHOT_STDOUT;
    const snapshot = {version: 1, cwd, ...(stdout ? {stdout} : {}), dirs, env, gitHead};
    fs.writeFileSync(out, JSON.stringify(snapshot, null, 1) + '\n');
  });
}

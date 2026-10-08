import {execFileSync} from "node:child_process";
import {mkdtempSync, rmSync, writeFileSync} from "node:fs";
import {tmpdir} from "node:os";
import {join} from "node:path";
import {strict as assert} from "node:assert";
import {verifyUpstream, upstreamPin} from "./statements-upstream.mjs";
const dir = mkdtempSync(join(tmpdir(), "oracle-pin-test-"));
try {
 execFileSync("git",["init","-q",dir]);
 writeFileSync(join(dir,"source.ts"),"export class Pretend {}\n");
 execFileSync("git",["-C",dir,"add","source.ts"]);
 execFileSync("git",["-C",dir,"-c","user.name=Test","-c","user.email=test@example.invalid","commit","-qm","wrong upstream"]);
 assert.throws(()=>verifyUpstream(dir), new RegExp(upstreamPin));
 console.log("oracle provenance: rejects a build source at the wrong pin");
} finally { rmSync(dir,{recursive:true,force:true}); }

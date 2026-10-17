#!/usr/bin/env node
// Replays a program under the inspector and records which breakpoints it hits
// first, in execution order (each breakpoint is removed on its first hit).
// usage: selfhost-step4-order.mjs <breakpoints.json> <hits.txt> <stdout-file> <script> [args...]
// breakpoints.json: [{id, url, line, column}] with 0-based line/column (UTF-16).
import {spawn} from 'node:child_process';
import {readFileSync, appendFileSync, openSync} from 'node:fs';
const [bpFile, hitsFile, stdoutFile, ...command] = process.argv.slice(2);
const breakpoints = JSON.parse(readFileSync(bpFile, 'utf8'));
const child = spawn(process.execPath, ['--inspect-brk=127.0.0.1:0', ...command], {stdio: ['ignore', openSync(stdoutFile, 'w'), 'pipe']});
let err = '', socket;
const exited = new Promise(ok => child.on('exit', code => ok(code)));
child.stderr.on('data', chunk => {
  const text = chunk.toString();
  err += text;
  if (!socket && /ws:\/\/\S+/.test(err)) connect(err.match(/ws:\/\/\S+/)[0]);
  // After the program ends, Node keeps running until the debugger detaches.
  if (/Waiting for the debugger to disconnect/.test(err)) socket?.close();
  if (!/Debugger (listening|attached)|For help, see|Waiting for the debugger/.test(text)) process.stderr.write(text);
});
function connect(url) {
  socket = new WebSocket(url);
  let next = 1;
  const pending = new Map(), sites = new Map();
  const post = (method, params = {}) => new Promise((ok, fail) => { const id = next++; pending.set(id, {ok, fail}); socket.send(JSON.stringify({id, method, params})); });
  socket.onmessage = ({data}) => {
    const m = JSON.parse(data);
    if (m.id && pending.has(m.id)) { const p = pending.get(m.id); pending.delete(m.id); m.error ? p.fail(new Error(m.error.message)) : p.ok(m.result); return; }
    if (m.method === 'Debugger.paused') {
      for (const id of m.params.hitBreakpoints ?? []) {
        if (!sites.has(id)) continue;
        appendFileSync(hitsFile, sites.get(id) + '\n');
        sites.delete(id);
        post('Debugger.removeBreakpoint', {breakpointId: id});
      }
      post('Debugger.resume');
    }
  };
  socket.onopen = () => start().catch(e => { process.stderr.write(`inspector: ${e.message}\n`); child.kill(); });
  const start = async () => {
    await post('Debugger.enable');
    // Sites sharing a location share one breakpoint and are hit together.
    const grouped = new Map();
    for (const b of breakpoints) {
      const key = JSON.stringify([b.url, b.line, b.column]);
      grouped.set(key, [...(grouped.get(key) ?? []), b.id]);
    }
    for (const [key, ids] of grouped) {
      const [url, lineNumber, columnNumber] = JSON.parse(key);
      const r = await post('Debugger.setBreakpointByUrl', {url, lineNumber, columnNumber});
      sites.set(r.breakpointId, ids.join(' '));
    }
    await post('Runtime.runIfWaitingForDebugger');
  };
  socket.onerror = e => process.stderr.write(`inspector: ${e.message ?? e}\n`);
}
process.exitCode = await exited;
socket?.close();

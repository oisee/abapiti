// Admission control, not a substitute for the caller's shared OSG flock.
import {readFileSync} from 'node:fs';
import {loadavg} from 'node:os';
import {setTimeout as sleep} from 'node:timers/promises';

export function resourceSample() {
 const io = readFileSync('/proc/pressure/io', 'utf8').match(/^some .*?avg10=([\d.]+)/m);
 const memory = readFileSync('/proc/meminfo', 'utf8').match(/^MemAvailable:\s+(\d+) kB/m);
 if (!io || !memory) throw new Error('OSG admission requires Linux IO pressure and MemAvailable');
 return {load:loadavg()[0], io:Number(io[1]), availableGiB:Number(memory[1]) / 1048576};
}
export function resourcesReady({load, io, availableGiB}) {
 return [load,io,availableGiB].every(Number.isFinite) && load < 12 && io < 30 && availableGiB >= 20;
}
export async function waitForResources({sample=resourceSample, pause=sleep, timeoutMs=1800000, log=console.error} = {}) {
 const started = Date.now();
 let warned = false;
 while (true) {
  const resources = sample();
  if (resourcesReady(resources)) return {waitMs:Date.now()-started, resources};
  if (!warned) { log(`OSG waiting for load < 12, IO avg10 < 30, MemAvailable >= 20 GiB: ${JSON.stringify(resources)}`); warned = true; }
  if (Date.now()-started >= timeoutMs) throw new Error('OSG resource admission timed out');
  await pause(5000);
 }
}
// Opt-in hard limits for one heavy OSG step: an exclusive flock shared with
// other sessions, a cgroup memory ceiling, and lower CPU priority. Do not set
// ABAPITI_OSG_LOCK when the caller already holds the same lock (deadlock).
export function heavyCommand(command, args, env = process.env) {
 let argv = [command, ...args];
 if (env.ABAPITI_OSG_MEMORY_MAX) argv = ['systemd-run', '--user', '--scope', '-q', '-p', `MemoryMax=${env.ABAPITI_OSG_MEMORY_MAX}`, 'nice', '-n', env.ABAPITI_OSG_NICE ?? '10', ...argv];
 else if (env.ABAPITI_OSG_NICE) argv = ['nice', '-n', env.ABAPITI_OSG_NICE, ...argv];
 if (env.ABAPITI_OSG_LOCK) argv = ['flock', env.ABAPITI_OSG_LOCK, ...argv];
 return {command:argv[0], args:argv.slice(1)};
}

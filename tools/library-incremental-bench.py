#!/usr/bin/env python3
"""Alternate persistent before/after consumers on the same frozen M1 manifest.
Build bench.go.txt with outline-before/after.go.txt respectively. Usage:
  library-incremental-bench.py BEFORE AFTER MANIFEST OUTPUT [--gc]
Timing excludes completion of the previous lazy restore and runtime.ReadMemStats.
Each broken request includes Update(overlay), CheckObject, Update(restore).
"""
import json
import os
import shutil
import statistics
import subprocess
import sys
import time

before, after, manifest, output = sys.argv[1:5]
root = os.path.dirname(os.path.abspath(output))


def guard():
    if shutil.disk_usage(root).free < 45 * 10**9:
        raise RuntimeError('less than 45 GB free')
    while os.getloadavg()[0] >= 12:
        print('waiting for load < 12', flush=True)
        time.sleep(10)


class Server:
    def __init__(self, binary, label, gogc='100'):
        guard()
        self.label = label
        env = dict(os.environ, GOMAXPROCS='2', GOGC=gogc)
        self.err = open(os.path.join(root, label + '-stderr.log'), 'w')
        self.proc = subprocess.Popen([binary, manifest, os.path.join(root, label + '.cpu')],
                                     stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                     stderr=self.err, text=True, env=env)
        if self.read() != {'Ready': True}:
            raise RuntimeError('server not ready')

    def read(self):
        line = self.proc.stdout.readline()
        if not line:
            raise RuntimeError(self.label + ' exited; inspect stderr')
        return json.loads(line)

    def call(self, command):
        self.proc.stdin.write(command + '\n')
        self.proc.stdin.flush()
        return self.read()

    def peak_rss_kib(self):
        with open('/proc/' + str(self.proc.pid) + '/status') as status:
            for line in status:
                if line.startswith('VmHWM:'):
                    return int(line.split()[1])
        raise RuntimeError('missing peak RSS')

    def close(self):
        self.proc.stdin.close()
        if self.proc.wait() != 0:
            raise RuntimeError(self.label + ' failed')
        self.err.close()


servers = []
try:
    gc = '--gc' in sys.argv
    a = Server(after if gc else before, 'gc100' if gc else 'before')
    servers.append(a)
    b = Server(after, 'gc200' if gc else 'after', '200' if gc else '100')
    servers.append(b)
    commands = ['broken', 'clean', 'tree', 'snapshot', 'outline', 'probe']
    for server in servers:
        for command in commands:
            for _ in range(2):
                server.call(command)
    rows = []
    with open(output, 'w') as out:
        for i in range(20):
            for command in commands:
                for server in servers if i % 2 == 0 else servers[::-1]:
                    guard()
                    row = dict(server.call(command), Variant=server.label, Run=i,
                               Load=os.getloadavg()[0])
                    if command in ('broken', 'probe') and row['Issues'] != 1:
                        raise RuntimeError('expected exactly the first-method undefined issue')
                    rows.append(row)
                    out.write(json.dumps(row) + '\n')
                    out.flush()
            print('completed pair', i + 1, flush=True)
    for server in servers:
        server.call('profile')
        for _ in range(5):
            server.call('broken')
        server.call('stop')
    summary = []
    for server in servers:
        for command in commands:
            selected = [r for r in rows if r['Variant'] == server.label and r['Command'] == command]
            summary.append(dict(Variant=server.label, Command=command, Runs=len(selected),
                                MedianMS=statistics.median(r['Milliseconds'] for r in selected),
                                ProcessPeakRSSKiB=server.peak_rss_kib(),
                                MedianAllocBytes=statistics.median(r['AllocBytes'] for r in selected),
                                MedianMallocs=statistics.median(r['Mallocs'] for r in selected),
                                TotalGCs=sum(r['GCs'] for r in selected),
                                Parts=[statistics.median(r['Parts'][i] for r in selected)
                                       for i in range(len(selected[0]['Parts']))],
                                Stats=selected[0]['Stats']))
    with open(output + '.summary.json', 'w') as out:
        json.dump(summary, out, indent=2)
    print(json.dumps(summary, indent=2))
finally:
    for server in servers:
        server.close()

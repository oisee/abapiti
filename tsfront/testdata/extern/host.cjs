const fs=require('node:fs'), p=require('node:path').posix, cp=require('node:child_process');
exports.readText=f=>fs.readFileSync(f,'utf8');
exports.writeText=(f,s)=>fs.writeFileSync(f,s,'utf8');
exports.listDir=d=>fs.readdirSync(d).sort((a,b)=>Buffer.compare(Buffer.from(a),Buffer.from(b)));
exports.listEntries=d=>exports.listDir(d).map(n=>[n,fs.lstatSync(p.join(d,n)).isDirectory()]);
exports.basename=p.basename;exports.dirname=p.dirname;exports.join=p.join;
exports.gitHead=d=>{try{return cp.execFileSync('git',['rev-parse','HEAD'],{cwd:d,stdio:['ignore','pipe','ignore']}).toString().trim()}catch{return undefined}};
exports.env=n=>process.env[n];

#include <stdio.h>
#include <string.h>
#include "quickjs.h"

static const char *progs[] = {
  "1+2",
  "let s=0; for (let i=1;i<=100;i++) s+=i; s",
  "[5,3,9,1].sort((a,b)=>a-b).join('').length*1000 + [5,3,9,1].sort((a,b)=>a-b)[3]",
  "JSON.stringify({a:[1,2,{b:3}]}).length",
  "(function f(n){return n<2?n:f(n-1)+f(n-2)})(15)",
  "'abapiti'.toUpperCase().charCodeAt(0)",
  "Math.floor(Math.sqrt(1e6))",
  "new Map([[1,'x'],[2,'y']]).size + /a+b/.exec('xaaab')[0].length*10",
  "(function f(n){return n<2?n:f(n-1)+f(n-2)})(22)",
  "let s=0; for (let i=0;i<1000000;i++) s=(s+i*7)%1000003; s",
  "let s=''; for (let i=0;i<20000;i++) s+=String.fromCharCode(97+i%26); s.length + s.charCodeAt(12345)",
  "let a=[]; let x=12345; for (let i=0;i<10000;i++){x=(x*1103515245+12345)%2147483648; a.push(x%100000);} a.sort((p,q)=>p-q); let c=0; for (let i=0;i<a.length;i+=97) c=(c*31+a[i])%1000000007; c",
  "let t=0; for (let i=0;i<20000;i++){ let o={a:i,b:i*2,c:{d:i%7}}; t=(t+o.a+o.b+o.c.d)%1000003; } t",
  "function mk(){let n=0; return ()=>++n;} let f=mk(); let r=0; for (let i=0;i<100000;i++) r=f(); r",
  "let a=[]; for (let i=0;i<1000;i++) a.push({i:i,s:'v'+i,f:[i,i*2]}); let j=JSON.stringify(a); let b=JSON.parse(j); j.length + b[999].f[1]",
  "let s=''; for (let i=0;i<2000;i++) s+='ab'+(i%10)+'cd '; let m=s.match(/b[0-9]c/g); m.length + s.replace(/[0-9]/g,'').length",
};

__attribute__((export_name("qjs_eval")))
int qjs_eval(int which) {
  if (which < 0 || which >= (int)(sizeof progs / sizeof progs[0])) return -1;
  JSRuntime *rt = JS_NewRuntime();
  JSContext *ctx = JS_NewContext(rt);
  const char *src = progs[which];
  JSValue v = JS_Eval(ctx, src, strlen(src), "<eval>", JS_EVAL_TYPE_GLOBAL);
  int r = -2;
  if (!JS_IsException(v)) JS_ToInt32(ctx, &r, v);
  JS_FreeValue(ctx, v);
  JS_FreeContext(ctx);
  JS_FreeRuntime(rt);
  return r;
}

__attribute__((export_name("qjs_print")))
int qjs_print(int n) {
  return printf("abapiti says %d\n", n);
}

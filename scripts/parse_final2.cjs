const fs = require('fs');
const input = fs.readFileSync(process.argv[2], 'utf-8');

// Find the defaultSites block
const startMark = 'var defaultSites = ';
const idx = input.indexOf(startMark);
if (idx === -1) { console.error('not found'); process.exit(1); }

let raw = input.substring(idx + startMark.length);

// Find matching closing brace
function findCloseBrace(text, start) {
  let depth = 1;
  let i = start;
  let inStr = false, inRE = false, quote = '';
  while (i < text.length && depth > 0) {
    const c = text[i], p = i > 0 ? text[i-1] : '';
    if (inStr) { if (c === quote && p !== '\\') inStr = false; }
    else if (inRE) { if (c === '/' && p !== '\\') inRE = false; }
    else {
      if ('\"\''.includes(c)) { inStr = true; quote = c; }
      else if (c === '/') {
        const before = text.substring(Math.max(0,i-1),i).replace(/\s+$/,'');
        if (before === '' || ',:=([{!&|?'.includes(before.slice(-1))) inRE = true;
      }
      else if (c === '{') depth++;
      else if (c === '}') {
        depth--;
        if (depth === 0) return i;
      }
    }
    i++;
  }
  return -1;
}

const end = findCloseBrace(raw, 0);
if (end === -1) { console.error('no match'); process.exit(1); }

raw = raw.substring(0, end + 1);

// Now convert JS object literal to JSON
function js2json(t) {
  let out = '', i = 0;
  while (i < t.length) {
    const c = t[i], n = t[i+1] || '';
    if (c === '/' && n === '/') { while (i < t.length && t[i] !== '\n') i++; i++; continue; }
    if (c === '/' && n === '*') { i+=2; while (i+1 < t.length && !(t[i]==='*'&&t[i+1]==='/')) i++; i+=2; continue; }
    
    if ('\"\''.includes(c)) {
      let s = '', q = c; i++;
      while (i < t.length) {
        if (t[i] === '\\') { s += t[i] + (t[i+1]||''); i+=2; continue; }
        if (t[i] === q) { i++; break; }
        s += t[i]; i++;
      }
      out += JSON.stringify(s);
      continue;
    }
    
    if (c === '/' && i > 0) {
      const b = out.replace(/\s+$/,'').slice(-1) || '';
      if ('\0,:=([{!&|?'.includes(b)) {
        let p = ''; i++;
        while (i < t.length && t[i] !== '/') {
          if (t[i] === '\\') { p += t[i] + (t[i+1]||''); i+=2; }
          else { p += t[i]; i++; }
        }
        if (i < t.length) i++; // closing /
        let flags = '';
        while (i < t.length && /[gimsuy]/.test(t[i])) { flags += t[i]; i++; }
        out += JSON.stringify({__r__: p, __f__: flags});
        continue;
      }
    }
    
    if (/[a-zA-Z_$]/.test(c)) {
      let w = '';
      while (i < t.length && /\w/.test(t[i])) { w += t[i]; i++; }
      const ws = t.substring(i).match(/^(\s*)/)[1].length;
      if (t[i+ws] === ':') {
        out += JSON.stringify(w); continue;
      }
      out += ({true:'true',false:'false',null:'null',undefined:'null'}[w] || JSON.stringify(w));
      continue;
    }
    
    if (c === ',') {
      let j = i+1; while (j < t.length && t[j] <= ' ') j++;
      if (t[j] === '}' || t[j] === ']') { i = j; continue; }
    }
    
    out += c; i++;
  }
  return out;
}

const json = js2json(raw);
let parsed;
try { parsed = JSON.parse(json); }
catch(e) {
  console.error('JSON Error:', e.message);
  const m = e.message.match(/position (\d+)/);
  const p = m ? parseInt(m[1]) : 0;
  console.error(json.substring(Math.max(0,p-100),Math.min(json.length,p+100)));
  process.exit(1);
}

const out = {};
for (const [k,v] of Object.entries(parsed)) {
  if (k.startsWith('*') || k.startsWith('#') || !v||!v.domain||v.domain.startsWith('#')) continue;
  out[k] = v;
}
console.log(JSON.stringify(out, null, 2));

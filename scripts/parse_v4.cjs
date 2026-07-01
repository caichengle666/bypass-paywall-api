const fs = require('fs');
const input = fs.readFileSync(process.argv[2], 'utf-8');

// Extract top-level var declarations
function extractVarBlock(text, varName) {
  const re = new RegExp('var\\s+' + varName + '\\s*=');
  const m = text.match(re);
  if (!m) return null;
  let pos = m.index + m[0].length;
  // Skip to opening brace
  while (pos < text.length && text[pos] !== '{') pos++;
  if (pos >= text.length) return null;
  
  let depth = 1;
  let start = pos;
  pos++;
  while (pos < text.length && depth > 0) {
    const c = text[pos];
    if (c === '"' || c === "'" || c === '') {
      const q = c; pos++;
      while (pos < text.length && text[pos] !== q) {
        if (text[pos] === '\\') pos++;
        pos++;
      }
    } else if (c === '/' && pos > 0 && '\0,:=([{!&|?'.includes(text[pos-1]||'')) {
      pos++;
      while (pos < text.length && text[pos] !== '/') {
        if (text[pos] === '\\') pos++;
        pos++;
      }
    } else if (c === '{') {
      depth++;
    } else if (c === '}') {
      depth--;
    }
    pos++;
  }
  return text.substring(start, pos);
}

const dsBlock = extractVarBlock(input, 'defaultSites');
const gsBlock = extractVarBlock(input, 'grouped_sites');

// Convert JS object to JSON
function js2json(t) {
  let out = '', i = 0;
  while (i < t.length) {
    const c = t[i];
    
    // Comments
    if (c === '/' && t[i+1] === '/') { while (i < t.length && t[i] !== '\n') i++; i++; continue; }
    if (c === '/' && t[i+1] === '*') { i+=2; while (i+1<t.length && !(t[i]=='*'&&t[i+1]=='/')) i++; i+=2; continue; }
    
    // Strings
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
    
    // Regex literal
    if (c === '/' && i > 0) {
      const before = out.replace(/\s+$/, '').slice(-1) || '';
      if ('\0,:=([{!&|?'.includes(before)) {
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
    
    // Identifiers (keys or keywords)
    if (/[a-zA-Z_$]/.test(c)) {
      let w = '';
      while (i < t.length && /\w/.test(t[i])) { w += t[i]; i++; }
      // Check if followed by ':'
      let j = i; while (j < t.length && t[j] <= ' ') j++;
      if (t[j] === ':') {
        out += JSON.stringify(w);
        continue;
      }
      // Keywords
      out += ({true:'true',false:'false',null:'null',undefined:'null'}[w] || JSON.stringify(w));
      continue;
    }
    
    // Trailing comma
    if (c === ',') {
      let j = i+1; while (j < t.length && t[j] <= ' ') j++;
      if (t[j] === '}' || t[j] === ']') { i = j; continue; }
    }
    
    out += c; i++;
  }
  return out;
}

let allSites = {};

for (const block of [dsBlock, gsBlock].filter(Boolean)) {
  const json = js2json(block);
  try {
    const parsed = JSON.parse(json);
    Object.assign(allSites, parsed);
  } catch(e) {
    console.error('Parse error for block:', e.message);
    const m = e.message.match(/position (\d+)/);
    const p = m ? parseInt(m[1]) : 0;
    const ctx = json.substring(Math.max(0,p-100), Math.min(json.length,p+100));
    console.error('Context:', ctx);
    continue;
  }
}

// Filter
const out = {};
for (const [k,v] of Object.entries(allSites)) {
  if (k.startsWith('*') || k.startsWith('#') || !v||!v.domain||v.domain.startsWith('#')||v.domain.startsWith('###')) continue;
  out[k] = v;
}

// Write to file and stdout
const jsonOut = JSON.stringify(out, null, 2);
console.log(jsonOut);

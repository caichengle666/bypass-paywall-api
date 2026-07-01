const fs = require('fs');
const path = require('path');

const input = fs.readFileSync(process.argv[2], 'utf-8');

// Step 1: extract brace-balanced block after each 'var X ='
function extractObject(text, keyword) {
  const idx = text.indexOf(keyword);
  if (idx === -1) return null;
  let pos = idx + keyword.length;
  while (pos < text.length && text[pos] !== '{') pos++;
  if (pos >= text.length) return null;
  
  let depth = 1;
  let start = pos;
  pos++;
  while (pos < text.length && depth > 0) {
    const c = text[pos];
    if (c === '{') depth++;
    else if (c === '}') depth--;
    // skip strings
    if (c === '"' || c === "'" || c === '') {
      const quote = c;
      pos++;
      while (pos < text.length && text[pos] !== quote) {
        if (text[pos] === '\\') pos++;
        pos++;
      }
    }
    // skip regex - simplified
    if (c === '/' && pos > 0 && ':,(= [!&|'.includes(text[pos-1])) {
      pos++;
      while (pos < text.length && text[pos] !== '/') {
        if (text[pos] === '\\') pos++;
        pos++;
      }
    }
    pos++;
  }
  return text.substring(start, pos);
}

const dsRaw = extractObject(input, 'var defaultSites =');
if (!dsRaw) { console.error('Cannot find defaultSites'); process.exit(1); }

// Now write it to a temp file and use child_process to evaluate it
// Actually, let me use a different approach - write the data to be loaded
// Use node:vm with proper sandbox
const vm = require('vm');

// Create sandbox
const sandbox = {};
// Add needed globals
const globalProps = ['Object','Array','String','Number','Boolean','RegExp','Date','Math','JSON','Error','TypeError','SyntaxError','parseInt','parseFloat','isNaN','NaN','Infinity','undefined','null','Map','Set','WeakMap','WeakSet','Symbol','Promise','Proxy','Reflect'];
for (const p of globalProps) {
  sandbox[p] = global[p];
}
sandbox.console = { log: ()=>{} };  // silent
sandbox.self = sandbox;
sandbox.globalThis = sandbox;
sandbox.window = sandbox;
sandbox.fetch = ()=>Promise.resolve({ok:false});
sandbox.setTimeout = ()=>0;
sandbox.setInterval = ()=>0;
sandbox.clearInterval = ()=>{};
sandbox.clearTimeout = ()=>{};

// Minimal URLPattern stub
sandbox.URLPattern = class { constructor(){} test(){return false} };

// Build the code to eval: just define defaultSites
let evalCode = 'var defaultSites = ' + dsRaw + ';\n';
// Also define grouped_sites if it exists
const gsRaw = extractObject(input, 'var grouped_sites =');
if (gsRaw) {
  evalCode += 'var grouped_sites = ' + gsRaw + ';\n';
}

try {
  vm.runInNewContext(evalCode, sandbox, { timeout: 5000 });
} catch(e) {
  console.error('VM eval error:', e.message);
  
  // Fallback: try to regex-convert to JSON
  console.error('Attempting regex fallback...');
  let converted = dsRaw
    .replace(/\/\/.*$/gm, '')
    .replace(/[^]*/g, m => JSON.stringify(m.slice(1,-1)))
    .replace(/'([^']*?)'/g, (m) => {
      if (m[1] === '{') return m; // skip template-like strings
      return '"' + m.slice(1,-1).replace(/"/g,'\\"') + '"';
    })
    .replace(/([{,]\s*)([a-zA-Z_$][a-zA-Z0-9_$]*)\s*:/g, '"":')
    .replace(/,\s*([}\]])/g, '')
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/\/([^\/\\\n]*(?:\\.[^\/\\\n]*)*)\/([gimsuy]*)/g, (m,p,f) => JSON.stringify({__regexp__:p,__flags__:f}))
    .replace(/;\s*$/,'');
  
  try {
    const parsed = JSON.parse(converted);
    const output = {};
    for (const [name, site] of Object.entries(parsed)) {
      if (name.startsWith('*') || name.startsWith('#')) continue;
      if (!site || typeof site !== 'object') continue;
      if (!site.domain || site.domain.startsWith('#')) continue;
      output[name] = site;
    }
    console.log(JSON.stringify(output, null, 2));
    process.exit(0);
  } catch(e2) {
    console.error('Fallback also failed:', e2.message);
    // Write debug file
    console.error('Writing debug file...');
    const line = parsed.substring(0,1000);
    console.error(line);
    process.exit(1);
  }
}

const ds = sandbox.defaultSites || {};
const gs = sandbox.grouped_sites || {};
const all = {...ds, ...gs};
const output = {};
for (const [name, site] of Object.entries(all)) {
  if (name.startsWith('*') || name.startsWith('#') || name.startsWith('###')) continue;
  if (!site || typeof site !== 'object') continue;
  if (!site.domain || site.domain.startsWith('#') || site.domain.startsWith('###')) continue;
  // Convert RegExp to serializable
  const clean = {};
  for (const [k,v] of Object.entries(site)) {
    if (v instanceof RegExp) {
      clean[k] = {__regexp__: v.source, __flags__: v.flags};
    } else {
      clean[k] = v;
    }
  }
  output[name] = clean;
}
console.log(JSON.stringify(output, null, 2));

const vm = require('vm');
const fs = require('fs');

const input = fs.readFileSync(process.argv[2], 'utf-8');

// 创建一个沙箱环境来执行 sites.js
const sandbox = {
  window: { navigator: { userAgent: 'Mozilla/5.0' } },
  document: {},
  console: { log: () => {}, error: () => {} },
  fetch: () => Promise.resolve({ ok: false, json: () => Promise.resolve({}) }),
  URLPattern: class {},
  setTimeout: () => {},
  setInterval: () => {},
  clearInterval: () => {},
  navigator: { userAgent: 'Mozilla/5.0' },
  location: { hostname: '', href: '' },
  // Chrome API stub
  chrome: {
    runtime: { getManifest: () => ({ key: 'test', version: '4.3.9.0', manifest_version: 3 }) },
    storage: {
      local: {
        get: () => Promise.resolve({}),
        set: () => Promise.resolve()
      }
    },
    declarativeNetRequest: {
      getDynamicRules: () => Promise.resolve([]),
      updateDynamicRules: () => Promise.resolve(),
      getSessionRules: () => Promise.resolve([]),
      updateSessionRules: () => Promise.resolve()
    }
  },
  browser: {},
  ext_api: null,
  self: {}
};

// 添加导出变量
let defaultSites = {};
let grouped_sites = {};

try {
  // 使用 contextified sandbox
  const ctx = vm.createContext(sandbox);
  
  // 预处理：移除 'var defaultSites' 和 'var grouped_sites' 以便我们捕获它们
  let processed = input
    .replace(/importScripts\s*\([^)]*\)/g, '')
    .replace(/\/\/\/ <reference[^>]*>/g, '')
    .replace(/\/\/\s*@ts-check/g, '');
  
  // 替换 var 为 globalThis 属性赋值  
  processed = processed.replace(/var\s+(defaultSites|grouped_sites)\s*=/g, 'globalThis. =');
  
  vm.runInContext(processed, ctx, { timeout: 5000 });
  
  defaultSites = ctx.defaultSites || {};
  grouped_sites = ctx.grouped_sites || {};
  
  // 合并
  let allSites = { ...defaultSites, ...grouped_sites };
  
  // 过滤特殊 key
  let output = {};
  for (let [name, site] of Object.entries(allSites)) {
    if (name.startsWith('###') || name.startsWith('*') || name.startsWith('#')) continue;
    if (!site || !site.domain) continue;
    if (site.domain.startsWith('###') || site.domain.startsWith('#')) continue;
    output[name] = site;
  }
  
  // 输出可序列化的 JSON (处理正则)
  console.log(JSON.stringify(output, (key, value) => {
    if (value instanceof RegExp) {
      return { __regex__: value.source, __flags__: value.flags };
    }
    return value;
  }, 2));
  
} catch (e) {
  console.error('VM Error:', e.message);
  process.exit(1);
}

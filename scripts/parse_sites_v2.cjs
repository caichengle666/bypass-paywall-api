const fs = require('fs');

// 更简单的转换：逐行解析 JS 对象结构为 JSON
function jsObjToJson(input) {
  // 移除注释
  let text = input.replace(/\/\/.*$/gm, '');
  
  // 处理正则文本: /pattern/flags -> {"__regexp__":"pattern","__flags__":"flags"}
  // 这个需要注意转义字符
  text = text.replace(/\/([^\/\\\n]*(?:\\.[^\/\\\n]*)*)\/([gimsuy]*)/g, 
    (match, pattern, flags) => {
      // 确认这不是除法符号 (前后有数字或变量)
      return JSON.stringify({__regexp__: pattern, __flags__: flags || ''});
    }
  );
  
  // 处理模板字符串: ...... -> "...{domain}..."
  text = text.replace(/([^]*)/g, (m, content) => {
    return '"' + content.replace(/\$\{domain\}/g, '{domain}').replace(/"/g, '\\"') + '"';
  });
  
  // 将 var/const/let X = 替换为 "X":
  text = text.replace(/(?:var|const|let)\s+(\w+)\s*=\s*/g, '"":');
  
  // 给未加引号的 key 加引号
  text = text.replace(/([{,]\s*)([a-zA-Z_$][a-zA-Z0-9_$]*)\s*:/g, '"":');
  
  // 移除 trailing commas
  text = text.replace(/,\s*([}\]])/g, '');
  
  // 移除末尾分号
  text = text.replace(/;\s*$/g, '');
  
  // 将 ' 替换为 " (只在值部分)
  // 简单方法: 将所有单引号替换为双引号
  text = text.replace(/'/g, '"');
  
  // 修复嵌套引导问题: "\"xxx\"" -> "xxx"
  text = text.replace(/""/g, '"');
  
  return text;
}

const input = fs.readFileSync(process.argv[2], 'utf-8');
let converted = jsObjToJson(input);

// 寻找从 defaultSites 开始的 JSON 对象
let startIdx = converted.indexOf('"defaultSites":');
if (startIdx === -1) {
  // 尝试找其他的
  startIdx = converted.indexOf('"grouped_sites":');
}
if (startIdx === -1) { console.error('Cannot find config start'); process.exit(1); }

// 从 { 开始
let braceIdx = converted.indexOf('{', startIdx);
if (braceIdx === -1) { console.error('Cannot find opening brace'); process.exit(1); }

// 尝试解析 JSON
try {
  // 先解析整个文件为一个大对象
  let fullJson = JSON.parse('{' + converted.substring(braceIdx));
  
  let defaultSites = fullJson.defaultSites || {};
  let grouped_sites = fullJson.grouped_sites || {};
  
  // 合并去重 (grouped_sites 优先)
  let allSites = { ...defaultSites, ...grouped_sites };
  
  // 过滤
  let output = {};
  for (let [name, site] of Object.entries(allSites)) {
    if (name.startsWith('###') || name.startsWith('*') || name.startsWith('#')) continue;
    if (!site || typeof site !== 'object') continue;
    if (!site.domain || site.domain.startsWith('###') || site.domain.startsWith('#')) continue;
    output[name] = site;
  }
  
  console.log(JSON.stringify(output, null, 2));
  
} catch (e) {
  console.error('Parse Error:', e.message);
  // 输出转换后的内容帮助调试
  const debugOut = '{' + converted.substring(braceIdx);
  const errPos = parseInt(e.message.match(/position (\d+)/)?.[1] || '0');
  const contextStart = Math.max(0, errPos - 100);
  const contextEnd = Math.min(debugOut.length, errPos + 100);
  console.error('Context around error:', debugOut.substring(contextStart, contextEnd));
  process.exit(1);
}

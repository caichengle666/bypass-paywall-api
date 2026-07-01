// sites_to_config.js - 将 BPC sites.js 转换为 Go 可读的 JSON 配置
// 用法: node sites_to_config.js < path/to/sites.js > config.json

const fs = require('fs');

// 读取 stdin 或文件参数
let input = fs.readFileSync(process.argv[2] || '/dev/stdin', 'utf-8');

// 提取 var defaultSites = {...} 或 const defaultSites = {...}
let match = input.match(/(?:var|const|let)\s+defaultSites\s*=\s*(\{[\s\S]*?\});\s*(?:\/\/|\n|$)/);
if (!match) {
  // 尝试找 grouped_sites
  match = input.match(/(?:var|const|let)\s+grouped_sites\s*=\s*(\{[\s\S]*?\});\s*(?:\/\/|\n|$)/);
}
if (!match) {
  console.error('ERROR: Could not find defaultSites or grouped_sites');
  process.exit(1);
}

let raw = match[1];

// 将 JS 对象转换为 JSON-like 结构
// 1. 处理注释
raw = raw.replace(/\/\/.*$/gm, '');
// 2. 处理 trailing commas
raw = raw.replace(/,\s*([}\]])/g, '');
// 3. 转换 key 名: 不加引号的 key 加引号
raw = raw.replace(/([{,]\s*)([a-zA-Z_$][a-zA-Z0-9_$]*)\s*:/g, '"":');
// 4. 转换正则 /pattern/flags -> {"__regex__":"pattern","flags":"flags"}
raw = raw.replace(/\/([^\/\\]*(?:\\.[^\/\\]*)*)\/([gimsuy]*)/g, (m, pattern, flags) => {
  return JSON.stringify({__regex__: pattern, __flags__: flags || ''});
});

// 5. 处理字符串模板 ${domain}
raw = raw.replace(/\$\{domain\}/g, '{domain}');

// 替换单引号为双引号 (但跳过已处理的部分)
// 更安全: 将未被 JSON 双引号包裹的单引号字符串转双引号
let lines = raw.split('\n');
let result = [];
for (let line of lines) {
  // 跳过已经是 JSON 字符串的行
  if (line.trim().startsWith('"__regex__"')) {
    result.push(line);
    continue;
  }
  // 将单引号字符值转义/替换
  let processed = line.replace(/'([^']*)'/g, (m, content) => {
    // 避免双引号嵌套
    let escaped = content.replace(/"/g, '\\"');
    return '"' + escaped + '"';
  });
  result.push(processed);
}
raw = result.join('\n');

try {
  let parsed = JSON.parse(raw);
  // 输出扁平化的站点列表 (name -> rules)
  let output = {};
  for (let [name, site] of Object.entries(parsed)) {
    if (name.startsWith('###') || name.startsWith('#')) continue;
    if (!site || !site.domain) continue;
    output[name] = site;
  }
  console.log(JSON.stringify(output, null, 2));
} catch (e) {
  console.error('Parse error:', e.message);
  // 输出原始内容辅助调试
  fs.writeFileSync('/tmp/sites_debug_raw.txt', raw.substring(0, 50000));
  process.exit(1);
}

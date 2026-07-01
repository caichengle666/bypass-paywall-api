# bypass-paywall-api

基于 [Bypass Paywalls Clean](https://gitflic.ru/project/magnolia1234/bpc_uploads) 扩展的 Go HTTP API，使用 headless Chrome 提取 456+ 付费新闻网站的全文内容。

## 快速开始

```bash
# 本地运行 (需要安装 Chrome)
go build -o bpc-api ./cmd/server
./bpc-api -browsers 2 -addr :8080 -proxy socks5://host:port

# Docker
docker build -t bpc-api .
docker run -d -p 8080:8080 -e BPC_API_KEY=mykey bpc-api -proxy socks5://host:port
```

## 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-addr` | `:8080` | 监听地址 |
| `-browsers` | `2` | 最大 Chrome 实例数 |
| `-chrome` | 自动查找 | Chrome 路径 |
| `-proxy` | 无 | SOCKS5 代理 (e.g. socks5://host:port) |
| `-api-key` | 无 | API 鉴权密钥 (或用环境变量 `BPC_API_KEY`) |
| `-config` | `config_sites.json` | BPC 站点配置 |

## API

### `POST /fetch/js`

获取文章全文或首页结构化内容。

**请求:**
```json
{ "url": "https://cn.wsj.com/articles/..." }
```

**文章页返回:**
```json
{
  "success": true,
  "title": "文章标题",
  "paragraphs": ["段1", "段2", ...],
  "full_text": "段1\n\n段2\n\n...",
  "source": "js_injection",
  "latency_ms": 30000,
  "sections": null
}
```

**首页/分类页返回:**
```json
{
  "success": true,
  "title": "页面标题",
  "sections": {
    "navigation": [{ "label": "中国", "href": "/zh-hans/news/china" }, ...],
    "sections": [
      { "section": "General", "articles": [{ "title": "文章标题", "url": "..." }, ...] }
    ],
    "total_articles": 30
  }
}
```

### `POST /fetch`
通过 HTML 解析提取（备选方案）。

### `GET /health`
健康检查。

### `GET /sites`
列出所有受支持的站点。

### `GET /sites/lookup?url=...`
检查 URL 是否受支持。

## 鉴权

设 `-api-key` 参数或环境变量 `BPC_API_KEY` 后，请求需带 Header:

```
Authorization: Bearer <your-key>
```

健康检查 `/health` 不需要鉴权。

## 支持的站点

基于 BPC 规则，覆盖 456+ 新闻网站，包括:

- wsj.com, nytimes.com, washingtonpost.com
- economist.com, ft.com, bloomberg.com
- barrons.com, axios.com, wired.com
- businessinsider.com, thetimes.com 等

## 项目结构

```
├── cmd/server/main.go        - 入口
├── internal/
│   ├── api/server.go         - HTTP API + 鉴权
│   ├── browser/pool.go       - Chrome 浏览器池
│   ├── config/               - 站点配置加载
│   ├── strategy/             - Referer/UA 策略
│   └── extract/              - HTML + JS 提取
├── bpc-src/                  - BPC 扩展源码
├── config_sites.json         - 456 站点规则
├── Dockerfile                - 多阶段构建
└── scripts/                  - 工具脚本
```

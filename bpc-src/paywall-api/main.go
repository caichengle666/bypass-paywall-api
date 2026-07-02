package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

var (
	bpcPath string
	allocCtx context.Context
	allocCancel context.CancelFunc
)

func init() {
	bpcPath = os.Getenv("BPC_EXTENSION_PATH")
	if bpcPath == "" {
		for _, c := range []string{
			"D:\\bypass-paywalls-chrome-clean-master",
			"/opt/bypass-paywalls-chrome-clean-master",
			"/app/plugin",
			".",
		} {
			if _, err := os.Stat(c + "/manifest.json"); err == nil {
				abs, _ := filepath.Abs(c)
				bpcPath = abs
				break
			}
		}
	}
	allocOpts := []chromedp.ExecAllocatorOption{
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("window-size", "1440,3000"),
	}
	if bpcPath != "" {
		allocOpts = append(allocOpts,
			chromedp.Flag("disable-extensions-except", bpcPath),
			chromedp.Flag("load-extension", bpcPath),
		)
	}
	if p := os.Getenv("PROXY_SERVER"); p != "" {
		allocOpts = append(allocOpts, chromedp.ProxyServer(p))
	}
	allocCtx, allocCancel = chromedp.NewExecAllocator(context.Background(), allocOpts...)
	if bpcPath != "" {
		log.Printf("BPC loaded: %s", bpcPath)
	} else {
		log.Println("WARNING: BPC not found")
	}
	log.Println("Chrome allocator ready")
}

type ArticleResponse struct {
	URL        string   `json:"url"`
	Title      string   `json:"title"`
	Text       string   `json:"text"`
	Paragraphs []string `json:"paragraphs"`
	Count      int      `json:"count"`
	Error      string   `json:"error,omitempty"`
}

const extractJS = `
(() => {
// 显示 paywall
document.querySelectorAll("div.paywall, div[class*=\"PaywalledContentContainer\"]").forEach(function(el) {
	el.hidden = false;
	el.removeAttribute("hidden");
	el.removeAttribute("aria-hidden");
	if (el.style) {
		el.style.display = "block";
		el.style.visibility = "visible";
		el.style.opacity = "1";
		el.style.maxHeight = "none";
		el.style.height = "auto";
		el.style.overflow = "visible";
	}
});

// 找文章容器
var article = document.querySelector("article section") || document.querySelector("article") || document.querySelector("main") || document.querySelector("[role=\"main\"]");
var paras = [];
if (article) {
	paras = Array.from(article.querySelectorAll("p"))
		.map(function(p) { return (p.innerText || p.textContent || "").replace(/\s+/g, " ").trim(); })
		.filter(Boolean)
		.filter(function(p) { return p.length >= 12 && p !== "/" && !p.startsWith("\u5e7f\u544a"); });
}

// 如果不够，从 paywall 容器找
if (paras.length < 5) {
	var pw = document.querySelector("div.paywall, div[class*=\"PaywalledContentContainer\"]");
	if (pw) {
		var extra = Array.from(pw.querySelectorAll("p"))
			.map(function(p) { return (p.innerText || p.textContent || "").replace(/\s+/g, " ").trim(); })
			.filter(Boolean)
			.filter(function(p) { return p.length >= 12; });
		if (extra.length > paras.length) {
			paras = extra;
		}
	}
}

// 还不行，试 __NEXT_DATA__
if (paras.length < 5) {
	try {
		var nd = document.querySelector("#__NEXT_DATA__");
		if (nd && nd.textContent) {
			var data = JSON.parse(nd.textContent);
			var str = JSON.stringify(data);
			var matches = str.match(/"text":"([^"]{30,})"/g) || [];
			if (matches.length >= 5) {
				paras = matches.map(function(m) {
					return JSON.parse("{" + m + "}").text.replace(/\s+/g, " ").trim();
				}).filter(function(t) { return t.length >= 12; });
			}
		}
	} catch(e) {}
}

return JSON.stringify({
	url: location.href,
	title: (document.querySelector("h1")?.innerText || document.title || "").trim(),
	text: paras.join("\n\n"),
	paragraphs: paras,
	count: paras.length
});
})()
`

func extractArticle(targetURL string, timeoutMs int) ArticleResponse {
	if timeoutMs <= 0 {
		timeoutMs = 60000
	}

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	if err := chromedp.Run(ctx,
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("body"),
	); err != nil {
		return ArticleResponse{URL: targetURL, Error: "navigate: " + err.Error()}
	}

	time.Sleep(3 * time.Second)

	var raw string
	if err := chromedp.Run(ctx, chromedp.Evaluate(extractJS, &raw, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	})); err != nil {
		return ArticleResponse{URL: targetURL, Error: "extract: " + err.Error()}
	}

	if raw == "" {
		return ArticleResponse{URL: targetURL, Error: "empty result"}
	}

	var resp ArticleResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return ArticleResponse{URL: targetURL, Error: "json: " + err.Error()}
	}
	if resp.URL == "" {
		resp.URL = targetURL
	}
	if resp.Error != "" {
		resp.Error = "page: " + resp.Error
	}
	return resp
}

func handleArticle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	urlParam := r.URL.Query().Get("url")
	timeoutMs := 0

	if urlParam == "" {
		var req struct {
			URL       string `json:"url"`
			TimeoutMs int    `json:"timeout_ms"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			urlParam = req.URL
			timeoutMs = req.TimeoutMs
		}
	}

	if t := r.URL.Query().Get("timeout_ms"); t != "" {
		fmt.Sscanf(t, "%d", &timeoutMs)
	}

	if urlParam == "" {
		json.NewEncoder(w).Encode(ArticleResponse{Error: "missing url"})
		return
	}

	result := extractArticle(urlParam, timeoutMs)
	json.NewEncoder(w).Encode(result)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "ok",
		"bpc_loaded": bpcPath != "",
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.HandleFunc("/article", handleArticle)
	http.HandleFunc("/health", handleHealth)
	log.Printf("API on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

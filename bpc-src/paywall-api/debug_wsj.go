package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func main() {
	allocOpts := []chromedp.ExecAllocatorOption{
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	}
	if p := os.Getenv("PROXY_SERVER"); p != "" {
		allocOpts = append(allocOpts, chromedp.ProxyServer(p))
	}
	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	target := "https://cn.wsj.com/articles/%E6%B1%BD%E6%B2%B9%E4%BB%B7%E6%A0%BC%E4%B8%8A%E6%B6%A8%E5%90%9E%E5%99%AC%E7%BE%8E%E5%9B%BD%E4%BA%BA%E7%9A%84%E8%96%AA%E8%B5%84%E6%B6%A8%E5%B9%85-f69b06c0?mod=cn_economy"
	chromedp.Run(ctx, chromedp.Navigate(target))
	chromedp.Run(ctx, chromedp.WaitReady("body"))
	time.Sleep(5 * time.Second)

	var result string

	// 1. Check __NEXT_DATA__
	js1 := `(() => {
		var nd = document.querySelector("#__NEXT_DATA__");
		if (!nd || !nd.textContent) return "NO_NEXT_DATA";
		try {
			var data = JSON.parse(nd.textContent);
			var bp = data && data.props && data.props.pageProps;
			var body = (bp && bp.articleBody) || (bp && bp.body) || "";
			if (body) return "articleBody found, len=" + body.length;
			var str = JSON.stringify(data);
			var re = /"text":"([^"]{40,}?)"/g;
			var m, n = 0;
			while ((m = re.exec(str)) !== null) { n++; if (n==1) return "First match: " + m[1].substring(0,100); }
			return "No long text matches found. keys=" + Object.keys(data).join(",");
		} catch(e) { return "ERROR: " + e.message; }
	})()`

	chromedp.Run(ctx, chromedp.Evaluate(js1, &result, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}))
	fmt.Println("NEXT_DATA:", result)

	// 2. Check paywall HTML
	js2 := `(() => {
		var pw = document.querySelector("div.paywall");
		if (!pw) return "NO_PAYWALL_DIV";
		return "PAYWALL_HTML(len=" + pw.outerHTML.length + "): " + pw.outerHTML.substring(0,800);
	})()`

	chromedp.Run(ctx, chromedp.Evaluate(js2, &result, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}))
	fmt.Println()
	fmt.Println(result)

	// 3. Show all p in the page
	js3 := `(() => {
		var all = document.querySelectorAll("p");
		var res = [];
		all.forEach(function(p) {
			var t = (p.innerText || "").trim();
			if (t.length > 5) res.push({text: t.substring(0,60), cl: (p.className||"").substring(0,40), parent: (p.parentElement ? p.parentElement.className.substring(0,40) : "")});
		});
		return JSON.stringify(res.slice(0,20));
	})()`

	chromedp.Run(ctx, chromedp.Evaluate(js3, &result, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}))
	fmt.Println()
	fmt.Println("ALL P TAGS:", result)
}

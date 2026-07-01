package browser

import (
  "context"
  "fmt"
  "os"
  "log"
  "path/filepath"
  "sync"
  "time"
  "github.com/chromedp/chromedp"
  "github.com/chromedp/cdproto/network"
)

type Pool struct {
  mu          sync.Mutex
  browsers    []*Browser
  maxBrowsers int
  chromePath  string
  proxy       string
}
type Browser struct {
  ctx    context.Context
  cancel context.CancelFunc
  id     int
}

func NewPool(maxBrowsers int) *Pool { return &Pool{maxBrowsers: maxBrowsers} }
func (p *Pool) SetChromePath(cp string) { p.chromePath = cp }
func (p *Pool) SetProxy(pr string) { p.proxy = pr }
func (p *Pool) Start() error { log.Printf("[browser] ready max=%d", p.maxBrowsers); return nil }

func (p *Pool) Acquire() (*Browser, error) {
  p.mu.Lock()
  defer p.mu.Unlock()
  for _, b := range p.browsers {
    select {
    case <-b.ctx.Done(): continue
    default:
      var x interface{}
      if err := chromedp.Run(b.ctx, chromedp.Evaluate("1", &x)); err == nil { return b, nil }
    }
  }
  if len(p.browsers) >= p.maxBrowsers { return nil, fmt.Errorf("max browsers (%d)", p.maxBrowsers) }
  return p.newBrowser()
}

func (p *Pool) newBrowser() (*Browser, error) {
  bpcPath := os.Getenv("BPC_EXTENSION_PATH")
  if bpcPath == "" {
    abs, _ := filepath.Abs("bpc-src/bypass-paywalls-chrome-clean-master")
    candidates := []string{
      abs,
      `D:\bypass-paywalls-chrome-clean-master`,
      "/opt/bypass-paywalls-chrome-clean-master",
      "/app/plugin",
    }
    for _, c := range candidates {
      if _, err := os.Stat(filepath.Join(c, "manifest.json")); err == nil { bpcPath = c; break }
    }
  }
  opts := []chromedp.ExecAllocatorOption{
    chromedp.NoFirstRun, chromedp.NoDefaultBrowserCheck, chromedp.DisableGPU,
    chromedp.Flag("disable-web-security", true),
    chromedp.Flag("ignore-certificate-errors", true),
    chromedp.Flag("no-sandbox", true),
    chromedp.Flag("disable-dev-shm-usage", true),
    chromedp.Flag("disable-blink-features", "AutomationControlled"),
    chromedp.Flag("disable-component-update", true),
    chromedp.Flag("disable-background-networking", true),
    chromedp.Flag("disable-sync", true),
    chromedp.Flag("no-first-run", true),
    chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"),
  }
  if p.chromePath != "" { opts = append(opts, chromedp.ExecPath(p.chromePath)) }
  if bpcPath != "" {
    opts = append(opts, chromedp.Flag("disable-extensions-except", bpcPath), chromedp.Flag("load-extension", bpcPath))
    log.Printf("[browser] BPC extension: %s", bpcPath)
  }
  if p.proxy != "" { opts = append(opts, chromedp.ProxyServer(p.proxy)) }
  allocCtx, cancel1 := chromedp.NewExecAllocator(context.Background(), opts...)
  ctx, cancel2 := chromedp.NewContext(allocCtx)
  if err := chromedp.Run(ctx); err != nil { cancel2(); cancel1(); return nil, fmt.Errorf("start: %w", err) }
  b := &Browser{ctx: ctx, cancel: func() { cancel2(); cancel1() }, id: len(p.browsers) + 1}
  p.browsers = append(p.browsers, b)
  log.Printf("[browser] #%d created (%d total)", b.id, len(p.browsers))
  return b, nil
}

func (p *Pool) Release(b *Browser) {}
func (p *Pool) GetAll() []*Browser { p.mu.Lock(); defer p.mu.Unlock(); r := make([]*Browser, len(p.browsers)); copy(r, p.browsers); return r }
func (p *Pool) ActiveCount() int { return len(p.GetAll()) }
func (p *Pool) Shutdown() { p.mu.Lock(); defer p.mu.Unlock(); for _, b := range p.browsers { b.Close() }; p.browsers = nil }
func (b *Browser) Close() { b.cancel() }

func (b *Browser) Navigate(rawURL string, h map[string]string) (string, error) {
  ctx, cancel := context.WithTimeout(b.ctx, 90*time.Second)
  defer cancel()
  var html string
  err := chromedp.Run(ctx,
    chromedp.ActionFunc(func(ctx context.Context) error {
      return network.Enable().Do(ctx)
    }),
    chromedp.ActionFunc(func(ctx context.Context) error {
      blockedPatterns := []*network.BlockPattern{
        {URLPattern: "*://*.piano.io/*", Block: true},
        {URLPattern: "*://*.tinypass.com/*", Block: true},
        {URLPattern: "*://*.poool.fr/*", Block: true},
        {URLPattern: "*://*.permutive.com/*", Block: true},
        {URLPattern: "*://*.zephr.com/*", Block: true},
        {URLPattern: "*://*.cxense.com/*", Block: true},
        {URLPattern: "*://*.blueconic.com/*", Block: true},
      }
      return network.SetBlockedURLs().WithURLPatterns(blockedPatterns).Do(ctx)
    }),
    chromedp.ActionFunc(func(ctx context.Context) error {
      if len(h) == 0 { return nil }
      hdr := make(network.Headers)
      for k, v := range h { hdr[k] = v }
      return network.SetExtraHTTPHeaders(network.Headers(hdr)).Do(ctx)
    }),
    chromedp.Navigate(rawURL),
    chromedp.WaitReady("body"),
    chromedp.Sleep(18*time.Second),
    chromedp.OuterHTML("html", &html),
  )
  if err != nil { return "", fmt.Errorf("navigate: %w", err) }
  return html, nil
}

func (b *Browser) ExecuteJSOnCurrent(js string) (string, error) {
  var result string
  return result, chromedp.Run(b.ctx, chromedp.Evaluate(js, &result))
}

func (b *Browser) Evaluate(target interface{}, js string) error {
  return chromedp.Run(b.ctx, chromedp.Evaluate(js, &target))
}

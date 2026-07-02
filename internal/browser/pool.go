package browser

import (
  "context"
  "fmt"
  "os"
  "log"
  "path/filepath"
  "sync/atomic"
  "time"
  "github.com/chromedp/chromedp"
  "github.com/chromedp/cdproto/network"
  "github.com/chromedp/cdproto/page"
)

type Pool struct {
  inUse       int32
  maxBrowsers int
  chromePath  string
  proxy       string
  bpcPath     string
}

type Browser struct {
  ctx    context.Context
  cancel context.CancelFunc
  id     uint64
}

var nextID atomic.Uint64

func NewPool(maxBrowsers int) *Pool { return &Pool{maxBrowsers: maxBrowsers} }
func (p *Pool) SetChromePath(cp string) { p.chromePath = cp }
func (p *Pool) SetProxy(pr string) { p.proxy = pr }

func (p *Pool) Start() error {
  if p.bpcPath == "" {
    p.bpcPath = os.Getenv("BPC_EXTENSION_PATH")
    if p.bpcPath == "" {
      candidates := []string{
        "/app/plugin",
        "/opt/bypass-paywalls-chrome-clean-master",
        "bpc-src/bypass-paywalls-chrome-clean-master",
      }
      if abs, _ := filepath.Abs("bpc-src/bypass-paywalls-chrome-clean-master"); abs != "" {
        candidates = append([]string{abs}, candidates...)
      }
      for _, c := range candidates {
        if _, err := os.Stat(filepath.Join(c, "manifest.json")); err == nil {
          p.bpcPath = c; break
        }
      }
    }
  }
  log.Printf("[browser] pool ready max=%d bpc=%s proxy=%s", p.maxBrowsers, p.bpcPath, p.proxy)
  return nil
}

// Acquire creates a fresh isolated browser. Blocks if maxBrowsers reached.
func (p *Pool) Acquire() (*Browser, error) {
  for {
    cur := atomic.LoadInt32(&p.inUse)
    if cur >= int32(p.maxBrowsers) {
      return nil, fmt.Errorf("max concurrent browsers (%d) reached", p.maxBrowsers)
    }
    if atomic.CompareAndSwapInt32(&p.inUse, cur, cur+1) {
      break
    }
  }
  b, err := p.newBrowser()
  if err != nil {
    atomic.AddInt32(&p.inUse, -1)
    return nil, err
  }
  return b, nil
}

func (p *Pool) newBrowser() (*Browser, error) {
  opts := []chromedp.ExecAllocatorOption{
    chromedp.NoFirstRun, chromedp.NoDefaultBrowserCheck, chromedp.DisableGPU,
    chromedp.Flag("no-sandbox", true),
    chromedp.Flag("disable-dev-shm-usage", true),
    chromedp.WindowSize(1920, 1080),
    chromedp.Flag("disable-blink-features", "AutomationControlled"),
    chromedp.Flag("disable-component-update", true),
    chromedp.Flag("disable-background-networking", true),
    chromedp.Flag("disable-sync", true),
    chromedp.Flag("no-first-run", true),
    chromedp.Flag("use-gl", "angle"),
    chromedp.Flag("use-angle", "swiftshader"),
    chromedp.Flag("disable-webrtc", true),
    chromedp.Flag("lang", "en-US,en;q=0.9,zh-CN;q=0.8"),
    chromedp.Flag("disable-web-security", true),
    chromedp.Flag("ignore-certificate-errors", true),
    chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"),
  }
  if p.chromePath != "" { opts = append(opts, chromedp.ExecPath(p.chromePath)) }
  if p.bpcPath != "" {
    opts = append(opts,
      chromedp.Flag("disable-extensions-except", p.bpcPath),
      chromedp.Flag("load-extension", p.bpcPath),
    )
    log.Printf("[browser] bpc-extension: %s", p.bpcPath)
  }
  if p.proxy != "" { opts = append(opts, chromedp.ProxyServer(p.proxy)) }

  allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
  ctx, ctxCancel := chromedp.NewContext(allocCtx)
  if err := chromedp.Run(ctx); err != nil {
    ctxCancel(); allocCancel()
    return nil, fmt.Errorf("browser start: %w", err)
  }

  id := nextID.Add(1)
  return &Browser{
    ctx:    ctx,
    cancel: func() { ctxCancel(); allocCancel() },
    id:     id,
  }, nil
}

// Release closes the browser immediately.
func (p *Pool) Release(b *Browser) {
  if b == nil { return }
  b.Close()
  atomic.AddInt32(&p.inUse, -1)
  log.Printf("[browser] #%d closed (%d in use)", b.id, atomic.LoadInt32(&p.inUse))
}

func (p *Pool) ActiveCount() int { return int(atomic.LoadInt32(&p.inUse)) }
func (p *Pool) Shutdown()        {}
func (b *Browser) Close()        { b.cancel() }

// Navigate creates a dedicated tab, navigates, and returns the outer HTML.
// The tab is closed before returning.
func (b *Browser) Navigate(rawURL string, h map[string]string) (string, error) {
  ctx, cancel := context.WithTimeout(b.ctx, 90*time.Second)
  defer cancel()

  tabCtx, tabCancel := chromedp.NewContext(ctx)
  defer tabCancel()

  var html string
  err := chromedp.Run(tabCtx,
    chromedp.ActionFunc(func(ctx context.Context) error { return network.Enable().Do(ctx) }),
    chromedp.ActionFunc(func(ctx context.Context) error {
      patterns := []*network.BlockPattern{
        {URLPattern: "*://*.piano.io/*", Block: true},
        {URLPattern: "*://*.tinypass.com/*", Block: true},
        {URLPattern: "*://*.poool.fr/*", Block: true},
        {URLPattern: "*://*.permutive.com/*", Block: true},
        {URLPattern: "*://*.zephr.com/*", Block: true},
        {URLPattern: "*://*.cxense.com/*", Block: true},
        {URLPattern: "*://*.blueconic.com/*", Block: true},
        {URLPattern: "*://*.newrelic.com/*", Block: true},
        {URLPattern: "*://*.nr-data.net/*", Block: true},
        {URLPattern: "*://*.datadome.co/*", Block: true},
        {URLPattern: "*://*.js.datadome.co/*", Block: true},
        {URLPattern: "*://*.api-js.datadome.co/*", Block: true},
      }
      return network.SetBlockedURLs().WithURLPatterns(patterns).Do(ctx)
    }),
    chromedp.ActionFunc(func(ctx context.Context) error {
      if len(h) == 0 { return nil }
      hdr := make(network.Headers)
      for k, v := range h { hdr[k] = v }
      return network.SetExtraHTTPHeaders(network.Headers(hdr)).Do(ctx)
    }),
    chromedp.ActionFunc(func(ctx context.Context) error {
      _, err := page.AddScriptToEvaluateOnNewDocument(`(()=>{
        Object.defineProperty(navigator,'webdriver',{get:()=>undefined});
        Object.defineProperty(navigator,'plugins',{get:()=>[1,2,3,4,5]});
        Object.defineProperty(navigator,'languages',{get:()=>['en-US','en','zh-CN']});
        window.chrome={runtime:{}};
      })()`).Do(ctx)
      return err
    }),
    chromedp.Navigate(rawURL),
    chromedp.WaitReady("body"),
    chromedp.Sleep(12*time.Second),
    chromedp.OuterHTML("html", &html),
  )
  if err != nil { return "", fmt.Errorf("navigate: %w", err) }
  return html, nil
}

// NavigateAndEval navigates in a dedicated tab, waits, evaluates JS, and returns result.
// The tab is closed when done. This is the primary method for /fetch/js.
func (b *Browser) NavigateAndEval(rawURL string, headers map[string]string, jsCode string, result interface{}) error {
  ctx, cancel := context.WithTimeout(b.ctx, 90*time.Second)
  defer cancel()

  tabCtx, tabCancel := chromedp.NewContext(ctx)
  defer tabCancel()

  return chromedp.Run(tabCtx,
    chromedp.ActionFunc(func(ctx context.Context) error { return network.Enable().Do(ctx) }),
    chromedp.ActionFunc(func(ctx context.Context) error {
      patterns := []*network.BlockPattern{
        {URLPattern: "*://*.piano.io/*", Block: true},
        {URLPattern: "*://*.tinypass.com/*", Block: true},
        {URLPattern: "*://*.poool.fr/*", Block: true},
        {URLPattern: "*://*.permutive.com/*", Block: true},
        {URLPattern: "*://*.zephr.com/*", Block: true},
        {URLPattern: "*://*.cxense.com/*", Block: true},
        {URLPattern: "*://*.blueconic.com/*", Block: true},
        {URLPattern: "*://*.newrelic.com/*", Block: true},
        {URLPattern: "*://*.nr-data.net/*", Block: true},
        {URLPattern: "*://*.datadome.co/*", Block: true},
        {URLPattern: "*://*.js.datadome.co/*", Block: true},
        {URLPattern: "*://*.api-js.datadome.co/*", Block: true},
      }
      return network.SetBlockedURLs().WithURLPatterns(patterns).Do(ctx)
    }),
    chromedp.ActionFunc(func(ctx context.Context) error {
      if len(headers) == 0 { return nil }
      hdr := make(network.Headers)
      for k, v := range headers { hdr[k] = v }
      return network.SetExtraHTTPHeaders(network.Headers(hdr)).Do(ctx)
    }),
    chromedp.ActionFunc(func(ctx context.Context) error {
      _, err := page.AddScriptToEvaluateOnNewDocument(`(()=>{
        Object.defineProperty(navigator,'webdriver',{get:()=>undefined});
        Object.defineProperty(navigator,'plugins',{get:()=>[1,2,3,4,5]});
        Object.defineProperty(navigator,'languages',{get:()=>['en-US','en','zh-CN']});
        window.chrome={runtime:{}};
      })()`).Do(ctx)
      return err
    }),
    chromedp.Navigate(rawURL),
    chromedp.WaitReady("body"),
    chromedp.Sleep(12*time.Second),
    chromedp.Evaluate(jsCode, result),
  )
}

package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/caichengle666/bypass-paywall-api/internal/browser"
	"github.com/caichengle666/bypass-paywall-api/internal/config"
	"github.com/caichengle666/bypass-paywall-api/internal/extract"
	"github.com/caichengle666/bypass-paywall-api/internal/strategy"
)

type Server struct {
	bp     *browser.Pool
	cfg    *config.SitesConfig
	apiKey string
	cache  *Cache
}

type FetchReq struct {
	URL     string `json:"url"`
	Timeout int    `json:"timeout,omitempty"`
	Sleep   int    `json:"sleep,omitempty"`
	Force   bool   `json:"force,omitempty"`
}

type FetchResp struct {
	Success    bool      `json:"success"`
	URL        string    `json:"url"`
	Status     string    `json:"status,omitempty"`
	SiteStatus string    `json:"site_status,omitempty"`
	Title      string    `json:"title,omitempty"`
	Paragraphs []string  `json:"paragraphs,omitempty"`
	FullText   string    `json:"full_text,omitempty"`
	Source     string    `json:"source,omitempty"`
	Error      string    `json:"error,omitempty"`
	Latency    int64     `json:"latency_ms,omitempty"`
	Cached     bool      `json:"cached,omitempty"`
	Sections   *Homepage `json:"sections,omitempty"`
	Articles   []Link    `json:"articles,omitempty"`
}

type NavItem struct {
	Label string `json:"label"`
	Href  string `json:"href"`
}
type Section struct {
	Section  string `json:"section"`
	Articles []Link `json:"articles"`
}
type Link struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

type Homepage struct {
	Navigation      []NavItem `json:"navigation"`
	Sections        []Section `json:"sections"`
	TotalArticles   int       `json:"total_articles"`
	TotalArticlesJS int       `json:"totalArticles,omitempty"`
}

type jsResult struct {
	Title          string    `json:"title"`
	Paragraphs     []string  `json:"paragraphs"`
	ParagraphCount int       `json:"paragraphCount"`
	Sections       *Homepage `json:"sections"`
}

type Cache struct {
	mu    sync.RWMutex
	items map[string]cacheItem
}

type cacheItem struct {
	resp      FetchResp
	expiresAt time.Time
}

var articleCacheTTL = 7 * 24 * time.Hour
var homeCacheTTL = 30 * time.Minute
var failedCacheTTL = 10 * time.Minute

func NewServer(bp *browser.Pool, cfg *config.SitesConfig) *Server {
	return &Server{bp: bp, cfg: cfg, cache: &Cache{items: make(map[string]cacheItem)}}
}
func (s *Server) SetAPIKey(key string) { s.apiKey = key }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/article", s.article)
	mux.HandleFunc("/home", s.home)
	mux.HandleFunc("/fetch", s.fetch)
	mux.HandleFunc("/fetch/js", s.fetchJS)
	mux.HandleFunc("/sites", s.sitesList)
	mux.HandleFunc("/sites/lookup", s.siteLookup)
	return s.authWrap(wrap(mux))
}

func (s *Server) authWrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.apiKey != "" {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				auth = r.URL.Query().Get("api_key")
			}
			expected := "Bearer " + s.apiKey
			if auth != expected && auth != s.apiKey {
				if r.URL.Path == "/health" {
					next.ServeHTTP(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(401)
				json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.Method == "OPTIONS" {
			w.WriteHeader(200)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok", "sites_count": s.cfg.Count(),
		"browsers": s.bp.ActiveCount(), "version": "2.0.0",
		"auth_enabled": s.apiKey != "", "cache_items": s.cacheLen(),
	})
}

func computeSleep(req *FetchReq) time.Duration {
	if req.Sleep > 0 && req.Sleep <= 30 {
		return time.Duration(req.Sleep) * time.Second
	}
	if req.Timeout > 0 && req.Timeout <= 30 {
		return time.Duration(req.Timeout) * time.Second
	}
	return 12 * time.Second
}

func (s *Server) article(w http.ResponseWriter, r *http.Request) { s.fetchJSWithMode(w, r, "article") }
func (s *Server) home(w http.ResponseWriter, r *http.Request)    { s.fetchJSWithMode(w, r, "home") }

func (s *Server) fetch(w http.ResponseWriter, r *http.Request) {
	req, err := parseReq(r)
	if err != nil {
		writeErr(w, err.Error(), 400)
		return
	}
	strat, _ := strategy.ResolveStrategy(req.URL, s.cfg)
	start := time.Now()
	br, err := s.bp.Acquire()
	if err != nil {
		writeErr(w, "browser: "+err.Error(), 503)
		return
	}
	defer s.bp.Release(br)
	html, err := br.Navigate(req.URL, strategy.BuildHeaders(strat, req.URL), computeSleep(req))
	if err != nil {
		writeErr(w, "navigate: "+err.Error(), 504)
		return
	}
	art := extract.ExtractFromHTML(html)
	resp := FetchResp{
		Success: len(art.Paragraphs) > 0, URL: req.URL,
		Paragraphs: art.Paragraphs, FullText: art.FullText,
		Source: art.Source, Latency: time.Since(start).Milliseconds(),
	}
	resp.Status, resp.SiteStatus = classifyResult(resp)
	resp.Success = resp.Status == "ok"
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) fetchJS(w http.ResponseWriter, r *http.Request) { s.fetchJSWithMode(w, r, "fetch") }

func (s *Server) fetchJSWithMode(w http.ResponseWriter, r *http.Request, mode string) {
	req, err := parseReq(r)
	if err != nil {
		writeErr(w, err.Error(), 400)
		return
	}
	cacheKey := mode + ":" + normalizeURL(req.URL)
	if !req.Force {
		if resp, ok := s.cacheGet(cacheKey); ok {
			resp.Cached = true
			json.NewEncoder(w).Encode(resp)
			return
		}
	}

	strat, _ := strategy.ResolveStrategy(req.URL, s.cfg)
	start := time.Now()
	br, err := s.bp.Acquire()
	if err != nil {
		writeErr(w, "browser: "+err.Error(), 503)
		return
	}
	defer s.bp.Release(br)

	var result jsResult
	err = br.NavigateAndEval(req.URL, strategy.BuildHeaders(strat, req.URL), extract.ArticleExtractionJS, computeSleep(req), &result)
	if err != nil {
		resp := FetchResp{Success: false, URL: req.URL, Status: "error", SiteStatus: "timeout", Error: "navigate/js: " + err.Error(), Latency: time.Since(start).Milliseconds()}
		s.cacheSet(cacheKey, resp, failedCacheTTL)
		w.WriteHeader(504)
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := FetchResp{
		URL: req.URL, Title: result.Title,
		Source: "js_injection", Latency: time.Since(start).Milliseconds(),
	}
	if result.Sections != nil {
		normalizeHomepage(result.Sections)
		resp.Sections = result.Sections
		resp.Articles = flattenArticles(result.Sections)
	}
	if len(result.Paragraphs) > 0 {
		resp.Paragraphs = result.Paragraphs
		resp.FullText = strings.Join(result.Paragraphs, "\n\n")
	}
	resp.Status, resp.SiteStatus = classifyResult(resp)
	resp.Success = resp.Status == "ok"
	if !resp.Success {
		resp.Error = resp.SiteStatus
	}

	ttl := articleCacheTTL
	if mode == "home" {
		ttl = homeCacheTTL
	}
	if !resp.Success {
		ttl = failedCacheTTL
	}
	s.cacheSet(cacheKey, resp, ttl)
	json.NewEncoder(w).Encode(resp)
}

func normalizeURL(rawURL string) string {
	return strings.TrimSpace(strings.TrimRight(rawURL, "/"))
}

func classifyResult(resp FetchResp) (string, string) {
	text := strings.ToLower(resp.Title + "\n" + resp.FullText)
	if strings.Contains(text, "akamai") || strings.Contains(text, "reference #") || strings.Contains(text, "datadome") {
		return "blocked", "blocked_by_datadome"
	}
	if strings.Contains(text, "page not found") || strings.Contains(text, "404") {
		return "not_found", "not_found"
	}
	if len(resp.Paragraphs) == 0 && len(resp.Articles) == 0 {
		return "no_content", "no_content"
	}
	return "ok", "ok"
}

func flattenArticles(home *Homepage) []Link {
	normalizeHomepage(home)
	if home == nil {
		return nil
	}
	seen := map[string]bool{}
	out := []Link{}
	for _, section := range home.Sections {
		for _, article := range section.Articles {
			if article.URL == "" || seen[article.URL] {
				continue
			}
			seen[article.URL] = true
			out = append(out, article)
		}
	}
	return out
}

func normalizeHomepage(home *Homepage) {
	if home == nil {
		return
	}
	if home.TotalArticles == 0 {
		home.TotalArticles = home.TotalArticlesJS
	}
	if home.TotalArticles == 0 {
		for _, section := range home.Sections {
			home.TotalArticles += len(section.Articles)
		}
	}
}

func (s *Server) cacheGet(key string) (FetchResp, bool) {
	s.cache.mu.RLock()
	item, ok := s.cache.items[key]
	s.cache.mu.RUnlock()
	if !ok {
		return FetchResp{}, false
	}
	if time.Now().After(item.expiresAt) {
		s.cache.mu.Lock()
		delete(s.cache.items, key)
		s.cache.mu.Unlock()
		return FetchResp{}, false
	}
	return item.resp, true
}

func (s *Server) cacheSet(key string, resp FetchResp, ttl time.Duration) {
	s.cache.mu.Lock()
	s.cache.items[key] = cacheItem{resp: resp, expiresAt: time.Now().Add(ttl)}
	s.cache.mu.Unlock()
}

func (s *Server) cacheLen() int {
	s.cache.mu.RLock()
	defer s.cache.mu.RUnlock()
	return len(s.cache.items)
}

func (s *Server) sitesList(w http.ResponseWriter, r *http.Request) {
	d := s.cfg.GetAllDomains()
	json.NewEncoder(w).Encode(map[string]interface{}{"domains": d, "count": len(d)})
}

func (s *Server) Serve(addr string) error {
	srv := &http.Server{Addr: addr, Handler: s.Handler(), ReadTimeout: 30 * time.Second, WriteTimeout: 120 * time.Second}
	log.Printf("[api] listening on %s", addr)
	return srv.ListenAndServe()
}

func parseReq(r *http.Request) (*FetchReq, error) {
	var req FetchReq
	if r.Method == "POST" {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return nil, err
		}
	} else {
		req.URL = r.URL.Query().Get("url")
		if v := r.URL.Query().Get("sleep"); v != "" {
			fmt.Sscanf(v, "%d", &req.Sleep)
		}
		if v := r.URL.Query().Get("timeout"); v != "" {
			fmt.Sscanf(v, "%d", &req.Timeout)
		}
		force := strings.ToLower(r.URL.Query().Get("force"))
		req.Force = force == "1" || force == "true" || force == "yes"
	}
	if req.URL == "" {
		return nil, fmt.Errorf("url required")
	}
	return &req, nil
}

func writeErr(w http.ResponseWriter, msg string, code int) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (s *Server) siteLookup(w http.ResponseWriter, r *http.Request) {
	u := r.URL.Query().Get("url")
	if u == "" {
		writeErr(w, "url required", 400)
		return
	}
	rule := s.cfg.Lookup(u)
	if rule == nil {
		json.NewEncoder(w).Encode(map[string]bool{"found": false})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"found": true, "rule": map[string]string{"domain": rule.Domain, "referer": rule.RefererURL}})
}

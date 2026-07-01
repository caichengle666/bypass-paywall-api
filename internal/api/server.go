package api

import (
  "encoding/json"
  "fmt"
  "log"
  "net/http"

  "strings"
  "time"
  "github.com/user/wsj-unlock-go-v2/internal/browser"
  "github.com/user/wsj-unlock-go-v2/internal/config"
  "github.com/user/wsj-unlock-go-v2/internal/extract"
  "github.com/user/wsj-unlock-go-v2/internal/strategy"
)

type Server struct{
  bp     *browser.Pool
  cfg    *config.SitesConfig
  apiKey string
}
type FetchReq struct {
  URL     string `json:"url"`
  Timeout int    `json:"timeout,omitempty"`
}
type FetchResp struct {
  Success    bool        `json:"success"`
  URL        string      `json:"url"`
  Title      string      `json:"title,omitempty"`
  Paragraphs []string    `json:"paragraphs,omitempty"`
  FullText   string      `json:"full_text,omitempty"`
  Source     string      `json:"source,omitempty"`
  Error      string      `json:"error,omitempty"`
  Latency    int64       `json:"latency_ms,omitempty"`
  Sections   *Homepage   `json:"sections,omitempty"`
}
type NavItem struct {
  Label string `json:"label"`
  Href  string `json:"href"`
}
type Section struct {
  Section   string `json:"section"`
  Articles  []Link `json:"articles"`
}
type Link struct {
  URL   string `json:"url"`
  Title string `json:"title"`
}
type Homepage struct {
  Navigation    []NavItem `json:"navigation"`
  Sections      []Section `json:"sections"`
  TotalArticles int       `json:"total_articles"`
}
type jsResult struct {
  Title          string     `json:"title"`
  Paragraphs     []string   `json:"paragraphs"`
  ParagraphCount int        `json:"paragraphCount"`
  Sections       *Homepage  `json:"sections"`
}

var startTime = time.Now()

func NewServer(bp *browser.Pool, cfg *config.SitesConfig) *Server { return &Server{bp: bp, cfg: cfg} }
func (s *Server) SetAPIKey(key string) { s.apiKey = key }

func (s *Server) Handler() http.Handler {
  mux := http.NewServeMux()
  mux.HandleFunc("/health", s.health)
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
      if auth == "" { auth = r.URL.Query().Get("api_key") }
      expected := "Bearer " + s.apiKey
      if auth != expected && auth != s.apiKey {
        if r.URL.Path == "/health" { next.ServeHTTP(w, r); return }
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
    if r.Method == "OPTIONS" { w.WriteHeader(200); return }
    next.ServeHTTP(w, r)
  })
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
  json.NewEncoder(w).Encode(map[string]interface{}{
    "status": "ok", "sites_count": s.cfg.Count(),
    "browsers": s.bp.ActiveCount(), "version": "2.0.0",
    "auth_enabled": s.apiKey != "",
  })
}

func (s *Server) fetch(w http.ResponseWriter, r *http.Request) {
  req, err := parseReq(r)
  if err != nil { writeErr(w, err.Error(), 400); return }
  strat, _ := strategy.ResolveStrategy(req.URL, s.cfg)
  start := time.Now()
  br, err := s.bp.Acquire()
  if err != nil { writeErr(w, "browser: "+err.Error(), 503); return }
  defer s.bp.Release(br)
  html, err := br.Navigate(req.URL, strategy.BuildHeaders(strat, req.URL))
  if err != nil { writeErr(w, "navigate: "+err.Error(), 504); return }
  art := extract.ExtractFromHTML(html)
  json.NewEncoder(w).Encode(FetchResp{
    Success: len(art.Paragraphs) > 0, URL: req.URL,
    Paragraphs: art.Paragraphs, FullText: art.FullText,
    Source: art.Source, Latency: time.Since(start).Milliseconds(),
  })
}

func (s *Server) fetchJS(w http.ResponseWriter, r *http.Request) {
  req, err := parseReq(r)
  if err != nil { writeErr(w, err.Error(), 400); return }
  strat, _ := strategy.ResolveStrategy(req.URL, s.cfg)
  start := time.Now()
  br, err := s.bp.Acquire()
  if err != nil { writeErr(w, "browser: "+err.Error(), 503); return }
  defer s.bp.Release(br)
  _, err = br.Navigate(req.URL, strategy.BuildHeaders(strat, req.URL))
  if err != nil { writeErr(w, "navigate: "+err.Error(), 504); return }
  var result jsResult
  if err := br.Evaluate(&result, extract.ArticleExtractionJS); err != nil {
    writeErr(w, "js: "+err.Error(), 500); return
  }
  resp := FetchResp{
    Success: result.ParagraphCount > 0 || result.Sections != nil,
    URL: req.URL, Title: result.Title,
    Source: "js_injection", Latency: time.Since(start).Milliseconds(),
  }
  if result.Sections != nil { resp.Sections = result.Sections }
  if len(result.Paragraphs) > 0 {
    resp.Paragraphs = result.Paragraphs
    resp.FullText = strings.Join(result.Paragraphs, "\n\n")
  }
  if !resp.Success { resp.Error = "no content extracted" }
  json.NewEncoder(w).Encode(resp)
}

func (s *Server) sitesList(w http.ResponseWriter, r *http.Request) {
  d := s.cfg.GetAllDomains()
  json.NewEncoder(w).Encode(map[string]interface{}{"domains": d, "count": len(d)})
}

func (s *Server) Serve(addr string) error {
  srv := &http.Server{Addr: addr, Handler: s.Handler(), ReadTimeout: 30*time.Second, WriteTimeout: 120*time.Second}
  log.Printf("[api] listening on %s", addr)
  return srv.ListenAndServe()
}

func parseReq(r *http.Request) (*FetchReq, error) {
  var req FetchReq
  if r.Method == "POST" {
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil { return nil, err }
  } else { req.URL = r.URL.Query().Get("url") }
  if req.URL == "" { return nil, fmt.Errorf("url required") }
  if req.Timeout <= 0 || req.Timeout > 30 { req.Timeout = 15 }
  return &req, nil
}

func writeErr(w http.ResponseWriter, msg string, code int) {
  w.WriteHeader(code)
  json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (s *Server) siteLookup(w http.ResponseWriter, r *http.Request) {
  u := r.URL.Query().Get("url")
  if u == "" { writeErr(w, "url required", 400); return }
  rule := s.cfg.Lookup(u)
  if rule == nil { json.NewEncoder(w).Encode(map[string]bool{"found": false}); return }
  json.NewEncoder(w).Encode(map[string]interface{}{"found": true, "rule": map[string]string{"domain": rule.Domain, "referer": rule.RefererURL}})
}

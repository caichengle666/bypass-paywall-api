package config

import (
  "encoding/json"
  "fmt"
  "net/url"
  "os"
  "strings"
  "sync"
)

type SiteRule struct {
  Domain        string `json:"domain"`
  AllowCookies  int    `json:"allow_cookies,omitempty"`
  UserAgent     string `json:"useragent,omitempty"`
  UserAgentCustom string `json:"useragent_custom,omitempty"`
  Referer       string `json:"referer,omitempty"`
  RefererCustom string `json:"referer_custom,omitempty"`
  CSDomPurify   int    `json:"cs_dompurify,omitempty"`
}

type ExpandedRule struct {
  Domain     string
  RefererURL string
  UserAgent  string
  CSEnabled  bool
}

type SitesConfig struct {
  mu    sync.RWMutex
  rules map[string]*ExpandedRule
}

func NewSitesConfig() *SitesConfig {
  return &SitesConfig{rules: make(map[string]*ExpandedRule)}
}

func (sc *SitesConfig) LoadFromJSON(filePath string) error {
  sc.mu.Lock()
  defer sc.mu.Unlock()
  data, err := os.ReadFile(filePath)
  if err != nil { return fmt.Errorf("read: %w", err) }
  var raw map[string]map[string]interface{}
  if err := json.Unmarshal(data, &raw); err != nil { return fmt.Errorf("parse: %w", err) }
  sc.rules = make(map[string]*ExpandedRule)
  for name, fields := range raw {
    if name == "" || name[0] == '*' || name[0] == '#' { continue }
    domain, _ := fields["domain"].(string)
    if domain == "" || domain[0] == '#' { continue }
    er := &ExpandedRule{Domain: domain}
    if ref, ok := fields["referer_custom"].(string); ok { er.RefererURL = ref }
    if ua, ok := fields["useragent"].(string); ok {
      switch ua {
      case "googlebot": er.UserAgent = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
      case "bingbot":  er.UserAgent = "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)"
      case "facebook": er.UserAgent = "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)"
      default: er.UserAgent = ua
      }
    }
    if uac, ok := fields["useragent_custom"].(string); ok && uac != "" { er.UserAgent = uac }
    er.CSEnabled = fields["cs_dompurify"] == 1.0 || fields["cs_dompurify"] == 1
    sc.rules[domain] = er
  }
  return nil
}

func (sc *SitesConfig) Lookup(rawURL string) *ExpandedRule {
  sc.mu.RLock()
  defer sc.mu.RUnlock()
  u, err := url.Parse(rawURL)
  if err != nil { return nil }
  host := strings.ToLower(u.Hostname())
  if rule, ok := sc.rules[host]; ok { return rule }
  trimmed := strings.TrimPrefix(host, "www.")
  trimmed = strings.TrimPrefix(trimmed, "cn.")
  if rule, ok := sc.rules[trimmed]; ok { return rule }
  return nil
}

func (sc *SitesConfig) GetAllDomains() []string {
  sc.mu.RLock()
  defer sc.mu.RUnlock()
  out := make([]string, 0, len(sc.rules))
  for d := range sc.rules { out = append(out, d) }
  return out
}

func (sc *SitesConfig) Count() int {
  sc.mu.RLock()
  defer sc.mu.RUnlock()
  return len(sc.rules)
}

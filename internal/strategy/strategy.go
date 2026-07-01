package strategy

import (
  "net/url"
  "strings"
  "github.com/user/wsj-unlock-go-v2/internal/config"
)

type Strategy struct {
  RefererURL string
  UserAgent  string
  Domain     string
}

func ResolveStrategy(targetURL string, cfg *config.SitesConfig) (*Strategy, error) {
  ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"
  s := &Strategy{UserAgent: ua}
  u, _ := url.Parse(targetURL)
  if u != nil { s.Domain = strings.TrimPrefix(strings.TrimPrefix(u.Hostname(), "www."), "cn.") }
  rule := cfg.Lookup(targetURL)
  if rule == nil { return s, nil }
  s.RefererURL = rule.RefererURL
  if rule.UserAgent != "" { s.UserAgent = rule.UserAgent }
  return s, nil
}

func BuildHeaders(strat *Strategy, targetURL string) map[string]string {
  h := map[string]string{
    "User-Agent": strat.UserAgent, "Accept": "text/html,*/*",
    "Accept-Language": "en-US,en;q=0.9",
  }
  if strat.RefererURL != "" { h["Referer"] = strat.RefererURL }
  return h
}

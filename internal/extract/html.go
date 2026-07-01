package extract

import (
  "regexp"
  "strings"
)

type ArticleResult struct {
  Title      string
  Paragraphs []string
  FullText   string
  Source     string
}

func ExtractFromHTML(html string) *ArticleResult {
  r := &ArticleResult{Source: "raw_html"}
  if p := extractNextData(html); len(p) > 0 {
    r.Source = "next_data"
    r.Paragraphs = p
    r.FullText = strings.Join(p, "\n\n")
    return r
  }
  if p := extractPaywall(html); len(p) > 0 {
    r.Source = "paywall"
    r.Paragraphs = p
    r.FullText = strings.Join(p, "\n\n")
    return r
  }
  return r
}

var (
  nextDataRe = regexp.MustCompile(`<script id="__NEXT_DATA__"[^>]*type="application/json"[^>]*>([\s\S]*?)<\/script>`)
  articleBodyRe = regexp.MustCompile(`"articleBody"\s*:\s*"((?:[^"\\]|\\.)*)"`)
  paywallDivRe = regexp.MustCompile(`<div[^>]*class="[^"]*paywall[^"]*"[^>]*>([\s\S]*?)<\/div>`)
  paywallPRe = regexp.MustCompile(`<p[^>]*data-type="paragraph"[^>]*>([\s\S]*?)<\/p>`)
  stripRe = regexp.MustCompile(`<[^>]*>`)
)

func extractNextData(html string) []string {
  m := nextDataRe.FindStringSubmatch(html)
  if len(m) < 2 { return nil }
  bm := articleBodyRe.FindStringSubmatch(m[1])
  if len(bm) >= 2 {
    t := strings.NewReplacer("\\n", "\n", "\\r", "", "\\t", " ", "\\\"", "\"").Replace(bm[1])
    return strings.Split(t, "\n\n")
  }
  return nil
}

func extractPaywall(html string) []string {
  m := paywallDivRe.FindStringSubmatch(html)
  if len(m) < 2 { return nil }
  var out []string
  for _, mm := range paywallPRe.FindAllStringSubmatch(m[1], -1) {
    t := strings.TrimSpace(stripRe.ReplaceAllString(mm[1], ""))
    if len(t) > 20 { out = append(out, t) }
  }
  return out
}
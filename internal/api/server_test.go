package api

import "testing"

func TestSearchURL(t *testing.T) {
	tests := []struct {
		site string
		want string
	}{
		{"", "https://cn.wsj.com/zh-hans/search?query=china"},
		{"wsj", "https://cn.wsj.com/zh-hans/search?query=china"},
		{"bbc", "https://www.bbc.com/search?q=china"},
		{"economist", "https://www.economist.com/search?q=china"},
	}
	for _, tt := range tests {
		got, err := searchURL(tt.site, "china")
		if err != nil || got != tt.want {
			t.Errorf("searchURL(%q) = %q, %v; want %q", tt.site, got, err, tt.want)
		}
	}
}

func TestIsNewsLink(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"wsj article", "https://cn.wsj.com/articles/example-123", true},
		{"bbc article", "https://www.bbc.com/news/articles/cvgl8dlxjd3o", true},
		{"economist article", "https://www.economist.com/china/2026/07/09/example", true},
		{"economist topic", "https://www.economist.com/topics/china", false},
		{"economist video", "https://www.economist.com/video/abc/def", false},
		{"bbc section", "https://www.bbc.com/news/world", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNewsLink(Link{URL: tt.url, Title: "Example article title"})
			if got != tt.want {
				t.Fatalf("isNewsLink(%q) = %v; want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestFilterSearchResultsPrefersMarkedWSJResults(t *testing.T) {
	articles := []Link{
		{URL: "https://cn.wsj.com/articles/trending-123?mod=trending_now", Title: "中东热门文章"},
		{URL: "https://cn.wsj.com/articles/iran-456?mod=Searchresults&pos=1", Title: "伊朗局势最新进展"},
	}
	got := filterSearchResults(articles, "中东", 10)
	if len(got) != 1 || got[0].Title != "伊朗局势最新进展" {
		t.Fatalf("filterSearchResults() = %#v; want marked WSJ result", got)
	}
}

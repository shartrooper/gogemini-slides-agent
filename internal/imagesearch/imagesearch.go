package imagesearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Options struct {
	ImgSize          string // icon|small|medium|large|xlarge|xxlarge|huge
	ImgType          string // clipart|face|lineart|news|photo
	ImgColorType     string // mono|gray|color
	ImgDominantColor string // red|orange|yellow|green|teal|blue|purple|pink|white|gray|black|brown
	Rights           string // e.g., cc_publicdomain|cc_attribute|...
	Safe             string // off|medium|active
	Num              int    // max results to fetch, 1-10
}

type SearchResponse struct {
	Items []struct {
		Title   string `json:"title"`
		Link    string `json:"link"`
		Snippet string `json:"snippet"`
		Mime    string `json:"mime"`
	} `json:"items"`
}

// SearchBestImage queries Google Custom Search for images and returns the best matching image URL.
func SearchBestImage(ctx context.Context, apiKey, cx, query string, opts Options) (string, error) {
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(cx) == "" {
		return "", fmt.Errorf("missing CSE key or cx")
	}
	if strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("empty query")
	}
	if opts.Num <= 0 || opts.Num > 10 {
		opts.Num = 5
	}

	u, _ := url.Parse("https://customsearch.googleapis.com/customsearch/v1")
	q := u.Query()
	q.Set("key", apiKey)
	q.Set("cx", cx)
	q.Set("q", query)
	q.Set("num", fmt.Sprintf("%d", opts.Num))
	q.Set("searchType", "image")
	if opts.Safe != "" {
		q.Set("safe", opts.Safe)
	}
	if opts.ImgSize != "" {
		q.Set("imgSize", opts.ImgSize)
	}
	if opts.ImgType != "" {
		q.Set("imgType", opts.ImgType)
	}
	if opts.ImgColorType != "" {
		q.Set("imgColorType", opts.ImgColorType)
	}
	if opts.ImgDominantColor != "" {
		q.Set("imgDominantColor", opts.ImgDominantColor)
	}
	if opts.Rights != "" {
		q.Set("rights", opts.Rights)
	}
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cse http %d", resp.StatusCode)
	}

	var sr SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return "", err
	}
	if len(sr.Items) == 0 {
		return "", fmt.Errorf("no results")
	}

	// Score by topic word matches in title/snippet
	terms := tokenize(query)
	bestIdx := 0
	bestScore := -1
	for i, it := range sr.Items {
		score := scoreItem(it.Title, it.Snippet, it.Link, terms)
		// prefer https and typical image mimes
		if strings.HasPrefix(strings.ToLower(it.Link), "https://") {
			score += 1
		}
		if strings.HasPrefix(it.Mime, "image/") {
			score += 1
		}
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	return sr.Items[bestIdx].Link, nil
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	repl := strings.NewReplacer(
		",", " ", ".", " ", "-", " ", "_", " ", "(", " ", ")", " ", "[", " ", "]", " ", "'", " ", "\"", " ", ":", " ", ";", " ", "!", " ", "?", " ", "&", " ")
	s = repl.Replace(s)
	parts := strings.Fields(s)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) >= 2 {
			out = append(out, p)
		}
	}
	return out
}

func scoreItem(title, snippet, link string, terms []string) int {
	text := strings.ToLower(strings.Join([]string{title, snippet, link}, " "))
	score := 0
	for _, t := range terms {
		if strings.Contains(text, t) {
			score++
		}
	}
	return score
}

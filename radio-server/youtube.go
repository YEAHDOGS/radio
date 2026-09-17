package main

// YouTube: resolve "title + artists" to a video ID via the Data API.
// Only called when the Spotify track changes — results are cached.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type ytResolver struct {
	key   string
	http  *http.Client
	mu    sync.Mutex
	cache map[string]string
}

func (y *ytResolver) resolve(title string, artists []string) string {
	if y.key == "" {
		return ""
	}
	q := title + " " + strings.Join(artists, " ") + " official audio"
	y.mu.Lock()
	if v, ok := y.cache[q]; ok {
		y.mu.Unlock()
		return v
	}
	y.mu.Unlock()
	vid := y.search(q)
	y.mu.Lock()
	y.cache[q] = vid
	y.mu.Unlock()
	return vid
}

func (y *ytResolver) search(q string) string {
	// Prefer the Music category, fall back to unfiltered search.
	for _, cat := range []string{"10", ""} {
		if v := y.searchCat(q, cat); v != "" {
			return v
		}
	}
	return ""
}

func (y *ytResolver) searchCat(q, cat string) string {
	params := url.Values{}
	params.Set("part", "snippet")
	params.Set("type", "video")
	params.Set("maxResults", "3")
	params.Set("q", q)
	params.Set("key", y.key)
	if cat != "" {
		params.Set("videoCategoryId", cat)
	}
	resp, err := y.http.Get("https://www.googleapis.com/youtube/v3/search?" + params.Encode())
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return ""
	}
	var out struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
		} `json:"items"`
	}
	if json.Unmarshal(b, &out) != nil || len(out.Items) == 0 {
		return ""
	}
	return out.Items[0].ID.VideoID
}

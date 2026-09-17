package main

// Spotify: refresh-token auth + 1-second currently-playing poll.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type spotifyClient struct {
	cfg      *config
	mu       sync.Mutex
	token    string
	tokenExp time.Time
	http     *http.Client
}

type spTrack struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	DurationMs int64  `json:"duration_ms"`
	Artists    []struct {
		Name string `json:"name"`
	} `json:"artists"`
	Album struct {
		Name   string `json:"name"`
		Images []struct {
			URL string `json:"url"`
		} `json:"images"`
	} `json:"album"`
}

type nowPlayingOut struct {
	Playing    bool
	ProgressMs int64
	Track      *spTrack
}

func (s *spotifyClient) accessToken() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && time.Now().Before(s.tokenExp.Add(-30*time.Second)) {
		return s.token, nil
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", s.cfg.SpotifyRefreshToken)
	req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(s.cfg.SpotifyClientID, s.cfg.SpotifyClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("token refresh: %s", strings.TrimSpace(string(b)))
	}
	var tr struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(b, &tr); err != nil {
		return "", err
	}
	s.token = tr.AccessToken
	s.tokenExp = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return s.token, nil
}

// nowPlaying returns nil-track / Playing=false when nothing is playing.
func (s *spotifyClient) nowPlaying() (*nowPlayingOut, error) {
	tok, err := s.accessToken()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", "https://api.spotify.com/v1/me/player/currently-playing", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 204 || resp.StatusCode == 404 {
		return &nowPlayingOut{Playing: false}, nil
	}
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("currently-playing: %d %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var body struct {
		IsPlaying  bool     `json:"is_playing"`
		ProgressMs int64    `json:"progress_ms"`
		Item       *spTrack `json:"item"`
	}
	if err := json.Unmarshal(b, &body); err != nil {
		return nil, err
	}
	return &nowPlayingOut{Playing: body.IsPlaying, ProgressMs: body.ProgressMs, Track: body.Item}, nil
}

// playlistTracks pulls every track from a Spotify playlist (paginated)
// with the owner's token. Used by the auto-DJ pool.
func (s *spotifyClient) playlistTracks(id string) ([]djTrack, error) {
	tok, err := s.accessToken()
	if err != nil {
		return nil, err
	}
	return s.playlistTracksWith(tok, id)
}

// playlistTracksWith is playlistTracks for an arbitrary bearer token —
// used by the converter with the visitor's own token.
func (s *spotifyClient) playlistTracksWith(tok string, id string) ([]djTrack, error) {
	var out []djTrack
	offset := 0
	for {
		u := fmt.Sprintf("https://api.spotify.com/v1/playlists/%s/tracks?limit=100&offset=%d&fields=total,items(track(id,name,duration_ms,artists(name),album(name,images)))", url.PathEscape(id), offset)
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := s.http.Do(req)
		if err != nil {
			return nil, err
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("playlist tracks: %d %s", resp.StatusCode, strings.TrimSpace(string(b)))
		}
		var body struct {
			Total int `json:"total"`
			Items []struct {
				Track *spTrack `json:"track"`
			} `json:"items"`
		}
		if err := json.Unmarshal(b, &body); err != nil {
			return nil, err
		}
		for _, it := range body.Items {
			t := it.Track
			if t == nil || t.ID == "" || t.DurationMs <= 0 {
				continue
			}
			out = append(out, djTrack{
				ID:         t.ID,
				Title:      t.Name,
				Artists:    artistNames(t),
				Album:      t.Album.Name,
				Art:        artURL(t),
				DurationMs: t.DurationMs,
			})
		}
		offset += len(body.Items)
		if offset >= body.Total || len(body.Items) == 0 {
			break
		}
	}
	return out, nil
}
// artistNames flattens Spotify artists for display + YouTube search.
func artistNames(t *spTrack) []string {
	out := make([]string, 0, len(t.Artists))
	for _, a := range t.Artists {
		out = append(out, a.Name)
	}
	return out
}

// artURL picks the smallest album image (fast for the now-playing card).
func artURL(t *spTrack) string {
	imgs := t.Album.Images
	if len(imgs) == 0 {
		return ""
	}
	return imgs[len(imgs)-1].URL
}

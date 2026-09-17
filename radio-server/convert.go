package main

// Playlist converter: Spotify <-> YouTube, per-user OAuth.
//
// Each visitor connects their own Spotify and Google accounts; the
// server converts one playlist at a time and reports progress as JSON.
// Sessions live in memory (a restart drops them) and are carried in an
// HMAC-signed cookie — no new dependencies, stdlib only.
//
// YouTube quota is guarded: the default 10,000 units/day project quota
// only stretches to ~60 converted tracks a day (search=100 +
// insert=50 per track), so jobs that would blow the budget are refused
// up front instead of dying halfway. Cached searches cost nothing.
//
// There is no official YouTube Music API, so conversions target regular
// YouTube playlists — those open and play inside the YouTube Music app.

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ---------- sessions ----------

type userTokens struct {
	spAccess  string
	spRefresh string
	spExp     time.Time
	goAccess  string
	goRefresh string
	goExp     time.Time
}

type sessionStore struct {
	mu     sync.Mutex
	m      map[string]*userTokens
	secret []byte
}

func newSessionStore(secret string) *sessionStore {
	s := &sessionStore{m: map[string]*userTokens{}}
	if secret != "" {
		s.secret = []byte(secret)
	} else {
		s.secret = make([]byte, 32)
		rand.Read(s.secret)
	}
	return s
}

func (s *sessionStore) sign(v string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(v))
	return base64.RawURLEncoding.EncodeToString([]byte(v)) + "." + hex.EncodeToString(mac.Sum(nil))
}

func (s *sessionStore) verify(v string) (string, bool) {
	parts := strings.Split(v, ".")
	if len(parts) != 2 {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	mac := hmac.New(sha256.New, s.secret)
	mac.Write(raw)
	if hex.EncodeToString(mac.Sum(nil)) != parts[1] {
		return "", false
	}
	return string(raw), true
}

// session returns the visitor's tokens and session id, creating an empty
// session (and setting the cookie) on first contact.
func (s *sessionStore) session(c *gin.Context) (*userTokens, string) {
	if ck, err := c.Cookie("drcv"); err == nil {
		if sid, ok := s.verify(ck); ok {
			s.mu.Lock()
			t, ok := s.m[sid]
			s.mu.Unlock()
			if ok {
				return t, sid
			}
		}
	}
	sid := make([]byte, 16)
	rand.Read(sid)
	id := hex.EncodeToString(sid)
	t := &userTokens{}
	s.mu.Lock()
	s.m[id] = t
	s.mu.Unlock()
	c.SetCookie("drcv", s.sign(id), 86400*30, "/", "", true, true)
	return t, id
}

// ---------- OAuth ----------

type convServer struct {
	cfg         *config
	sp          *spotifyClient
	yt          *ytResolver
	session     *sessionStore
	quota       *quotaTracker
	jobs        *jobStore
	jobTokensMu sync.Mutex
	jobTokens   map[string]*userTokens
}

const spConvScope = "playlist-read-private playlist-modify-public playlist-modify-private"
const goConvScope = "https://www.googleapis.com/auth/youtube.force-ssl"

func (cv *convServer) spLogin(c *gin.Context) {
	if cv.cfg.SpotifyClientID == "" {
		c.String(500, "server: SPOTIFY_CLIENT_ID not set")
		return
	}
	_, sid := cv.session.session(c)
	p := url.Values{}
	p.Set("client_id", cv.cfg.SpotifyClientID)
	p.Set("response_type", "code")
	p.Set("redirect_uri", cv.cfg.PublicURL+"/c/auth/spotify/callback")
	p.Set("scope", spConvScope)
	p.Set("state", cv.session.sign(sid+"|sp"))
	c.Redirect(302, "https://accounts.spotify.com/authorize?"+p.Encode())
}

func (cv *convServer) goLogin(c *gin.Context) {
	if cv.cfg.GoogleClientID == "" {
		c.String(500, "server: GOOGLE_CLIENT_ID not set")
		return
	}
	_, sid := cv.session.session(c)
	p := url.Values{}
	p.Set("client_id", cv.cfg.GoogleClientID)
	p.Set("redirect_uri", cv.cfg.PublicURL+"/c/auth/google/callback")
	p.Set("response_type", "code")
	p.Set("scope", goConvScope)
	p.Set("access_type", "offline")
	p.Set("prompt", "consent")
	p.Set("state", cv.session.sign(sid+"|go"))
	c.Redirect(302, "https://accounts.google.com/o/oauth2/v2/auth?"+p.Encode())
}

// sessionID returns the raw session id behind the visitor's cookie.
func (cv *convServer) sessionID(c *gin.Context) (string, bool) {
	ck, err := c.Cookie("drcv")
	if err != nil {
		return "", false
	}
	return cv.session.verify(ck)
}

func (cv *convServer) checkState(c *gin.Context, want string) (string, bool) {
	sid, ok := cv.session.verify(c.Query("state"))
	if !ok || sid != want {
		return "", false
	}
	return sid, true
}

func postForm(endpoint string, form url.Values, basicUser, basicPass string) ([]byte, int, error) {
	req, err := http.NewRequest("POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, err
	}
	if basicUser != "" {
		req.SetBasicAuth(basicUser, basicPass)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return b, resp.StatusCode, nil
}

type tokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func (cv *convServer) spCallback(c *gin.Context) {
	sid, ok := cv.sessionID(c)
	if _, ok2 := cv.checkState(c, sid+"|sp"); !ok || !ok2 {
		c.String(400, "bad state")
		return
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", c.Query("code"))
	form.Set("redirect_uri", cv.cfg.PublicURL+"/c/auth/spotify/callback")
	b, code, err := postForm("https://accounts.spotify.com/api/token", form, cv.cfg.SpotifyClientID, cv.cfg.SpotifyClientSecret)
	if err != nil || code != 200 {
		c.String(500, "spotify token exchange failed")
		return
	}
	var tr tokenResp
	if json.Unmarshal(b, &tr) != nil || tr.AccessToken == "" {
		c.String(500, "spotify token parse failed")
		return
	}
	cv.session.mu.Lock()
	t := cv.session.m[sid]
	t.spAccess, t.spRefresh = tr.AccessToken, tr.RefreshToken
	t.spExp = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	cv.session.mu.Unlock()
	c.Redirect(302, cv.cfg.AllowedOrigin+"/convert.html")
}

func (cv *convServer) goCallback(c *gin.Context) {
	sid, ok := cv.sessionID(c)
	if _, ok2 := cv.checkState(c, sid+"|go"); !ok || !ok2 {
		c.String(400, "bad state")
		return
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", c.Query("code"))
	form.Set("redirect_uri", cv.cfg.PublicURL+"/c/auth/google/callback")
	form.Set("client_id", cv.cfg.GoogleClientID)
	form.Set("client_secret", cv.cfg.GoogleClientSecret)
	b, code, err := postForm("https://oauth2.googleapis.com/token", form, "", "")
	if err != nil || code != 200 {
		c.String(500, "google token exchange failed")
		return
	}
	var tr tokenResp
	if json.Unmarshal(b, &tr) != nil || tr.AccessToken == "" {
		c.String(500, "google token parse failed")
		return
	}
	cv.session.mu.Lock()
	t := cv.session.m[sid]
	t.goAccess, t.goRefresh = tr.AccessToken, tr.RefreshToken
	t.goExp = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	cv.session.mu.Unlock()
	c.Redirect(302, cv.cfg.AllowedOrigin+"/convert.html")
}

// spToken returns a fresh Spotify access token for the visitor.
func (cv *convServer) spToken(t *userTokens) (string, error) {
	if t.spAccess == "" {
		return "", fmt.Errorf("spotify not connected")
	}
	if time.Now().Before(t.spExp.Add(-30 * time.Second)) {
		return t.spAccess, nil
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", t.spRefresh)
	b, code, err := postForm("https://accounts.spotify.com/api/token", form, cv.cfg.SpotifyClientID, cv.cfg.SpotifyClientSecret)
	if err != nil || code != 200 {
		return "", fmt.Errorf("spotify refresh failed")
	}
	var tr tokenResp
	if json.Unmarshal(b, &tr) != nil || tr.AccessToken == "" {
		return "", fmt.Errorf("spotify refresh parse failed")
	}
	t.spAccess = tr.AccessToken
	t.spExp = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return t.spAccess, nil
}

// goToken returns a fresh Google access token for the visitor.
func (cv *convServer) goToken(t *userTokens) (string, error) {
	if t.goAccess == "" {
		return "", fmt.Errorf("google not connected")
	}
	if time.Now().Before(t.goExp.Add(-30*time.Second)) {
		return t.goAccess, nil
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", t.goRefresh)
	form.Set("client_id", cv.cfg.GoogleClientID)
	form.Set("client_secret", cv.cfg.GoogleClientSecret)
	b, code, err := postForm("https://oauth2.googleapis.com/token", form, "", "")
	if err != nil || code != 200 {
		return "", fmt.Errorf("google refresh failed")
	}
	var tr tokenResp
	if json.Unmarshal(b, &tr) != nil || tr.AccessToken == "" {
		return "", fmt.Errorf("google refresh parse failed")
	}
	t.goAccess = tr.AccessToken
	t.goExp = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return t.goAccess, nil
}

func apiGet(tok, endpoint string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return b, resp.StatusCode, nil
}

func apiPost(tok, endpoint string, payload any) ([]byte, int, error) {
	var rdr io.Reader
	if payload != nil {
		j, _ := json.Marshal(payload)
		rdr = strings.NewReader(string(j))
	}
	req, err := http.NewRequest("POST", endpoint, rdr)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return b, resp.StatusCode, nil
}

// ---------- quota ----------

type quotaTracker struct {
	mu   sync.Mutex
	day  string
	used int
}

const (
	ytCostSearch         = 100
	ytCostPlaylistInsert = 50
	ytCostItemInsert     = 50
	ytDailyBudget        = 9000 // stay under the 10k default with headroom
)

func (q *quotaTracker) add(n int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	if q.day != today {
		q.day, q.used = today, 0
	}
	q.used += n
}

func (q *quotaTracker) fits(n int) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	used := q.used
	if q.day != today {
		used = 0
	}
	return used+n <= ytDailyBudget
}

// ---------- jobs ----------

type convJob struct {
	mu        sync.Mutex
	ID        string
	Direction string `json:"direction"`
	Name      string `json:"name"`
	Total     int    `json:"total"`
	Done      int    `json:"done"`
	Failed    []string `json:"failed"`
	State     string `json:"state"` // running | done | error
	ResultURL string `json:"result_url,omitempty"`
	Err       string `json:"err,omitempty"`
}

type jobStore struct {
	mu sync.Mutex
	m  map[string]*convJob
}

func newJob() *convJob {
	id := make([]byte, 8)
	rand.Read(id)
	return &convJob{ID: hex.EncodeToString(id), State: "running"}
}

func (j *convJob) snapshot() *convJob {
	j.mu.Lock()
	defer j.mu.Unlock()
	cp := *j
	cp.Failed = append([]string{}, j.Failed...)
	return &cp
}

// ---------- handlers ----------

func (cv *convServer) status(c *gin.Context) {
	t, _ := cv.session(c)
	c.JSON(200, gin.H{
		"spotify": t.spAccess != "",
		"google":  t.goAccess != "",
	})
}

func (cv *convServer) logout(c *gin.Context) {
	if sid, ok := cv.sessionID(c); ok {
		cv.session.mu.Lock()
		delete(cv.session.m, sid)
		cv.session.mu.Unlock()
	}
	c.SetCookie("drcv", "", -1, "/", "", true, true)
	c.JSON(200, gin.H{"ok": true})
}

type playlistInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Tracks int    `json:"tracks"`
}

func (cv *convServer) spPlaylistsH(c *gin.Context) {
	t, _ := cv.session(c)
	tok, err := cv.spToken(t)
	if err != nil {
		c.JSON(401, gin.H{"err": err.Error()})
		return
	}
	var out []playlistInfo
	nextURL := "https://api.spotify.com/v1/me/playlists?limit=50"
	for nextURL != "" && len(out) < 200 {
		b, code, err := apiGet(tok, nextURL)
		if err != nil || code != 200 {
			c.JSON(502, gin.H{"err": "spotify playlists failed"})
			return
		}
		var body struct {
			Next  string `json:"next"`
			Items []struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Tracks struct {
					Total int `json:"total"`
				} `json:"tracks"`
			} `json:"items"`
		}
		if json.Unmarshal(b, &body) != nil {
			c.JSON(502, gin.H{"err": "parse failed"})
			return
		}
		for _, p := range body.Items {
			out = append(out, playlistInfo{ID: p.ID, Name: p.Name, Tracks: p.Tracks.Total})
		}
		nextURL = body.Next
	}
	c.JSON(200, out)
}

func (cv *convServer) ytPlaylistsH(c *gin.Context) {
	t, _ := cv.session(c)
	tok, err := cv.goToken(t)
	if err != nil {
		c.JSON(401, gin.H{"err": err.Error()})
		return
	}
	var out []playlistInfo
	page := ""
	for len(out) < 200 {
		u := "https://www.googleapis.com/youtube/v3/playlists?part=snippet,contentDetails&mine=true&maxResults=50"
		if page != "" {
			u += "&pageToken=" + url.QueryEscape(page)
		}
		b, code, err := apiGet(tok, u)
		if err != nil || code != 200 {
			c.JSON(502, gin.H{"err": "youtube playlists failed"})
			return
		}
		var body struct {
			NextPageToken string `json:"nextPageToken"`
			Items         []struct {
				ID      string `json:"id"`
				Snippet struct {
					Title string `json:"title"`
				} `json:"snippet"`
				ContentDetails struct {
					ItemCount int `json:"itemCount"`
				} `json:"contentDetails"`
			} `json:"items"`
		}
		if json.Unmarshal(b, &body) != nil {
			c.JSON(502, gin.H{"err": "parse failed"})
			return
		}
		for _, p := range body.Items {
			out = append(out, playlistInfo{ID: p.ID, Name: p.Snippet.Title, Tracks: p.ContentDetails.ItemCount})
		}
		page = body.NextPageToken
		if page == "" {
			break
		}
	}
	c.JSON(200, out)
}

func (cv *convServer) convertH(c *gin.Context) {
	var req struct {
		Direction string `json:"direction"` // "sp2yt" | "yt2sp"
		SourceID  string `json:"source_id"`
		Name      string `json:"name"`
	}
	if err := c.BindJSON(&req); err != nil || req.SourceID == "" {
		c.JSON(400, gin.H{"err": "direction, source_id required"})
		return
	}
	if req.Direction != "sp2yt" && req.Direction != "yt2sp" {
		c.JSON(400, gin.H{"err": "direction must be sp2yt or yt2sp"})
		return
	}
	t, _ := cv.session(c)
	if req.Direction == "sp2yt" {
		tok, err := cv.spToken(t)
		if err != nil {
			c.JSON(401, gin.H{"err": "connect Spotify first"})
			return
		}
		if _, err := cv.goToken(t); err != nil {
			c.JSON(401, gin.H{"err": "connect YouTube first"})
			return
		}
		tracks, err := cv.sp.playlistTracksWith(tok, req.SourceID)
		if err != nil {
			c.JSON(502, gin.H{"err": "could not read Spotify playlist"})
			return
		}
		if len(tracks) == 0 {
			c.JSON(400, gin.H{"err": "source playlist is empty"})
			return
		}
		estimate := ytCostPlaylistInsert + len(tracks)*(ytCostSearch+ytCostItemInsert)
		if !cv.quota.fits(estimate) {
			c.JSON(429, gin.H{"err": "YouTube daily quota is spent — try again tomorrow"})
			return
		}
		name := req.Name
		if name == "" {
			name = "Converted from Spotify"
		}
		job := newJob()
		job.Direction, job.Name, job.Total = "sp2yt", name, len(tracks)
		cv.jobs.mu.Lock()
		cv.jobs.m[job.ID] = job
		cv.jobs.mu.Unlock()
		cv.captureJobTokens(job, c)
		go cv.runSp2yt(job, tracks)
		c.JSON(200, gin.H{"job": job.ID})
		return
	}
	// yt2sp
	tok, err := cv.goToken(t)
	if err != nil {
		c.JSON(401, gin.H{"err": "connect YouTube first"})
		return
	}
	if _, err := cv.spToken(t); err != nil {
		c.JSON(401, gin.H{"err": "connect Spotify first"})
		return
	}
	items, err := cv.ytPlaylistItems(tok, req.SourceID)
	if err != nil {
		c.JSON(502, gin.H{"err": "could not read YouTube playlist"})
		return
	}
	if len(items) == 0 {
		c.JSON(400, gin.H{"err": "source playlist is empty"})
		return
	}
	name := req.Name
	if name == "" {
		name = "Converted from YouTube"
	}
	job := newJob()
	job.Direction, job.Name, job.Total = "yt2sp", name, len(items)
	cv.jobs.mu.Lock()
	cv.jobs.m[job.ID] = job
	cv.jobs.mu.Unlock()
	cv.captureJobTokens(job, c)
	go cv.runYt2sp(job, items)
	c.JSON(200, gin.H{"job": job.ID})
}

func (cv *convServer) jobH(c *gin.Context) {
	cv.jobs.mu.Lock()
	j, ok := cv.jobs.m[c.Param("id")]
	cv.jobs.mu.Unlock()
	if !ok {
		c.JSON(404, gin.H{"err": "unknown job"})
		return
	}
	c.JSON(200, j.snapshot())
}

// ---------- conversion workers ----------

func (cv *convServer) runSp2yt(job *convJob, tracks []djTrack) {
	fail := func(err string) {
		job.mu.Lock()
		job.State, job.Err = "error", err
		job.mu.Unlock()
	}
	t := cv.jobToken(job.ID)
	if t == nil {
		fail("session expired")
		return
	}
	tok, err := cv.goToken(t)
	if err != nil {
		fail("google disconnected")
		return
	}
	// Create the YouTube playlist.
	b, code, err := apiPost(tok, "https://www.googleapis.com/youtube/v3/playlists?part=snippet,status", map[string]any{
		"snippet": map[string]any{"title": job.Name},
		"status":  map[string]any{"privacyStatus": "private"},
	})
	if err != nil || code != 200 {
		fail("could not create YouTube playlist")
		return
	}
	cv.quota.add(ytCostPlaylistInsert)
	var created struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(b, &created) != nil || created.ID == "" {
		fail("could not parse YouTube playlist")
		return
	}
	job.mu.Lock()
	job.ResultURL = "https://www.youtube.com/playlist?list=" + created.ID
	job.mu.Unlock()
	for _, tr := range tracks {
		vid := cv.yt.resolveCounted(tr.Title, tr.Artists, func() { cv.quota.add(ytCostSearch) })
		if vid == "" {
			job.mu.Lock()
			job.Failed = append(job.Failed, tr.Title+" — "+strings.Join(tr.Artists, ", "))
			job.Done++
			job.mu.Unlock()
			continue
		}
		_, code, err := apiPost(tok, "https://www.googleapis.com/youtube/v3/playlistItems?part=snippet", map[string]any{
			"snippet": map[string]any{
				"playlistId": created.ID,
				"resourceId": map[string]any{"kind": "youtube#video", "videoId": vid},
			},
		})
		if err != nil || code != 200 {
			job.mu.Lock()
			job.Failed = append(job.Failed, tr.Title+" — "+strings.Join(tr.Artists, ", "))
			job.mu.Unlock()
		} else {
			cv.quota.add(ytCostItemInsert)
		}
		job.mu.Lock()
		job.Done++
		job.mu.Unlock()
	}
	job.mu.Lock()
	job.State = "done"
	job.mu.Unlock()
}

// jobToken returns the tokens captured when the job was started.
func (cv *convServer) jobToken(id string) *userTokens {
	cv.jobTokensMu.Lock()
	defer cv.jobTokensMu.Unlock()
	return cv.jobTokens[id]
}

// captureJobTokens snapshots the starter's tokens so the worker doesn't
// depend on cookies.
func (cv *convServer) captureJobTokens(job *convJob, c *gin.Context) {
	t, _ := cv.session(c)
	cv.jobTokensMu.Lock()
	cv.jobTokens[job.ID] = t
	cv.jobTokensMu.Unlock()
}

type ytItem struct {
	VideoID string
	Title   string
}

func (cv *convServer) ytPlaylistItems(tok, playlistID string) ([]ytItem, error) {
	var out []ytItem
	page := ""
	for len(out) < 500 {
		u := "https://www.googleapis.com/youtube/v3/playlistItems?part=snippet,contentDetails&maxResults=50&playlistId=" + url.QueryEscape(playlistID)
		if page != "" {
			u += "&pageToken=" + url.QueryEscape(page)
		}
		b, code, err := apiGet(tok, u)
		if err != nil || code != 200 {
			return nil, fmt.Errorf("playlistItems failed")
		}
		var body struct {
			NextPageToken string `json:"nextPageToken"`
			Items         []struct {
				Snippet struct {
					Title string `json:"title"`
				} `json:"snippet"`
				ContentDetails struct {
					VideoID string `json:"videoId"`
				} `json:"contentDetails"`
			} `json:"items"`
		}
		if json.Unmarshal(b, &body) != nil {
			return nil, fmt.Errorf("parse failed")
		}
		for _, it := range body.Items {
			if it.ContentDetails.VideoID == "" {
				continue
			}
			out = append(out, ytItem{VideoID: it.ContentDetails.VideoID, Title: it.Snippet.Title})
		}
		page = body.NextPageToken
		if page == "" {
			break
		}
	}
	return out, nil
}

// cleanYTTitle turns "Artist - Title" / "Title - Topic" style YouTube
// titles into something Spotify search understands.
func cleanYTTitle(t string) string {
	t = strings.TrimSpace(strings.TrimSuffix(t, " - Topic"))
	t = strings.ReplaceAll(t, "(Official Video)", "")
	t = strings.ReplaceAll(t, "(Official Audio)", "")
	t = strings.ReplaceAll(t, "[Official Video]", "")
	t = strings.ReplaceAll(t, "[Official Audio]", "")
	return strings.TrimSpace(t)
}

func (cv *convServer) runYt2sp(job *convJob, items []ytItem) {
	fail := func(err string) {
		job.mu.Lock()
		job.State, job.Err = "error", err
		job.mu.Unlock()
	}
	t := cv.jobToken(job.ID)
	if t == nil {
		fail("session expired")
		return
	}
	tok, err := cv.spToken(t)
	if err != nil {
		fail("spotify disconnected")
		return
	}
	// Who owns the new playlist?
	b, code, err := apiGet(tok, "https://api.spotify.com/v1/me")
	if err != nil || code != 200 {
		fail("could not read Spotify profile")
		return
	}
	var me struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(b, &me) != nil || me.ID == "" {
		fail("could not parse Spotify profile")
		return
	}
	b, code, err = apiPost(tok, "https://api.spotify.com/v1/users/"+url.PathEscape(me.ID)+"/playlists", map[string]any{
		"name":   job.Name,
		"public": false,
	})
	if err != nil || code != 200 && code != 201 {
		fail("could not create Spotify playlist")
		return
	}
	var created struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(b, &created) != nil || created.ID == "" {
		fail("could not parse Spotify playlist")
		return
	}
	job.mu.Lock()
	job.ResultURL = "https://open.spotify.com/playlist/" + created.ID
	job.mu.Unlock()
	var uris []string
	flush := func() {
		if len(uris) == 0 {
			return
		}
		_, _, _ = apiPost(tok, "https://api.spotify.com/v1/playlists/"+url.PathEscape(created.ID)+"/tracks", map[string]any{"uris": uris})
		uris = nil
	}
	for _, it := range items {
		q := cleanYTTitle(it.Title)
		b, code, err := apiGet(tok, "https://api.spotify.com/v1/search?"+url.Values{"q": {q}, "type": {"track"}, "limit": {"1"}}.Encode())
		found := ""
		if err == nil && code == 200 {
			var sr struct {
				Tracks struct {
					Items []struct {
						URI string `json:"uri"`
					} `json:"items"`
				} `json:"tracks"`
			}
			if json.Unmarshal(b, &sr) == nil && len(sr.Tracks.Items) > 0 {
				found = sr.Tracks.Items[0].URI
			}
		}
		if found == "" {
			job.mu.Lock()
			job.Failed = append(job.Failed, it.Title)
			job.mu.Unlock()
		} else {
			uris = append(uris, found)
			if len(uris) >= 100 {
				flush()
			}
		}
		job.mu.Lock()
		job.Done++
		job.mu.Unlock()
	}
	flush()
	job.mu.Lock()
	job.State = "done"
	job.mu.Unlock()
}

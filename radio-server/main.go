package main

// Dogs Radio — Gin live server.
//
// Polls Jack Sparrow's Spotify every second. When the track changes it
// resolves the matching YouTube video and broadcasts the now-playing
// state to every connected listener over a websocket. The static site
// (radio.dogs.red) plays the YouTube audio in the background —
// listeners never sign in to anything.
//
// Run: go run .  (see README.md for env + deploy)

import (
	"bufio"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type config struct {
	Port                string
	PublicURL           string // e.g. https://radio-api.dogs.red (used for the OAuth callback)
	AllowedOrigin       string // e.g. https://radio.dogs.red
	SpotifyClientID     string
	SpotifyClientSecret string
	SpotifyRefreshToken string
	YoutubeKey          string
	AutoDJPlaylists     string // comma-separated Spotify playlist IDs for the auto-DJ pool
	AutoDJIdleSecs      int    // seconds of Spotify silence before the DJ takes over
	GoogleClientID      string // converter: Google OAuth client
	GoogleClientSecret  string
	ConvertSessSecret   string // converter: session cookie HMAC secret (random per boot if empty)
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// loadConfig reads .env.local (if present) then the environment.
func loadConfig() *config {
	if f, err := os.Open(".env.local"); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			k, v, ok := strings.Cut(line, "=")
			if ok && os.Getenv(strings.TrimSpace(k)) == "" {
				os.Setenv(strings.TrimSpace(k), strings.TrimSpace(v))
			}
		}
		f.Close()
	}
	idleSecs, _ := strconv.Atoi(getenv("AUTODJ_IDLE_SECS", "120"))
	if idleSecs < 10 {
		idleSecs = 10
	}
	return &config{
		Port:                getenv("PORT", "8080"),
		PublicURL:           getenv("PUBLIC_URL", "http://localhost:8080"),
		AllowedOrigin:       getenv("ALLOWED_ORIGIN", "https://radio.dogs.red"),
		SpotifyClientID:     os.Getenv("SPOTIFY_CLIENT_ID"),
		SpotifyClientSecret: os.Getenv("SPOTIFY_CLIENT_SECRET"),
		SpotifyRefreshToken: os.Getenv("SPOTIFY_REFRESH_TOKEN"),
		YoutubeKey:          os.Getenv("YOUTUBE_API_KEY"),
		AutoDJPlaylists:     os.Getenv("AUTODJ_PLAYLISTS"),
		AutoDJIdleSecs:      idleSecs,
		GoogleClientID:      os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:  os.Getenv("GOOGLE_CLIENT_SECRET"),
		ConvertSessSecret:   os.Getenv("CONVERT_SESSION_SECRET"),
	}
}

func corsMiddleware(cfg *config) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == cfg.AllowedOrigin {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func main() {
	cfg := loadConfig()
	if cfg.SpotifyClientID == "" || cfg.SpotifyRefreshToken == "" {
		log.Println("note: SPOTIFY_CLIENT_ID / SPOTIFY_REFRESH_TOKEN not set — visit /auth/login after configuring")
	}
	if cfg.YoutubeKey == "" {
		log.Println("note: YOUTUBE_API_KEY not set — listeners will get metadata but no audio")
	}

	hub := newHub()
	sp := &spotifyClient{cfg: cfg, http: &http.Client{Timeout: 10 * time.Second}}
	yt := &ytResolver{key: cfg.YoutubeKey, http: &http.Client{Timeout: 10 * time.Second}, cache: map[string]string{}}
	p := &poller{cfg: cfg, hub: hub, sp: sp, yt: yt}
	go p.loop()
	dj := &autoDJ{cfg: cfg, hub: hub, sp: sp, yt: yt, poller: p}
	go dj.loop()

	cv := &convServer{
		cfg:       cfg,
		sp:        sp,
		yt:        yt,
		session:   newSessionStore(cfg.ConvertSessSecret),
		quota:     &quotaTracker{},
		jobs:      &jobStore{m: map[string]*convJob{}},
		jobTokens: map[string]*userTokens{},
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), corsMiddleware(cfg))
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	r.GET("/api/now", func(c *gin.Context) { c.JSON(200, hub.snapshot()) })
	r.GET("/ws", hub.serveWS)
	r.GET("/auth/login", authLogin(cfg))
	r.GET("/auth/callback", authCallback(cfg))

	// Playlist converter (per-user OAuth, see convert.go).
	r.GET("/c/status", cv.status)
	r.POST("/c/logout", cv.logout)
	r.GET("/c/auth/spotify/login", cv.spLogin)
	r.GET("/c/auth/spotify/callback", cv.spCallback)
	r.GET("/c/auth/google/login", cv.goLogin)
	r.GET("/c/auth/google/callback", cv.goCallback)
	r.GET("/c/spotify/playlists", cv.spPlaylistsH)
	r.GET("/c/youtube/playlists", cv.ytPlaylistsH)
	r.POST("/c/convert", cv.convertH)
	r.GET("/c/jobs/:id", cv.jobH)

	log.Printf("dogs radio gin server listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}

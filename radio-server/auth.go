package main

// One-time OAuth bootstrap: visit /auth/login, approve, and the server
// saves the refresh token to .env.local. Restart once, done.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func authLogin(cfg *config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.SpotifyClientID == "" {
			c.String(500, "set SPOTIFY_CLIENT_ID first")
			return
		}
		params := url.Values{}
		params.Set("client_id", cfg.SpotifyClientID)
		params.Set("response_type", "code")
		params.Set("redirect_uri", cfg.PublicURL+"/auth/callback")
		params.Set("scope", "user-read-playback-state")
		c.Redirect(302, "https://accounts.spotify.com/authorize?"+params.Encode())
	}
}

func authCallback(cfg *config) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Query("code")
		if code == "" {
			c.String(400, "missing code")
			return
		}
		form := url.Values{}
		form.Set("grant_type", "authorization_code")
		form.Set("code", code)
		form.Set("redirect_uri", cfg.PublicURL+"/auth/callback")
		req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", strings.NewReader(form.Encode()))
		if err != nil {
			c.String(500, err.Error())
			return
		}
		req.SetBasicAuth(cfg.SpotifyClientID, cfg.SpotifyClientSecret)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			c.String(500, err.Error())
			return
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			c.String(500, string(b))
			return
		}
		var tok struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.Unmarshal(b, &tok); err != nil || tok.RefreshToken == "" {
			c.String(500, "no refresh_token in response")
			return
		}
		f, err := os.OpenFile(".env.local", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			c.String(500, err.Error())
			return
		}
		fmt.Fprintf(f, "SPOTIFY_REFRESH_TOKEN=%s\n", tok.RefreshToken)
		f.Close()
		c.Header("Content-Type", "text/html")
		c.String(200, "<h1>Connected.</h1><p>Refresh token saved to .env.local — restart the server.</p>")
	}
}

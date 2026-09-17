package main

// Poll loop: hits Spotify every second. On track change, resolves the
// YouTube video and broadcasts. A 10s heartbeat re-syncs stragglers.

import (
	"log"
	"strings"
	"time"
)

type poller struct {
	cfg         *config
	hub         *hub
	sp          *spotifyClient
	yt          *ytResolver
	lastTrackID string
	errs        int
}

func (p *poller) loop() {
	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()
	heartbeat := time.NewTicker(10 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-tick.C:
			p.tick()
		case <-heartbeat.C:
			p.hub.setState(p.hub.snapshot()) // re-broadcast = re-sync
		}
	}
}

func (p *poller) tick() {
	np, err := p.sp.nowPlaying()
	if err != nil {
		p.errs++
		log.Printf("spotify: %v", err)
		if p.errs >= 5 && p.lastTrackID != "" {
			p.hub.setState(nowMsg{T: "offair"})
			p.lastTrackID = ""
		}
		return
	}
	p.errs = 0
	if np == nil || !np.Playing || np.Track == nil {
		if p.lastTrackID != "" {
			p.hub.setState(nowMsg{T: "offair"})
			p.lastTrackID = ""
			log.Println("off air")
		}
		return
	}
	if np.Track.ID == p.lastTrackID {
		return
	}
	p.lastTrackID = np.Track.ID
	artists := artistNames(np.Track)
	vid := p.yt.resolve(np.Track.Name, artists)
	p.hub.setState(nowMsg{
		T:          "now",
		Playing:    true,
		ProgressMs: np.ProgressMs,
		At:         time.Now().UnixMilli(),
		Yt:         vid,
		Track: &trackJSON{
			ID:         np.Track.ID,
			Title:      np.Track.Name,
			Artists:    artists,
			Album:      np.Track.Album.Name,
			Art:        artURL(np.Track),
			DurationMs: np.Track.DurationMs,
		},
	})
	log.Printf("now playing: %s — %s → yt:%s", np.Track.Name, strings.Join(artists, ", "), vid)
}

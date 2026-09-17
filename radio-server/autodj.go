package main

// Auto-DJ: when his Spotify goes quiet, the station keeps playing.
//
// The DJ is virtual — it never touches Spotify playback. It pulls the
// track list from his configured playlists, shuffles, and walks through
// it on each track's real duration, broadcasting the same now-playing
// state live tracks use. Listeners resolve each track to YouTube exactly
// like a live pick. The moment he plays something on Spotify himself,
// the poller sees it and live takes over again instantly.

import (
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"
)

type djTrack struct {
	ID         string
	Title      string
	Artists    []string
	Album      string
	Art        string
	DurationMs int64
}

type autoDJ struct {
	cfg    *config
	hub    *hub
	sp     *spotifyClient
	yt     *ytResolver
	poller *poller // read-only: last live signal

	mu        sync.Mutex
	pool      []djTrack // every track across his playlists
	order     []int     // shuffled indices into pool
	pos       int
	cur       *djTrack
	startedAt time.Time
	lastFetch time.Time
	djing     bool
}

func (d *autoDJ) playlists() []string {
	var out []string
	for _, id := range strings.Split(d.cfg.AutoDJPlaylists, ",") {
		if id = strings.TrimSpace(id); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// refreshPool re-pulls his playlists every 15 minutes so new adds show up.
func (d *autoDJ) refreshPool() {
	d.mu.Lock()
	stale := time.Since(d.lastFetch) > 15*time.Minute
	d.mu.Unlock()
	if !stale {
		return
	}
	var pool []djTrack
	for _, pid := range d.playlists() {
		tracks, err := d.sp.playlistTracks(pid)
		if err != nil {
			log.Printf("autodj: playlist %s: %v", pid, err)
			continue
		}
		pool = append(pool, tracks...)
	}
	d.mu.Lock()
	d.pool = pool
	d.lastFetch = time.Now()
	// Re-shuffle when the pool changes so the order stays fresh.
	d.shuffleLocked()
	d.mu.Unlock()
	log.Printf("autodj: pool refreshed — %d tracks across %d playlists", len(pool), len(d.playlists()))
}

func (d *autoDJ) shuffleLocked() {
	d.order = rand.Perm(len(d.pool))
	d.pos = 0
}

// next advances to the next shuffled track, reshuffling at the end.
func (d *autoDJ) next() *djTrack {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.pool) == 0 {
		return nil
	}
	if d.pos >= len(d.order) {
		d.shuffleLocked()
	}
	t := d.pool[d.order[d.pos]]
	d.pos++
	d.cur = &t
	d.startedAt = time.Now()
	return d.cur
}

func (d *autoDJ) broadcast(t *djTrack) {
	vid := d.yt.resolve(t.Title, t.Artists)
	d.hub.setState(nowMsg{
		T:          "now",
		Src:        "autodj",
		Playing:    true,
		ProgressMs: 0,
		At:         time.Now().UnixMilli(),
		Yt:         vid,
		Track: &trackJSON{
			ID:         "dj-" + t.ID,
			Title:      t.Title,
			Artists:    t.Artists,
			Album:      t.Album,
			Art:        t.Art,
			DurationMs: t.DurationMs,
		},
	})
	log.Printf("autodj: %s — %s → yt:%s", t.Title, strings.Join(t.Artists, ", "), vid)
}

func (d *autoDJ) loop() {
	if len(d.playlists()) == 0 {
		log.Println("autodj: no AUTODJ_PLAYLISTS configured — DJ stays off")
		return
	}
	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()
	for range tick.C {
		// He's live — stand down instantly.
		if time.Since(d.poller.lastLive()) < time.Duration(d.cfg.AutoDJIdleSecs)*time.Second {
			d.mu.Lock()
			was := d.djing
			d.djing = false
			d.cur = nil
			d.mu.Unlock()
			if was {
				log.Println("autodj: he's back — handing over to live")
			}
			continue
		}
		d.refreshPool()
		d.mu.Lock()
		djing := d.djing
		cur := d.cur
		startedAt := d.startedAt
		poolEmpty := len(d.pool) == 0
		d.mu.Unlock()
		if poolEmpty {
			continue
		}
		if !djing {
			d.mu.Lock()
			d.djing = true
			d.mu.Unlock()
			log.Println("autodj: he's quiet — DJ taking over")
		}
		if cur == nil || time.Since(startedAt) >= time.Duration(cur.DurationMs)*time.Millisecond {
			if t := d.next(); t != nil {
				d.broadcast(t)
			}
		}
	}
}

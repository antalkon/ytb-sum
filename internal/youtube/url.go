// Package youtube fetches video metadata and transcripts via yt-dlp, keeping
// everything in memory.
package youtube

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var idRe = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// ParseVideoID extracts the 11-character video id from a URL or bare id.
func ParseVideoID(input string) (string, error) {
	s := strings.TrimSpace(input)
	if idRe.MatchString(s) {
		return s, nil
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("not a YouTube URL: %q", input)
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	host = strings.TrimPrefix(host, "m.")
	var id string
	switch host {
	case "youtu.be":
		id = firstSegment(u.Path)
	case "youtube.com", "music.youtube.com", "youtube-nocookie.com":
		if v := u.Query().Get("v"); v != "" {
			id = v
			break
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) == 2 {
			switch parts[0] {
			case "shorts", "live", "embed", "v":
				id = parts[1]
			}
		}
	}
	if !idRe.MatchString(id) {
		return "", fmt.Errorf("not a YouTube video URL: %q", input)
	}
	return id, nil
}

// WatchURL returns the canonical watch URL for id.
func WatchURL(id string) string { return "https://www.youtube.com/watch?v=" + id }

// TimestampURL returns a watch URL that starts playback at sec.
func TimestampURL(id string, sec int) string {
	return fmt.Sprintf("%s&t=%ds", WatchURL(id), sec)
}

func firstSegment(p string) string {
	p = strings.Trim(p, "/")
	if i := strings.IndexByte(p, '/'); i >= 0 {
		p = p[:i]
	}
	return p
}

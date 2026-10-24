package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/antalkon/ytb-sum/internal/proc"
)

// Chapter is a creator-defined chapter marker.
type Chapter struct {
	Start float64 `json:"start_time"`
	Title string  `json:"title"`
}

// Track is one downloadable subtitle format.
type Track struct {
	Ext  string `json:"ext"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

// Video holds the metadata ytb-sum needs from `yt-dlp --dump-json`.
type Video struct {
	ID          string             `json:"id"`
	Title       string             `json:"title"`
	Channel     string             `json:"channel"`
	Uploader    string             `json:"uploader"`
	Duration    float64            `json:"duration"`
	UploadDate  string             `json:"upload_date"` // YYYYMMDD
	Description string             `json:"description"`
	Tags        []string           `json:"tags"`
	ViewCount   int64              `json:"view_count"`
	Language    string             `json:"language"`
	Chapters    []Chapter          `json:"chapters"`
	Subtitles   map[string][]Track `json:"subtitles"`
	AutoCaps    map[string][]Track `json:"automatic_captions"`
	IsLive      bool               `json:"is_live"`
}

// Author returns the channel name, falling back to the uploader.
func (v *Video) Author() string {
	if v.Channel != "" {
		return v.Channel
	}
	return v.Uploader
}

// Date returns the upload date, or the zero time if unknown.
func (v *Video) Date() time.Time {
	t, _ := time.Parse("20060102", v.UploadDate)
	return t
}

// Options control how yt-dlp is invoked.
type Options struct {
	CookiesFromBrowser string
}

// BaseArgs are passed to every yt-dlp call so nothing is cached on disk.
func (o Options) BaseArgs() []string {
	args := []string{"--no-cache-dir", "--no-playlist", "--no-warnings", "--ignore-config"}
	if o.CookiesFromBrowser != "" {
		args = append(args, "--cookies-from-browser", o.CookiesFromBrowser)
	}
	return args
}

// Env returns the environment for yt-dlp, pointing its cache at cacheDir
// as a second line of defence behind --no-cache-dir.
func Env(cacheDir string) []string {
	return append(os.Environ(), "XDG_CACHE_HOME="+cacheDir)
}

// FetchVideo loads metadata for the video id. cacheDir is a throwaway
// directory owned by the caller.
func FetchVideo(ctx context.Context, id, cacheDir string, o Options) (*Video, error) {
	args := append(o.BaseArgs(), "--dump-json", "--skip-download", "--", WatchURL(id))
	out, err := proc.Run(ctx, Env(cacheDir), "yt-dlp", args...)
	if err != nil {
		return nil, fmt.Errorf("fetch metadata: %w", err)
	}
	var v Video
	if err := json.Unmarshal(out, &v); err != nil {
		return nil, fmt.Errorf("parse yt-dlp metadata: %w", err)
	}
	if v.IsLive {
		return nil, fmt.Errorf("%q is a live stream in progress; try again after it ends", v.Title)
	}
	return &v, nil
}

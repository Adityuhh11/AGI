package files

import (
	"math"
	"path/filepath"
	"strconv"
	"strings"
)

// Player is a program that can start a recording at a given moment.
type Player struct {
	// Name is what the bar calls it ("VLC").
	Name string
	// Exe is the path of its program.
	Exe  string
	kind playerKind
}

type playerKind int

const (
	vlc playerKind = iota
	mpv
	mpc  // Media Player Classic (Windows)
	iina // IINA (macOS), which passes `--mpv-` options on to mpv
)

// args is the command line that plays path from a moment, in seconds from the start.
func (p Player) args(path string, seconds float64) []string {
	if math.IsNaN(seconds) || seconds < 0 {
		seconds = 0
	}
	switch p.kind {
	case vlc:
		return []string{"--start-time=" + strconv.FormatFloat(seconds, 'f', 1, 64), path}
	case mpv:
		return []string{"--start=" + strconv.FormatFloat(seconds, 'f', 1, 64), path}
	case iina:
		return []string{"--mpv-start=" + strconv.FormatFloat(seconds, 'f', 1, 64), path}
	default:
		return []string{path, "/start", strconv.FormatInt(int64(seconds*1000), 10)}
	}
}

// mediaExtensions are the sounds and videos the engine searches by what they hold. They are safe to hand to whatever plays
// them: a player does not run a recording.
var mediaExtensions = map[string]bool{
	".mp3": true, ".wav": true, ".flac": true, ".m4a": true, ".aac": true, ".ogg": true, ".opus": true, ".wma": true,
	".mp4": true, ".m4v": true, ".mov": true, ".mkv": true, ".webm": true, ".avi": true, ".wmv": true,
}

func isMedia(path string) bool { return mediaExtensions[strings.ToLower(filepath.Ext(path))] }

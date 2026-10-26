package player

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
)

// Searches and lyric fetches go through these, so a test can answer
// without the network.
var (
	searchSongs = netease.Search
	fetchLyrics = netease.Lyrics
)

func LocalLyrics(path string) ([]lyrics.Line, error) {
	ext := filepath.Ext(path)
	b, err := os.ReadFile(strings.TrimSuffix(path, ext) + ".lrc")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return lyrics.Build(string(b), "", ""), nil
}

// localSyncedLyrics is a local file's lyrics: the same song on NetEase,
// word-timed and translated as an online song is, or the sidecar .lrc
// when that song cannot be found.
func localSyncedLyrics(path string, durationMs int64) []lyrics.Line {
	title, artists := localIdentity(path)
	if title != "" {
		if lines := lyricsFromNetEase(title, artists, durationMs); len(lines) > 0 {
			return lines
		}
	}
	lines, _ := LocalLyrics(path)
	return lines
}

func lyricsFromNetEase(title string, artists []string, durationMs int64) []lyrics.Line {
	hits, err := searchSongs(lyricQuery(title, artists), 10)
	if err != nil {
		return nil
	}
	hit, ok := pickLyricTrack(title, artists, durationMs, hits)
	if !ok {
		return nil
	}
	lrc, trans, yrc, err := fetchLyrics(hit.ID)
	if err != nil {
		return nil
	}
	return lyrics.Build(lrc, trans, yrc)
}

func lyricQuery(title string, artists []string) string {
	if len(artists) == 0 {
		return title
	}
	return title + " " + artists[0]
}

// lyricSyncSlack is how far a local file's length may sit from a NetEase
// song's and still be the same recording. Past that, the lines would not
// meet the singing.
const lyricSyncSlack = 8 * 1000

// pickLyricTrack chooses the hit that is this song: the same title, an
// artist in common, and about the same length.
func pickLyricTrack(title string, artists []string, durationMs int64, hits []netease.Track) (netease.Track, bool) {
	bestScore := 0
	var best netease.Track
	found := false
	for _, hit := range hits {
		if hit.ID <= 0 {
			continue
		}
		ts := titleScore(title, hit)
		if ts < 7 {
			continue
		}
		as := artistScore(artists, hit.Artists)
		if len(artists) > 0 && as == 0 {
			continue
		}
		score := ts + as
		if durationMs > 0 && hit.Duration > 0 {
			d := durationMs - hit.Duration
			if d < 0 {
				d = -d
			}
			if d > lyricSyncSlack {
				continue
			}
			if d <= 3000 {
				score += 3
			}
		}
		if !found || score > bestScore {
			best, bestScore, found = hit, score, true
		}
	}
	return best, found
}

func titleScore(want string, hit netease.Track) int {
	best := 0
	for _, name := range append([]string{hit.Name}, hit.Alias...) {
		w, g := foldName(want), foldName(name)
		switch {
		case w != "" && w == g:
			best = max(best, 10)
		case foldName(baseTitle(want)) != "" && foldName(baseTitle(want)) == foldName(baseTitle(name)):
			best = max(best, 7)
		}
	}
	return best
}

func artistScore(want []string, got []netease.Artist) int {
	if len(want) == 0 || len(got) == 0 {
		return 0
	}
	n := 0
	for _, w := range want {
		fw := foldName(w)
		if fw == "" {
			continue
		}
		for _, g := range got {
			fg := foldName(g.Name)
			if fg == fw || strings.Contains(fg, fw) || strings.Contains(fw, fg) {
				n++
				break
			}
		}
	}
	if n == 0 {
		return 0
	}
	return 6
}

// baseTitle drops a trailing parenthetical, as "(Live)" or "(英文版)".
func baseTitle(s string) string {
	s = strings.TrimSpace(s)
	for {
		rs := []rune(s)
		if len(rs) == 0 || (rs[len(rs)-1] != ')' && rs[len(rs)-1] != '）') {
			return s
		}
		open := -1
		for i := len(rs) - 1; i >= 0; i-- {
			if rs[i] == '(' || rs[i] == '（' {
				open = i
				break
			}
		}
		if open < 0 {
			return s
		}
		s = strings.TrimSpace(string(rs[:open]))
	}
}

func foldName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

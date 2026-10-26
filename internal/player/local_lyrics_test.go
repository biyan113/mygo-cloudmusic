package player

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"netease-music-mygo/internal/netease"
)

func TestLocalLyricsReadsSidecarLRC(t *testing.T) {
	// A same-basename LRC file uses the existing timed-lyrics model.
	path := filepath.Join(t.TempDir(), "song.mp3")
	if err := os.WriteFile(path[:len(path)-4]+".lrc", []byte("[00:01.00]hello\n[00:03.50]world\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lines, err := LocalLyrics(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 || lines[0].Text != "hello" || lines[1].Text != "world" || lines[1].At != 3500*time.Millisecond {
		t.Errorf("lines = %+v", lines)
	}
}

func TestLocalLyricsAllowsMissingFile(t *testing.T) {
	// Songs without a sidecar lyric file keep the normal empty lyrics view.
	lines, err := LocalLyrics(filepath.Join(t.TempDir(), "song.flac"))
	if err != nil || len(lines) != 0 {
		t.Errorf("lines = %v, err = %v", lines, err)
	}
}

func TestLocalIdentityReadsTagsAndFilename(t *testing.T) {
	// Tags name the song; a file without them is read from "index.Artist - Title".
	path := filepath.Join(t.TempDir(), "1.wrong - name.mp3")
	if err := os.WriteFile(path, id3Tag("Stay With You (英文版)", "林俊杰/孙燕姿"), 0o600); err != nil {
		t.Fatal(err)
	}
	title, artists := localIdentity(path)
	if title != "Stay With You (英文版)" || len(artists) != 2 || artists[0] != "林俊杰" || artists[1] != "孙燕姿" {
		t.Fatalf("identity = %q %v", title, artists)
	}
	title, artists = localIdentity(filepath.Join(t.TempDir(), "10.Billie Eilish - BIRDS OF A FEATHER.mp3"))
	if title != "BIRDS OF A FEATHER" || len(artists) != 1 || artists[0] != "Billie Eilish" {
		t.Fatalf("filename identity = %q %v", title, artists)
	}
}

func TestPickLyricTrackUsesArtistAndDuration(t *testing.T) {
	// The lyrics have to belong to this recording, not another song of the same name.
	hits := []netease.Track{
		{ID: 1, Name: "江南", Artists: []netease.Artist{{Name: "林俊杰"}}, Duration: 180000},
		{ID: 2, Name: "江南 (Live)", Artists: []netease.Artist{{Name: "林俊杰"}}, Duration: 320000},
		{ID: 3, Name: "江南", Artists: []netease.Artist{{Name: "其他"}}, Duration: 181000},
		{ID: 4, Name: "Stay With You", Artists: []netease.Artist{{Name: "林俊杰"}}, Duration: 200000},
		{ID: 5, Name: "Stay With You (英文版)", Artists: []netease.Artist{{Name: "林俊杰"}, {Name: "孙燕姿"}}, Duration: 200000},
	}
	got, ok := pickLyricTrack("江南", []string{"林俊杰"}, 181500, hits)
	if !ok || got.ID != 1 {
		t.Fatalf("studio = %+v, %v", got, ok)
	}
	if _, ok := pickLyricTrack("江南", []string{"林俊杰"}, 400000, hits); ok {
		t.Error("a much longer recording was accepted")
	}
	got, ok = pickLyricTrack("Stay With You (英文版)", []string{"林俊杰", "孙燕姿"}, 200100, hits)
	if !ok || got.ID != 5 {
		t.Fatalf("version = %+v, %v", got, ok)
	}
}

func TestLocalLyricsUsesMatchedNetEaseSong(t *testing.T) {
	// A local file takes the NetEase lyrics of the song with the same name, artist and length.
	realSearch, realLyrics := searchSongs, fetchLyrics
	searchSongs = func(query string, limit int) ([]netease.Track, error) {
		if query != "小船 163braces" || limit != 10 {
			t.Errorf("query = %q, limit = %d", query, limit)
		}
		return []netease.Track{
			{ID: 9, Name: "小船", Artists: []netease.Artist{{Name: "别人"}}, Duration: 184000},
			{ID: 42, Name: "小船", Artists: []netease.Artist{{Name: "163braces"}}, Duration: 184000},
		}, nil
	}
	fetchLyrics = func(id int64) (string, string, string, error) {
		if id != 42 {
			t.Errorf("lyrics id = %d", id)
		}
		return "[00:02.00]网易\n", "[00:02.00]cloud\n", "", nil
	}
	defer func() { searchSongs, fetchLyrics = realSearch, realLyrics }()
	path := filepath.Join(t.TempDir(), "1.163braces - 小船.mp3")
	if err := os.WriteFile(path, []byte("not audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path[:len(path)-4]+".lrc", []byte("[00:01.00]local\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lines := localSyncedLyrics(path, 184200)
	if len(lines) != 1 || lines[0].Text != "网易" || lines[0].Translation != "cloud" {
		t.Fatalf("lines = %+v", lines)
	}
}

func TestLocalLyricsFallsBackToSidecar(t *testing.T) {
	// When NetEase has no matching song, the sidecar lyric file still shows.
	realSearch, realLyrics := searchSongs, fetchLyrics
	searchSongs = func(string, int) ([]netease.Track, error) {
		return nil, os.ErrClosed
	}
	fetchLyrics = func(int64) (string, string, string, error) {
		t.Fatal("lyrics fetched without a match")
		return "", "", "", nil
	}
	defer func() { searchSongs, fetchLyrics = realSearch, realLyrics }()
	path := filepath.Join(t.TempDir(), "1.163braces - 小船.mp3")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path[:len(path)-4]+".lrc", []byte("[00:01.00]hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lines := localSyncedLyrics(path, 1000)
	if len(lines) != 1 || lines[0].Text != "hello" {
		t.Fatalf("lines = %+v", lines)
	}
}

func id3Tag(title, artist string) []byte {
	tag := append(id3TextFrame("TIT2", title), id3TextFrame("TPE1", artist)...)
	header := []byte{'I', 'D', '3', 3, 0, 0, 0, 0, 0, 0}
	header[6], header[7], header[8], header[9] = byte(len(tag)>>21), byte(len(tag)>>14), byte(len(tag)>>7), byte(len(tag))
	return append(header, tag...)
}

func id3TextFrame(id, text string) []byte {
	payload := append([]byte{3}, text...)
	frame := append([]byte(id), 0, 0, 0, 0, 0, 0)
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
	return append(frame, payload...)
}

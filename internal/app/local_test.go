package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/player"
)

func TestLocalLibraryAddsEachFileOnce(t *testing.T) {
	// Duplicate selections must not create duplicate rows or queue entries.
	a := newTestApp()
	mp3 := "../player/testdata/seek.mp3"
	a.addLocalPaths([]string{mp3, mp3, "../player/testdata/seek.wav"}, false)
	if len(a.local) != 1 || !a.local[0].IsLocal() || a.local[0].ID >= 0 {
		t.Fatalf("local tracks = %+v", a.local)
	}
	if a.localErr == "" {
		t.Error("unsupported selection was not reported")
	}
	a.addLocalPaths([]string{"../player/testdata/seek.flac"}, false)
	if len(a.local) != 2 {
		t.Fatalf("appending produced %d tracks", len(a.local))
	}
}

func TestLocalLibraryPlaysAddedQueue(t *testing.T) {
	// Starting an added song keeps the other local songs in the play queue.
	a := newTestApp()
	a.addLocalPaths([]string{"../player/testdata/seek.mp3"}, false)
	a.addLocalPaths([]string{"../player/testdata/seek.flac"}, true)
	if got := a.pl.Current(); got == nil || got.Name != "seek" || got.LocalPath != a.local[1].LocalPath {
		t.Fatalf("current = %+v", got)
	}
	if len(a.pl.Queue()) != 2 || !a.pl.Playing() {
		t.Errorf("queue = %d, playing = %v", len(a.pl.Queue()), a.pl.Playing())
	}
}

func TestLocalImportParsesBeforePosting(t *testing.T) {
	// File decoding belongs on the dialog goroutine, before main-thread state changes.
	real := makeLocalTrack
	var events []string
	makeLocalTrack = func(path string, id int64) (netease.Track, error) {
		events = append(events, "parse")
		return player.LocalTrack(path, id)
	}
	defer func() { makeLocalTrack = real }()
	a := newTestApp()
	a.importLocalPaths([]string{"../player/testdata/seek.mp3"}, false, func(fn func()) {
		events = append(events, "post")
		fn()
	})
	if !reflect.DeepEqual(events, []string{"parse", "post"}) {
		t.Errorf("events = %v", events)
	}
}

func TestLocalTrackLeavesCommentsPane(t *testing.T) {
	// Switching from online comments to a local song must not leave a comments pane.
	a := newTestApp()
	a.settings = defaultSettings()
	tr, err := player.LocalTrack("../player/testdata/seek.mp3", -1)
	if err != nil {
		t.Fatal(err)
	}
	a.pl.Cue([]netease.Track{tr})
	a.nowPlaying, a.npPane = true, paneComments
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if a.npPane == paneComments {
		t.Error("local track kept the comments pane open")
	}
}

func TestLocalRowClickPlaysQueue(t *testing.T) {
	// Clicking a local row starts playback just like an online track row.
	a := newTestApp()
	a.settings = defaultSettings()
	a.addLocalPaths([]string{"../player/testdata/seek.mp3"}, false)
	a.route = route{page: pageLocal}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("seek"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := a.pl.Current(); got == nil || got.LocalPath != a.local[0].LocalPath {
		t.Errorf("current = %+v", got)
	}
}

func TestLocalFolderFindsNestedAudio(t *testing.T) {
	// A folder scan keeps nested MP3 and FLAC, in path order, and leaves other files out.
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"notes.txt",
		"skip.wav",
		filepath.Join("nested", "cover.jpg"),
		"z.flac",
		filepath.Join("nested", "a.MP3"),
		"C.Flac",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "not-a-song.mp3"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := localAudioPaths(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(dir, "C.Flac"),
		filepath.Join(dir, "nested", "a.MP3"),
		filepath.Join(dir, "z.flac"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("paths = %v, want %v", got, want)
	}
}

func TestLocalFolderImportParsesBeforePosting(t *testing.T) {
	// Scanning a folder and reading its files happen before the library changes.
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	copyFile(t, "../player/testdata/seek.flac", filepath.Join(dir, "b.flac"))
	copyFile(t, "../player/testdata/seek.mp3", filepath.Join(nested, "a.mp3"))
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	real := makeLocalTrack
	var events []string
	makeLocalTrack = func(path string, id int64) (netease.Track, error) {
		events = append(events, "parse")
		return player.LocalTrack(path, id)
	}
	defer func() { makeLocalTrack = real }()
	a := newTestApp()
	a.importLocalFolder(dir, true, func(fn func()) {
		events = append(events, "post")
		fn()
	})
	if !reflect.DeepEqual(events, []string{"parse", "parse", "post"}) {
		t.Fatalf("events = %v", events)
	}
	if len(a.local) != 2 || a.local[0].Name != "b" || a.local[1].Name != "a" {
		t.Fatalf("local = %+v", a.local)
	}
	if got := a.pl.Current(); got == nil || got.LocalPath != a.local[0].LocalPath || !a.pl.Playing() {
		t.Errorf("current = %+v, playing = %v", got, a.pl.Playing())
	}
	a.importLocalFolder(dir, true, func(fn func()) { fn() })
	if len(a.local) != 2 {
		t.Errorf("importing the same folder again produced %d tracks", len(a.local))
	}
}

func TestLocalFolderReportsWhenNothingPlays(t *testing.T) {
	// A folder with no supported audio, or a missing folder, reports an error and adds nothing.
	a := newTestApp()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	a.importLocalFolder(dir, true, func(fn func()) { fn() })
	if len(a.local) != 0 || a.localErr == "" || a.pl.Current() != nil {
		t.Fatalf("local = %+v, err = %q, current = %+v", a.local, a.localErr, a.pl.Current())
	}
	a.localErr = ""
	a.importLocalFolder(filepath.Join(dir, "gone"), false, func(fn func()) { fn() })
	if len(a.local) != 0 || a.localErr == "" {
		t.Fatalf("local = %+v, err = %q", a.local, a.localErr)
	}
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	b, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

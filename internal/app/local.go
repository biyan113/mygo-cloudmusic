package app

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/egoist/mygo"

	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/player"
)

var makeLocalTrack = player.LocalTrack

func readLocalTracks(paths []string) ([]netease.Track, error) {
	tracks := make([]netease.Track, 0, len(paths))
	var lastErr error
	for _, path := range paths {
		tr, err := makeLocalTrack(path, -1)
		if err != nil {
			lastErr = err
			continue
		}
		tracks = append(tracks, tr)
	}
	return tracks, lastErr
}

func (a *app) addLocalTracks(tracks []netease.Track, err error, play bool) {
	seen := make(map[string]bool, len(a.local))
	for _, tr := range a.local {
		seen[tr.LocalPath] = true
	}
	first := -1
	a.localErr = ""
	if err != nil {
		a.localErr = err.Error()
	}
	for _, tr := range tracks {
		if seen[tr.LocalPath] {
			continue
		}
		a.nextLocalID--
		tr.ID = a.nextLocalID
		if first < 0 {
			first = len(a.local)
		}
		a.local = append(a.local, tr)
		seen[tr.LocalPath] = true
	}
	if play && first >= 0 {
		a.pl.PlayQueue(a.local, first)
	}
}

func (a *app) addLocalPaths(paths []string, play bool) {
	tracks, err := readLocalTracks(paths)
	a.addLocalTracks(tracks, err, play)
}

func (a *app) importLocalPaths(paths []string, play bool, post func(func())) {
	tracks, err := readLocalTracks(paths)
	post(func() { a.addLocalTracks(tracks, err, play) })
}

// localAudioPaths lists the MP3 and FLAC files under root, including nested
// folders, in path order. Other files stay out.
func localAudioPaths(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".mp3", ".flac":
			paths = append(paths, path)
		}
		return nil
	})
	sort.Strings(paths)
	return paths, err
}

func (a *app) importLocalFolder(root string, play bool, post func(func())) {
	paths, err := localAudioPaths(root)
	if err != nil {
		post(func() { a.localErr = err.Error() })
		return
	}
	if len(paths) == 0 {
		post(func() { a.localErr = "没有找到 MP3 或 FLAC 文件" })
		return
	}
	a.importLocalPaths(paths, play, post)
}

func (a *app) chooseLocal() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:      a.win,
		Title:       "打开本地音乐",
		ButtonLabel: "添加",
		Message:     "选择要播放的 MP3 或 FLAC 文件",
		Filters:     []mygo.FileFilter{{Name: "音频文件", Extensions: []string{"mp3", "flac"}}},
		Multiple:    true,
	})
	if err != nil {
		a.post(func() { a.localErr = err.Error() })
		return
	}
	if len(paths) > 0 {
		a.importLocalPaths(paths, true, a.post)
	}
}

func (a *app) chooseLocalFolder() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:      a.win,
		Title:       "打开音乐文件夹",
		ButtonLabel: "添加",
		Message:     "选择包含 MP3 或 FLAC 的文件夹",
		Directory:   true,
	})
	if err != nil {
		a.post(func() { a.localErr = err.Error() })
		return
	}
	if len(paths) > 0 {
		a.importLocalFolder(paths[0], true, a.post)
	}
}

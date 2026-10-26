package player

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"netease-music-mygo/internal/netease"
)

// LocalTrack describes an audio file on this computer.
func LocalTrack(path string, id int64) (netease.Track, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".mp3" && ext != ".flac" {
		return netease.Track{}, errors.New("仅支持 MP3 和 FLAC 文件")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return netease.Track{}, err
	}
	s, err := openLocalFile(path, 0)
	if err != nil {
		return netease.Track{}, err
	}
	defer s.Close()
	if id >= 0 {
		id = -id
		if id == 0 {
			id = -1
		}
	}
	return netease.Track{
		ID:        id,
		Name:      strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Artists:   []netease.Artist{{Name: "本地音乐"}},
		Album:     netease.Album{Name: "本地音乐", PicURL: LocalArtworkScheme + path},
		Duration:  s.Len() * 1000 / int64(s.SampleRate()),
		LocalPath: path,
	}, nil
}

func openLocalFile(path string, durationMs int64) (stream, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		f.Close()
		if err == nil {
			err = fmt.Errorf("不是普通音频文件: %s", path)
		}
		return nil, err
	}
	dl := opened(f, st.Size())
	var index []mp3Frame
	if strings.EqualFold(filepath.Ext(path), ".mp3") {
		var scannedMs int64
		scannedMs, index = localMP3Index(dl)
		if durationMs == 0 {
			durationMs = scannedMs
		}
	}
	s, err := openStream(dl, durationMs)
	if err != nil {
		dl.Close()
		return nil, err
	}
	if mp3, ok := s.(*mp3Stream); ok {
		mp3.index = index
	}
	return s, nil
}

func localMP3Index(dl *download) (int64, []mp3Frame) {
	off := id3Size(dl)
	var samples, rate int64
	var index []mp3Frame
	for {
		if dl.waitFor(off+4) < off+4 {
			break
		}
		h := dl.peek(off, 4)
		size := frameSize(h)
		if size <= 0 {
			break
		}
		index = append(index, mp3Frame{off: off, at: samples})
		version := (h[1] >> 3) & 3
		rate = int64(rates[(h[2]>>2)&3])
		if version == 2 {
			rate /= 2
		} else if version == 0 {
			rate /= 4
		}
		if version == 3 {
			samples += 1152
		} else {
			samples += 576
		}
		off += int64(size)
	}
	if rate == 0 {
		return 0, index
	}
	return samples * 1000 / rate, index
}

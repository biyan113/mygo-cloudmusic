package player

import (
	"encoding/json"
	"testing"

	"netease-music-mygo/internal/netease"
)

func TestLocalTrackReadsNameAndDuration(t *testing.T) {
	// Local files need enough metadata for the track list and progress bar.
	for _, ext := range []string{"mp3", "flac"} {
		tr, err := LocalTrack("testdata/seek."+ext, 7)
		if err != nil {
			t.Fatalf("LocalTrack(%s): %v", ext, err)
		}
		if tr.Name != "seek" || tr.ID != -7 || !tr.IsLocal() || tr.Duration <= 0 {
			t.Errorf("LocalTrack(%s) = %+v", ext, tr)
		}
	}
}

func TestLocalTrackRejectsUnsupportedAndMissingFiles(t *testing.T) {
	// Only formats the existing decoders understand may enter the queue.
	if _, err := LocalTrack("testdata/seek.wav", 1); err == nil {
		t.Error("unsupported extension was accepted")
	}
	if _, err := LocalTrack("testdata/missing.mp3", 1); err == nil {
		t.Error("missing file was accepted")
	}
}

func TestLocalTrackOpensAsAudio(t *testing.T) {
	// A local track must use the real decoder rather than the NetEase cache.
	tr, err := LocalTrack("testdata/seek.mp3", 1)
	if err != nil {
		t.Fatal(err)
	}
	p := &Player{}
	_, s, err := p.open(tr, "exhigh")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if n, err := s.Read(make([]float32, 4096)); n == 0 || err != nil {
		t.Errorf("Read = %d, %v", n, err)
	}
}

func TestLocalMP3SeekUsesFrameIndex(t *testing.T) {
	// Variable-bitrate files must seek by frame time, not by byte proportion.
	s := &mp3Stream{index: []mp3Frame{{off: 100, at: 0}, {off: 1000, at: 1152}, {off: 8000, at: 2304}}}
	if off := s.seekOffset(6000); off != 1000 {
		t.Errorf("seekOffset(6000) = %d, want 1000", off)
	}
}

func TestLocalPathSurvivesSessionJSON(t *testing.T) {
	// Restoring playback must not turn a local file into a NetEase track.
	tr, err := LocalTrack("testdata/seek.flac", 1)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(Session{Queue: []netease.Track{tr}})
	if err != nil {
		t.Fatal(err)
	}
	var restored Session
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Queue) != 1 || restored.Queue[0].LocalPath != tr.LocalPath {
		t.Errorf("restored queue = %+v", restored.Queue)
	}
}

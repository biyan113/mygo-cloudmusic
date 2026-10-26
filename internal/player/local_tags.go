package player

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/mewkiz/flac"
	"github.com/mewkiz/flac/meta"
)

var indexedFile = regexp.MustCompile(`^\d+\.(.+)$`)

// localIdentity is the title and artists to look a file up by: its tags,
// or, when those are missing, a name like "12.Artist - Title".
func localIdentity(path string) (title string, artists []string) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
		title, artists = id3Identity(path)
	case ".flac":
		title, artists = flacIdentity(path)
	}
	if title != "" {
		return title, artists
	}
	return filenameIdentity(path)
}

func filenameIdentity(path string) (string, []string) {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if m := indexedFile.FindStringSubmatch(name); m != nil {
		name = m[1]
	}
	artist, title, ok := strings.Cut(name, " - ")
	if !ok {
		return strings.TrimSpace(name), nil
	}
	return strings.TrimSpace(title), splitArtists(artist)
}

func splitArtists(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '/' || r == ';' || r == '；' || r == '、' || r == '&'
	})
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func id3Identity(path string) (string, []string) {
	f, err := os.Open(path)
	if err != nil {
		return "", nil
	}
	defer f.Close()
	var h [10]byte
	if _, err := f.ReadAt(h[:], 0); err != nil || string(h[:3]) != "ID3" || h[3] < 3 {
		return "", nil
	}
	size := int64(h[6]&0x7f)<<21 | int64(h[7]&0x7f)<<14 | int64(h[8]&0x7f)<<7 | int64(h[9]&0x7f)
	if size <= 0 || size > maxArtworkSize {
		return "", nil
	}
	r := io.NewSectionReader(f, 10, size)
	var title, artist string
	for r.Size() >= 10 && (title == "" || artist == "") {
		var fh [10]byte
		if _, err := io.ReadFull(r, fh[:]); err != nil {
			break
		}
		n := int64(binary.BigEndian.Uint32(fh[4:8]))
		if h[3] == 4 {
			n = int64(fh[4]&0x7f)<<21 | int64(fh[5]&0x7f)<<14 | int64(fh[6]&0x7f)<<7 | int64(fh[7]&0x7f)
		}
		if n < 0 || n > r.Size() {
			break
		}
		id := string(fh[:4])
		if id != "TIT2" && id != "TPE1" {
			if _, err := r.Seek(n, io.SeekCurrent); err != nil {
				break
			}
			continue
		}
		payload := make([]byte, n)
		if _, err := io.ReadFull(r, payload); err != nil {
			break
		}
		text := strings.Join(id3Texts(payload), "/")
		if id == "TIT2" && title == "" {
			title = text
		}
		if id == "TPE1" && artist == "" {
			artist = text
		}
	}
	return strings.TrimSpace(title), splitArtists(artist)
}

func id3Texts(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	enc, data := b[0], b[1:]
	var parts []string
	switch enc {
	case 1, 2:
		parts = strings.Split(decodeUTF16(enc, data), "\x00")
	default:
		for _, c := range bytes.Split(data, []byte{0}) {
			if len(c) == 0 {
				continue
			}
			s := string(c)
			if enc == 0 && !utf8.ValidString(s) {
				s = latin1(c)
			}
			parts = append(parts, s)
		}
	}
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func decodeUTF16(enc byte, b []byte) string {
	if enc == 1 && len(b) >= 2 && b[0] == 0xFE && b[1] == 0xFF {
		return utf16String(b[2:], binary.BigEndian)
	}
	if enc == 1 && len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE {
		return utf16String(b[2:], binary.LittleEndian)
	}
	var order binary.ByteOrder = binary.BigEndian
	if enc == 1 {
		order = binary.LittleEndian
	}
	return utf16String(b, order)
}

func utf16String(b []byte, order binary.ByteOrder) string {
	if len(b)%2 == 1 {
		b = b[:len(b)-1]
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = order.Uint16(b[2*i:])
	}
	return string(utf16.Decode(u))
}

func latin1(b []byte) string {
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return string(r)
}

func flacIdentity(path string) (string, []string) {
	f, err := os.Open(path)
	if err != nil {
		return "", nil
	}
	defer f.Close()
	st, err := flac.Parse(f)
	if err != nil {
		return "", nil
	}
	var title, artist string
	for _, block := range st.Blocks {
		vc, ok := block.Body.(*meta.VorbisComment)
		if !ok {
			continue
		}
		for _, tag := range vc.Tags {
			switch strings.ToUpper(tag[0]) {
			case "TITLE":
				if title == "" {
					title = tag[1]
				}
			case "ARTIST":
				if artist == "" {
					artist = tag[1]
				}
			}
		}
	}
	return strings.TrimSpace(title), splitArtists(artist)
}

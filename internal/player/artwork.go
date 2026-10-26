package player

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mewkiz/flac"
	"github.com/mewkiz/flac/meta"
)

const LocalArtworkScheme = "local-art:"

const maxArtworkSize = 128 << 20

func LocalArtwork(path string) ([]byte, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return readAPIC(f)
	case ".flac":
		return readFLACPicture(path)
	default:
		return nil, errors.New("不支持的本地音频格式")
	}
}

func readAPIC(f *os.File) ([]byte, error) {
	var h [10]byte
	if _, err := f.ReadAt(h[:], 0); err != nil {
		return nil, err
	}
	if string(h[:3]) != "ID3" || h[3] < 3 {
		return nil, errors.New("没有内嵌封面")
	}
	size := int64(h[6]&0x7f)<<21 | int64(h[7]&0x7f)<<14 | int64(h[8]&0x7f)<<7 | int64(h[9]&0x7f)
	if size <= 0 || size > maxArtworkSize {
		return nil, errors.New("无效的 ID3 标签大小")
	}
	r := io.NewSectionReader(f, 10, size)
	var fallback []byte
	for r.Size() >= 10 {
		var fh [10]byte
		if _, err := io.ReadFull(r, fh[:]); err != nil {
			return nil, err
		}
		n := int64(binary.BigEndian.Uint32(fh[4:8]))
		if h[3] == 4 {
			n = int64(fh[4]&0x7f)<<21 | int64(fh[5]&0x7f)<<14 | int64(fh[6]&0x7f)<<7 | int64(fh[7]&0x7f)
		}
		if n <= 0 || n > r.Size() || n > maxArtworkSize {
			return nil, errors.New("无效的 APIC 帧大小")
		}
		if string(fh[:4]) != "APIC" || fh[9]&3 != 0 {
			if _, err := r.Seek(n, io.SeekCurrent); err != nil {
				return nil, err
			}
			continue
		}
		payload := make([]byte, n)
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, err
		}
		kind, img, err := apicImage(payload)
		if err != nil {
			continue
		}
		if kind == 3 {
			return img, nil
		}
		if fallback == nil {
			fallback = img
		}
	}
	if fallback != nil {
		return fallback, nil
	}
	return nil, errors.New("没有内嵌封面")
}

// apicImage reads an ID3 APIC frame: a text encoding, a MIME type, the
// picture type, a description, then the image. The description follows the
// encoding, so a JPEG begins only after that terminator.
func apicImage(b []byte) (byte, []byte, error) {
	if len(b) < 4 {
		return 0, nil, errors.New("无效的 APIC 数据")
	}
	encoding := b[0]
	_, rest, ok := cutTerminated(b[1:], 0)
	if !ok || len(rest) < 2 {
		return 0, nil, errors.New("无效的 APIC 数据")
	}
	kind := rest[0]
	_, img, ok := cutTerminated(rest[1:], encoding)
	if !ok || len(img) == 0 {
		return 0, nil, errors.New("APIC 中没有图片")
	}
	return kind, img, nil
}

func cutTerminated(b []byte, encoding byte) ([]byte, []byte, bool) {
	if encoding == 1 || encoding == 2 {
		for i := 0; i+1 < len(b); i += 2 {
			if b[i] == 0 && b[i+1] == 0 {
				return b[:i], b[i+2:], true
			}
		}
		return nil, nil, false
	}
	if i := strings.IndexByte(string(b), 0); i >= 0 {
		return b[:i], b[i+1:], true
	}
	return nil, nil, false
}

func readFLACPicture(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := flac.Parse(f)
	if err != nil {
		return nil, err
	}
	var fallback []byte
	for _, block := range st.Blocks {
		pic, ok := block.Body.(*meta.Picture)
		if !ok || len(pic.Data) == 0 {
			continue
		}
		if pic.Type == 3 {
			return pic.Data, nil
		}
		if fallback == nil {
			fallback = pic.Data
		}
	}
	if fallback != nil {
		return fallback, nil
	}
	return nil, errors.New("没有内嵌封面")
}

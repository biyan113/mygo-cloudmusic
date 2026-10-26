package player

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"netease-music-mygo/internal/netease"
)

func TestLocalArtworkReadsMP3FrontCover(t *testing.T) {
	// APIC artwork must come from the file, preferring the front cover.
	img := testPNG(t)
	path := filepath.Join(t.TempDir(), "cover.mp3")
	audio, err := os.ReadFile("testdata/seek.mp3")
	if err != nil {
		t.Fatal(err)
	}
	tag := append(apicFrame(4, []byte("back cover")), apicFrame(3, img)...)
	header := []byte{'I', 'D', '3', 3, 0, 0, 0, 0, 0, 0}
	header[6], header[7], header[8], header[9] = byte(len(tag)>>21), byte(len(tag)>>14), byte(len(tag)>>7), byte(len(tag))
	data := append(header, tag...)
	data = append(data, audio...)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LocalArtwork(path)
	if err != nil || !bytes.Equal(got, img) {
		t.Fatalf("LocalArtwork = %d bytes, %v", len(got), err)
	}
	if _, _, err := image.Decode(bytes.NewReader(got)); err != nil {
		t.Errorf("decoded cover: %v", err)
	}
}

func TestLocalArtworkReadsFLACPicture(t *testing.T) {
	// FLAC Picture blocks must feed the same cover cache as MP3 APIC frames.
	img := testPNG(t)
	audio, err := os.ReadFile("testdata/seek.flac")
	if err != nil {
		t.Fatal(err)
	}
	start, err := flacDataStart(downloaded(audio))
	if err != nil {
		t.Fatal(err)
	}
	data := append([]byte(nil), audio[:42]...)
	data[4] &= 0x7f
	body := flacPicture(img)
	header := []byte{0x86, byte(len(body) >> 16), byte(len(body) >> 8), byte(len(body))}
	data = append(data, header...)
	data = append(data, body...)
	data = append(data, audio[start:]...)
	path := filepath.Join(t.TempDir(), "cover.flac")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LocalArtwork(path)
	if err != nil || !bytes.Equal(got, img) {
		t.Fatalf("LocalArtwork = %d bytes, %v", len(got), err)
	}
}

func TestLocalArtworkHandlesMissingAndMalformedMetadata(t *testing.T) {
	// Missing or broken metadata must leave the normal placeholder intact.
	if _, err := LocalArtwork("testdata/seek.mp3"); err == nil {
		t.Error("file without artwork returned no error")
	}
	path := filepath.Join(t.TempDir(), "broken.mp3")
	if err := os.WriteFile(path, []byte("ID3\x04\x00\x00\x00\x00\x00\x20APIC"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LocalArtwork(path); err == nil {
		t.Error("truncated APIC returned no error")
	}
}

func TestLocalArtworkURLStaysLocal(t *testing.T) {
	// Resizing an online cover must not turn a local path into a remote URL.
	const key = "local-art:/tmp/song.mp3"
	if got := netease.Sized(key, 600); got != key {
		t.Errorf("Sized = %q", got)
	}
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestLocalArtworkSkipsDescription(t *testing.T) {
	// A UTF-16 description sits before the image and must not become part of it.
	img := testPNG(t)
	desc := []byte{0xff, 0xfe, 'A', 0, 'B', 0, 0, 0}
	payload := []byte{1}
	payload = append(payload, "image/png"...)
	payload = append(payload, 0, 3)
	payload = append(payload, desc...)
	payload = append(payload, img...)
	path := writeAPIC(t, payload)
	got, err := LocalArtwork(path)
	if err != nil || !bytes.Equal(got, img) {
		t.Fatalf("LocalArtwork = %d bytes, %v", len(got), err)
	}
}

func apicFrame(kind byte, img []byte) []byte {
	payload := []byte{0}
	payload = append(payload, "image/png"...)
	payload = append(payload, 0, kind, 0)
	payload = append(payload, img...)
	return apicFramePayload(payload)
}

func apicFramePayload(payload []byte) []byte {
	frame := []byte{'A', 'P', 'I', 'C', 0, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
	return append(frame, payload...)
}

func writeAPIC(t *testing.T, payload []byte) string {
	t.Helper()
	audio, err := os.ReadFile("testdata/seek.mp3")
	if err != nil {
		t.Fatal(err)
	}
	tag := apicFramePayload(payload)
	header := []byte{'I', 'D', '3', 3, 0, 0, 0, 0, 0, 0}
	header[6], header[7], header[8], header[9] = byte(len(tag)>>21), byte(len(tag)>>14), byte(len(tag)>>7), byte(len(tag))
	path := filepath.Join(t.TempDir(), "cover.mp3")
	if err := os.WriteFile(path, append(append(header, tag...), audio...), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func flacPicture(img []byte) []byte {
	var b bytes.Buffer
	for _, v := range []uint32{3, uint32(len("image/png"))} {
		_ = binary.Write(&b, binary.BigEndian, v)
	}
	b.WriteString("image/png")
	for _, v := range []uint32{0, 1, 1, 32, 0, uint32(len(img))} {
		_ = binary.Write(&b, binary.BigEndian, v)
	}
	b.Write(img)
	return b.Bytes()
}

package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestPNG creates a valid PNG of the given size and returns its bytes.
func writeTestPNG(t *testing.T, path string, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 0x80, A: 0xFF})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write png: %v", err)
	}
	return buf.Bytes()
}

// newTestApp builds an App plus a temporary packs root containing one pack.
func newTestApp(t *testing.T, packName string) (*App, string) {
	t.Helper()
	base := t.TempDir()
	packDir := filepath.Join(base, packName)
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatalf("mkdir pack: %v", err)
	}
	a := NewApp(false)
	a.settings["resourcePacksPath"] = base
	return a, base
}

func TestGetPackThumbReturnsScaledDataURI(t *testing.T) {
	a, base := newTestApp(t, "testpack")
	// Deliberately larger than the 128px cap so scaling is exercised.
	writeTestPNG(t, filepath.Join(base, "testpack", "textures", "items", "stick.png"), 256, 128)

	uri, err := a.GetPackThumb("testpack", "textures/items/stick.png", base)
	if err != nil {
		t.Fatalf("GetPackThumb: %v", err)
	}
	if !strings.HasPrefix(uri, "data:image/png;base64,") {
		t.Fatalf("expected png data URI, got %.40q", uri)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.SplitN(uri, ",", 2)[1])
	if err != nil {
		t.Fatalf("decode data uri: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	b := img.Bounds()
	// Output is always an exact square so the frontend can render it crisply.
	if b.Dx() != 128 || b.Dy() != 128 {
		t.Errorf("expected 128x128 thumbnail canvas, got %dx%d", b.Dx(), b.Dy())
	}
	// A 256x128 source scales to 128x64 and is centered, leaving transparent
	// bands top and bottom.
	pixR, pixG, pixB, _ := img.At(64, 8).RGBA()
	if pixR|pixG|pixB != 0 {
		t.Errorf("expected transparent band above the centered image, got rgba(%d,%d,%d)", pixR, pixG, pixB)
	}
	if _, _, _, alpha := img.At(64, 64).RGBA(); alpha == 0 {
		t.Errorf("expected opaque content at the center of the thumbnail")
	}
}

func TestGetPackThumbUsesCache(t *testing.T) {
	a, base := newTestApp(t, "testpack")
	pngPath := filepath.Join(base, "testpack", "textures", "ui", "icon.png")
	writeTestPNG(t, pngPath, 32, 32)

	first, err := a.GetPackThumb("testpack", "textures/ui/icon.png", base)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	// Delete the source: a cache hit must still succeed.
	if err := os.Remove(pngPath); err != nil {
		t.Fatalf("remove source: %v", err)
	}
	second, err := a.GetPackThumb("testpack", "textures/ui/icon.png", base)
	if err != nil {
		t.Fatalf("second call should hit cache, got %v", err)
	}
	if first != second {
		t.Errorf("cached result differs from original")
	}
}

func TestGetPackThumbRejectsBadInput(t *testing.T) {
	a, base := newTestApp(t, "testpack")
	writeTestPNG(t, filepath.Join(base, "testpack", "ok.png"), 8, 8)

	cases := []struct {
		name     string
		packName string
		rel      string
	}{
		{"traversal rel", "testpack", "../../../etc/passwd"},
		{"absolute rel", "testpack", filepath.Join(string(filepath.Separator), "etc", "passwd")},
		{"empty rel", "testpack", ""},
		{"traversal pack", "../escape", "ok.png"},
		{"pack with separator", "sub/dir", "ok.png"},
		{"missing pack", "nosuchpack", "ok.png"},
		{"missing file", "testpack", "nope.png"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if uri, err := a.GetPackThumb(tc.packName, tc.rel, base); err == nil {
				t.Errorf("expected error, got uri %.40q", uri)
			}
		})
	}
}

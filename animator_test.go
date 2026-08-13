package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func makeTestGIF(t *testing.T, frames int) []byte {
	t.Helper()
	palette := color.Palette{color.Transparent, color.RGBA{R: 255, A: 255}, color.RGBA{G: 255, A: 255}}
	var images []*image.Paletted
	var delays []int
	for i := 0; i < frames; i++ {
		img := image.NewPaletted(image.Rect(0, 0, 100, 80), palette)
		c := color.RGBA{R: 255, A: 255}
		if i%2 == 1 {
			c = color.RGBA{G: 255, A: 255}
		}
		for y := 10; y < 70; y++ {
			for x := 10; x < 90; x++ {
				img.Set(x, y, c)
			}
		}
		images = append(images, img)
		delays = append(delays, 10)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, &gif.GIF{Image: images, Delay: delays, LoopCount: 0}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipEntryNames(t *testing.T, zipPath string) (map[string][]byte, error) {
	t.Helper()
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	out := make(map[string][]byte)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		out[f.Name] = data
	}
	return out, nil
}

func TestBuildAnimatedPackTransparent(t *testing.T) {
	outDir := t.TempDir()
	path, err := buildAnimatedPack(makeTestGIF(t, 2), "cat.gif", "0.06", nil, false, true, "#000000", outDir, "desc", nil)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "cat.mcpack" {
		t.Fatalf("unexpected pack name: %s", path)
	}

	entries, err := zipEntryNames(t, path)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"manifest.json",
		"pack_icon.png",
		"ui/_global_variables.json",
		"ui/_ui_defs.json",
		"ui/inventory_screen.json",
		"textures/animated_ui/common/inventory_cell_image_red.json",
		"textures/animated_ui/common/inventory_cell_image_red.png",
		"textures/animated_ui/inventory_bg/inventory_overlay.png",
		"textures/animated_ui/inventory_bg/inventory_vertical_flipbook.png",
		"uidx/animated_ui/custom_anim_inventory_bg.uidx",
		"uidx/animated_ui/inventory_bg_base.uidx",
		"uidx/animated_ui/modified_inventory_screen.uidx",
		"uidx/inventory_screens/inventory_new_screen.uidx",
		"uidx/inventory_screens/inventory_old_screen.uidx",
		"uidx/inventory_screens/inventory_screen.uidx",
	}
	for _, name := range want {
		if _, ok := entries[name]; !ok {
			t.Errorf("missing zip entry: %s", name)
		}
	}
	for name := range entries {
		if strings.Contains(name, `\`) {
			t.Errorf("backslash in zip entry name: %s", name)
		}
	}

	vars := string(entries["ui/_global_variables.json"])
	if !strings.Contains(vars, `"$total_inventory_frames": "02_frames"`) {
		t.Errorf("frames not templated correctly:\n%s", vars)
	}
	if !strings.Contains(vars, `"$inventory_duration_per_frame": 0.06`) {
		t.Errorf("duration not templated correctly:\n%s", vars)
	}

	var manifest map[string]interface{}
	if err := json.Unmarshal(entries["manifest.json"], &manifest); err != nil {
		t.Fatalf("manifest is not valid json: %v", err)
	}
	header := manifest["header"].(map[string]interface{})
	if header["name"] != "cat" {
		t.Errorf("manifest name = %v, want cat", header["name"])
	}

	flipbook, err := png.Decode(bytes.NewReader(entries["textures/animated_ui/inventory_bg/inventory_vertical_flipbook.png"]))
	if err != nil {
		t.Fatal(err)
	}
	if flipbook.Bounds().Dx() != animFrameWidth || flipbook.Bounds().Dy() != animFrameHeight*2 {
		t.Errorf("flipbook size = %dx%d, want %dx%d", flipbook.Bounds().Dx(), flipbook.Bounds().Dy(), animFrameWidth, animFrameHeight*2)
	}
}

func TestBuildAnimatedPackSolidFillAndOverlay(t *testing.T) {
	overlay := image.NewRGBA(image.Rect(0, 0, 120, 90))
	for y := 0; y < 90; y++ {
		for x := 0; x < 120; x++ {
			overlay.Set(x, y, color.RGBA{R: 0, G: 0, B: 255, A: 255})
		}
	}
	var overlayBuf bytes.Buffer
	if err := png.Encode(&overlayBuf, overlay); err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	path, err := buildAnimatedPack(makeTestGIF(t, 1), "dog.gif", "0.07", overlayBuf.Bytes(), true, false, "#123456", outDir, "desc", nil)
	if err != nil {
		t.Fatal(err)
	}

	entries, err := zipEntryNames(t, path)
	if err != nil {
		t.Fatal(err)
	}

	overlayPng, err := png.Decode(bytes.NewReader(entries["textures/animated_ui/inventory_bg/inventory_overlay.png"]))
	if err != nil {
		t.Fatal(err)
	}
	if overlayPng.Bounds().Dx() != animFrameWidth || overlayPng.Bounds().Dy() != animFrameHeight {
		t.Errorf("overlay size = %dx%d, want %dx%d", overlayPng.Bounds().Dx(), overlayPng.Bounds().Dy(), animFrameWidth, animFrameHeight)
	}

	icon, err := png.Decode(bytes.NewReader(entries["pack_icon.png"]))
	if err != nil {
		t.Fatal(err)
	}
	if icon.Bounds().Dx() != animFrameWidth || icon.Bounds().Dy() != animFrameHeight {
		t.Errorf("pack icon size = %dx%d, want %dx%d", icon.Bounds().Dx(), icon.Bounds().Dy(), animFrameWidth, animFrameHeight)
	}

	// solid fill: corner pixel must be the fill color
	corner := icon.At(0, 0)
	r, g, b, _ := corner.RGBA()
	if r>>8 != 0x12 || g>>8 != 0x34 || b>>8 != 0x56 {
		t.Errorf("corner color = %x %x %x, want 123456", r>>8, g>>8, b>>8)
	}

	vars := string(entries["ui/_global_variables.json"])
	if !strings.Contains(vars, `"$total_inventory_frames": "01_frames"`) {
		t.Errorf("frames not templated correctly:\n%s", vars)
	}
}

func TestBuildAnimatedPackCapsAtForty(t *testing.T) {
	outDir := t.TempDir()
	path, err := buildAnimatedPack(makeTestGIF(t, 45), "long.gif", "0.05", nil, false, true, "#000000", outDir, "desc", nil)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := zipEntryNames(t, path)
	if err != nil {
		t.Fatal(err)
	}
	vars := string(entries["ui/_global_variables.json"])
	if !strings.Contains(vars, `"$total_inventory_frames": "40_frames"`) {
		t.Errorf("frames not capped at 40:\n%s", vars)
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetPackSkinThumbnails(t *testing.T) {
	skin, err := os.ReadFile("frontend/src/assets/default-skin.png")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	skinDir := filepath.Join(root, "withSkin", "textures", "entity")
	if err := os.MkdirAll(skinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skinDir, "steve.png"), skin, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "noSkin"), 0o755); err != nil {
		t.Fatal(err)
	}

	a := NewApp(false)
	a.settings["resourcePacksPath"] = root
	got := a.GetPackSkinThumbnails([]string{"withSkin", "noSkin", "../withSkin", ""})

	if len(got) != 1 {
		t.Fatalf("got thumbnails for %d packs, want 1: %v", len(got), keys(got))
	}
	if !strings.HasPrefix(got["withSkin"], "data:image/png;base64,") {
		t.Fatalf("withSkin thumbnail is not a PNG data URI")
	}
}

func TestRenderSkinThumbnailRejectsGarbage(t *testing.T) {
	if _, err := renderSkinThumbnail([]byte("not an image")); err == nil {
		t.Fatal("expected an error for non-image bytes")
	}
}

func keys(m map[string]string) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

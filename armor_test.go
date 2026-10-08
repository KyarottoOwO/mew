package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestGetPackArmorTexturesFallsBackToVanillaPerLayer(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	packLayer := []byte("pack diamond layer 1")
	vanillaLayer := []byte("vanilla diamond layer 2")

	// Seed the vanilla cache so the fallback never touches the network.
	cached := filepath.Join(vanillaCacheDir(), "textures", "models", "armor", "diamond_2.png")
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached, vanillaLayer, 0o644); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	armorDir := filepath.Join(root, "pack", "textures", "models", "armor")
	if err := os.MkdirAll(armorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(armorDir, "diamond_1.png"), packLayer, 0o644); err != nil {
		t.Fatal(err)
	}

	a := NewApp(false)
	a.settings["resourcePacksPath"] = root

	got, err := a.GetPackArmorTextures("pack", "diamond")
	if err != nil {
		t.Fatal(err)
	}
	if want := dataURI(packLayer); got.Layer1 != want {
		t.Errorf("layer 1 should come from the pack")
	}
	if want := dataURI(vanillaLayer); got.Layer2 != want {
		t.Errorf("layer 2 should fall back to vanilla")
	}

	naked, err := a.GetPackArmorTextures("pack", "naked")
	if err != nil {
		t.Fatal(err)
	}
	if naked.Layer1 != "" || naked.Layer2 != "" {
		t.Errorf("naked should have no armor, got %+v", naked)
	}
}

func dataURI(b []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b)
}

package main

import (
	"encoding/base64"
	"image/color"
	"path/filepath"
	"testing"
)

func TestGetPackArmorTexturesFallsBackToVanillaPerLayer(t *testing.T) {
	a := testApp(t)
	root := a.getResourcePacksPath()
	packLayer := solidPNG(t, 64, 32, color.NRGBA{255, 0, 0, 255})
	vanillaLayer := solidPNG(t, 64, 32, color.NRGBA{0, 255, 0, 255})

	// Seed the vanilla cache so the fallback never touches the network.
	writeFile(t, filepath.Join(vanillaCacheDir(), "textures", "models", "armor", "diamond_2.png"), vanillaLayer)
	writeFile(t, filepath.Join(root, "pack", "textures", "models", "armor", "diamond_1.png"), packLayer)

	got, err := a.GetPackArmorTextures("pack", "diamond")
	if err != nil {
		t.Fatal(err)
	}
	if img := decodeDataURI(t, got.Layer1); !isRed(img) {
		t.Errorf("layer 1 should come from the pack")
	}
	if img := decodeDataURI(t, got.Layer2); !isGreen(img) {
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

func TestGetHeldItemTexture(t *testing.T) {
	a := testApp(t)
	root := a.getResourcePacksPath()
	vanillaSword := solidPNG(t, 16, 16, color.NRGBA{255, 255, 0, 255})
	packSword := solidPNG(t, 16, 16, color.NRGBA{0, 0, 255, 255})

	writeFile(t, filepath.Join(vanillaCacheDir(), "textures", "items", "iron_sword.png"), vanillaSword)
	writeFile(t, filepath.Join(root, "pack", "textures", "items", "diamond_sword.png"), packSword)

	if uri, _ := a.GetHeldItemTexture("pack", "diamond_sword"); !isBlue(decodeDataURI(t, uri)) {
		t.Errorf("diamond_sword should come from the pack")
	}
	if uri, _ := a.GetHeldItemTexture("pack", "iron_sword"); !isYellow(decodeDataURI(t, uri)) {
		t.Errorf("iron_sword should fall back to vanilla")
	}
	if _, err := a.GetHeldItemTexture("pack", "../../secret"); err == nil {
		t.Errorf("expected an error for a name that isn't a tool")
	}
}

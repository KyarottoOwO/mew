package main

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"os"
	"strings"
	"testing"
)

func TestThumbDump(t *testing.T) {
	a := NewApp(false)
	a.settings["resourcePacksPath"] = `C:\Users\catto\AppData\Roaming\levilauncher.exe\versions\1.26.44.03\Minecraft Bedrock\Users\Shared\games\com.mojang\resource_packs`
	targets := []string{
		"assets/minecraft/textures/gui/container/anvil.png",
		"assets/minecraft/textures/gui/container/creative_inventory/tabs.png",
		"assets/minecraft/textures/gui/container/stats_icons.png",
		"textures/items/stick.png",
	}
	for _, rel := range targets {
		uri, err := a.GetPackThumb("infera", rel)
		if err != nil {
			t.Logf("SKIP %s: %v", rel, err)
			continue
		}
		b64 := strings.SplitN(uri, ",", 2)[1]
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			t.Logf("DECODE %s: %v", rel, err)
			continue
		}
		name := strings.ReplaceAll(strings.TrimPrefix(rel, "assets/minecraft/textures/"), "/", "_")
		out := `C:\Users\catto\AppData\Local\Temp\opencode\thumb_` + name
		if strings.Contains(rel, "items/stick") {
			out = `C:\Users\catto\AppData\Local\Temp\opencode\thumb_stick.png`
		}
		if err := os.WriteFile(out, data, 0644); err != nil {
			t.Logf("WRITE %s: %v", out, err)
			continue
		}
		// decode to confirm
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Logf("PNG %s: %v", rel, err)
			continue
		}
		b := img.Bounds()
		t.Logf("WROTE   %-70s -> %-40s (%dx%d)", rel, out, b.Dx(), b.Dy())
	}
}
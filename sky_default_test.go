package main

import (
	"os"
	"path/filepath"
	"testing"
)

// GetSkyPacks lists only packs with a sky, whether their own or in a subpack,
// under the manifest's name, with only the subpacks that have a sky.
func TestGetSkyPacks(t *testing.T) {
	a := testApp(t)
	root := a.getResourcePacksPath()
	write := func(rel, data string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("own/manifest.json", `{"header":{"name":"Own Sky"}}`)
	write("own/textures/environment/overworld_cubemap/cubemap_0.png", "x")
	write("subs/manifest.json", `{"header":{"name":"Sub Skies"},"subpacks":[
		{"folder_name":"night","name":"Night"},{"folder_name":"empty","name":"Empty"}]}`)
	write("subs/subpacks/night/textures/environment/overworld_cubemap/cubemap_3.png", "x")
	write("subs/subpacks/empty/textures/blocks/stone.png", "x")
	write("none/manifest.json", `{"header":{"name":"No Sky"}}`)
	write("none/textures/blocks/stone.png", "x")

	got := a.GetSkyPacks()
	if len(got) != 2 {
		t.Fatalf("got %d sky packs, want 2: %+v", len(got), got)
	}
	if p := got[0]; p.DirName != "own" || p.Name != "Own Sky" || !p.HasSky || len(p.Subpacks) != 0 {
		t.Errorf("own sky pack: %+v", p)
	}
	if p := got[1]; p.DirName != "subs" || p.Name != "Sub Skies" || p.HasSky ||
		len(p.Subpacks) != 1 || p.Subpacks[0].FolderName != "night" {
		t.Errorf("subpack sky pack: %+v", p)
	}
}

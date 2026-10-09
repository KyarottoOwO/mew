package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SkyPack is an installed pack whose sky can be the Pack Viewer's default:
// one with a cubemap of its own, or in at least one of its subpacks.
type SkyPack struct {
	DirName  string       `json:"dirName"`
	Name     string       `json:"name"`
	HasSky   bool         `json:"hasSky"` // a cubemap outside any subpack
	Subpacks []SkySubpack `json:"subpacks"`
}

// GetSkyPacks lists the installed packs that ship a sky, for the Settings
// page's default sky pick. It reads only manifests and the environment
// folders, so it stays quick with many packs installed.
func (a *App) GetSkyPacks() []SkyPack {
	base := a.getResourcePacksPath()
	entries, err := os.ReadDir(base)
	if err != nil {
		return []SkyPack{}
	}
	out := []SkyPack{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(base, e.Name())
		envDir := filepath.Join(dir, "textures", "environment")
		p := SkyPack{
			DirName: e.Name(),
			Name:    e.Name(),
			HasSky:  hasSkyCubemap(filepath.Join(envDir, "overworld_cubemap"), envDir),
		}
		p.Subpacks, _ = a.GetPackSkySubpacks(e.Name())
		if !p.HasSky && len(p.Subpacks) == 0 {
			continue
		}
		if p.Subpacks == nil {
			p.Subpacks = []SkySubpack{}
		}
		if data, err := os.ReadFile(filepath.Join(dir, "manifest.json")); err == nil {
			var m struct {
				Header struct {
					Name string `json:"name"`
				} `json:"header"`
			}
			if json.Unmarshal(data, &m) == nil && strings.TrimSpace(m.Header.Name) != "" {
				p.Name = m.Header.Name
			}
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

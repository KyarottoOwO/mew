package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	bedrockskin "github.com/THEBOSS9345/bedrock-skin-go"
)

// skinThumbSize is the edge length of a rendered skin thumbnail. Cards show it
// at about half that, so it stays crisp on high-DPI screens.
const skinThumbSize = 96

// maxSkinEdge bounds the skin textures we render. Bedrock skins are 64 or 128
// pixels square; anything far bigger is not a skin and not worth decoding.
const maxSkinEdge = 1024

// GetPackSkinThumbnails renders a small 3D body of the player skin each pack
// overrides, keyed by pack directory name. Packs without a skin, or whose skin
// fails to render, are left out of the map.
func (a *App) GetPackSkinThumbnails(packNames []string) map[string]string {
	out := make(map[string]string)
	for _, name := range packNames {
		if name == "" || filepath.Base(name) != name {
			continue
		}
		if uri := a.packSkinThumbnail(name); uri != "" {
			out[name] = uri
		}
	}
	return out
}

func (a *App) packSkinThumbnail(packName string) string {
	p := findPackSkin(a.getPackDir(packName))
	if p == "" {
		return ""
	}
	st, err := os.Stat(p)
	if err != nil {
		return ""
	}
	// Key on size and mtime too, so a skin edited in the Recolor tool re-renders.
	cacheKey := fmt.Sprintf("skin\x00%s\x00%d\x00%d", p, st.Size(), st.ModTime().UnixNano())
	if v, ok := a.thumbCache.Load(cacheKey); ok {
		return v.(string)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	uri, err := renderSkinThumbnail(data)
	if err != nil {
		debugLogf("skin thumbnail for %s: %v", packName, err)
		return ""
	}
	a.thumbCache.Store(cacheKey, uri)
	return uri
}

// renderSkinThumbnail renders an encoded skin texture as an angled full-body
// PNG data URI.
func renderSkinThumbnail(texture []byte) (string, error) {
	w, h, err := bedrockskin.ImageDimensions(texture)
	if err != nil {
		return "", err
	}
	if w > maxSkinEdge || h > maxSkinEdge {
		return "", fmt.Errorf("skin is %dx%d, larger than %d", w, h, maxSkinEdge)
	}
	png, err := bedrockskin.RenderBytes(bedrockskin.BytesOptions{
		Texture: texture,
		View:    bedrockskin.ViewBody,
		Angle:   bedrockskin.AngleIso,
		Size:    skinThumbSize,
	})
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}

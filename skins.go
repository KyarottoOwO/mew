package main

import (
	"os"
)

// skinThumbSize is the edge length of a rendered pack thumbnail. Cards show it
// at about half that, so it stays crisp on high-DPI screens.
const skinThumbSize = 96

// GetPackSkinThumbnails renders the look of each pack: the pack's player skin
// (or the user's default skin, or MEW's) in the pack's diamond armor holding
// the pack's diamond sword, both falling back to vanilla. Every installed pack
// gets one, so a pack that only retextures armor or swords still shows it.
// Keyed by pack directory name. base is the packs' folder, as in
// RenderRequest.
func (a *App) GetPackSkinThumbnails(packNames []string, base string) map[string]string {
	out := make(map[string]string)
	for _, name := range packNames {
		dir, err := a.packDirFor(base, name)
		if err != nil || dir == "" {
			continue
		}
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			continue
		}
		if uri := a.packSkinThumbnail(name, base); uri != "" {
			out[name] = uri
		}
	}
	return out
}

// packSkinThumbnail renders one pack's card image, cached by the request and
// the pack files it read (see RenderSkin), so editing a texture in the Recolor
// tool refreshes the card.
func (a *App) packSkinThumbnail(packName, base string) string {
	uri, err := a.RenderSkin(RenderRequest{
		Pack:     packName,
		Base:     base,
		Material: "diamond",
		Right:    HandRequest{Item: "diamond_sword"},
		Angle:    "iso",
		Size:     skinThumbSize,
	})
	if err != nil {
		debugLogf("skin thumbnail for %s: %v", packName, err)
		return ""
	}
	return uri
}

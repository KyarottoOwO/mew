package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// chosenSkinPath is where the skin the user picked for one pack with "Change
// Skin" is kept, or "" for a name that is not a plain pack directory name.
// Each pack has its own, so changing the skin in one pack leaves the others
// alone, and it survives restarts.
func chosenSkinPath(packName string) string {
	if packName == "" || filepath.Base(packName) != packName || packName == "." || packName == ".." {
		return ""
	}
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "mew", "pack_skins", packName+".png")
}

// SetPackSkin saves a skin for one pack, replacing the pack's own skin in the
// Pack Viewer and on its card. The data URI must decode to a skin-sized image.
func (a *App) SetPackSkin(packName string, dataURI string) error {
	p := chosenSkinPath(packName)
	if p == "" {
		return fmt.Errorf("invalid pack name")
	}
	_, data, err := splitDataURI(dataURI)
	if err != nil {
		return err
	}
	img, err := decodeTexture(data)
	if err != nil {
		return fmt.Errorf("not a skin image: %w", err)
	}
	if b := img.Bounds(); b.Dx() < 64 || b.Dy() < 32 {
		return fmt.Errorf("skin is %dx%d, smaller than 64x32", b.Dx(), b.Dy())
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	// Write then rename, so a render never reads a half-written skin.
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// ClearPackSkin forgets the skin chosen for one pack, so it shows its own
// skin (or the default) again.
func (a *App) ClearPackSkin(packName string) error {
	p := chosenSkinPath(packName)
	if p == "" {
		return fmt.Errorf("invalid pack name")
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// HasPackSkin reports whether the user chose a skin for this pack.
func (a *App) HasPackSkin(packName string) bool {
	p := chosenSkinPath(packName)
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	xdraw "golang.org/x/image/draw"
	"github.com/woozymasta/tga"
)

type RecolorTexture struct {
	Folder   string `json:"folder"`
	Filename string `json:"filename"`
	RelPath  string `json:"relPath"`
	Size     int64  `json:"size"`
}

func (a *App) getPackRoot(basePath, packName string) (string, error) {
	base := basePath
	if base == "" {
		base = a.getResourcePacksPath()
	}
	if packName == "" || strings.Contains(packName, "..") || strings.ContainsAny(packName, `\/`) {
		return "", fmt.Errorf("invalid pack name")
	}
	root := filepath.Join(base, packName)
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return "", fmt.Errorf("pack not found: %s", packName)
	}
	return root, nil
}

func (a *App) GetPackRoot(packName string, basePath string) (string, error) {
	return a.getPackRoot(basePath, packName)
}

func safeRelPath(relPath string) (string, error) {
	relPath = strings.TrimSpace(relPath)
	if relPath == "" {
		return "", fmt.Errorf("empty path")
	}
	clean := filepath.Clean(filepath.FromSlash(relPath))
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || strings.ContainsAny(clean, `\`) && strings.HasPrefix(clean, "\\") {
		return "", fmt.Errorf("invalid path")
	}
	return clean, nil
}

func (a *App) ListPackTextures(packName string, basePath string) ([]RecolorTexture, error) {
	root, err := a.getPackRoot(basePath, packName)
	if err != nil {
		return nil, err
	}

	exts := map[string]bool{".png": true, ".tga": true, ".jpg": true, ".jpeg": true}
	var result []RecolorTexture
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if strings.Contains(relSlash, "entity") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(rel))
		if !exts[ext] {
			return nil
		}
		info, _ := d.Info()
		folder := filepath.ToSlash(filepath.Dir(rel))
		if folder == "." {
			folder = ""
		}
		result = append(result, RecolorTexture{
			Folder:   folder,
			Filename: filepath.Base(rel),
			RelPath:  relSlash,
			Size:     info.Size(),
		})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].RelPath < result[j].RelPath
	})
	return result, nil
}

func (a *App) GetPackTexture(packName string, relPath string, basePath string) (string, error) {
	root, err := a.getPackRoot(basePath, packName)
	if err != nil {
		return "", err
	}
	clean, err := safeRelPath(relPath)
	if err != nil {
		return "", err
	}
	full := filepath.Join(root, clean)
	if !strings.HasPrefix(full, root+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid path")
	}
	return a.readImageAsDataURI(full), nil
}

// GetPackThumb decodes a texture and returns a small PNG data URI (max 96px)
// for fast sidebar thumbnails, avoiding full-size transfers over IPC.
func (a *App) GetPackThumb(packName string, relPath string, basePath string) (string, error) {
	cacheKey := packName + "\x00" + basePath + "\x00" + relPath
	if v, ok := a.thumbCache.Load(cacheKey); ok {
		return v.(string), nil
	}
	root, err := a.getPackRoot(basePath, packName)
	if err != nil {
		return "", err
	}
	clean, err := safeRelPath(relPath)
	if err != nil {
		return "", err
	}
	full := filepath.Join(root, clean)
	if !strings.HasPrefix(full, root+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid path")
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	var img image.Image
	ext := strings.ToLower(filepath.Ext(clean))
	if ext == ".tga" {
		img, err = tga.Decode(bytes.NewReader(data))
		if err != nil {
			return "", fmt.Errorf("decode tga: %v", err)
		}
	} else {
		img, _, err = image.Decode(bytes.NewReader(data))
		if err != nil {
			return "", fmt.Errorf("decode image: %v", err)
		}
	}

	const maxSide = 128.0
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	scale := maxSide / float64(w)
	if float64(h) > float64(w) {
		scale = maxSide / float64(h)
	}
	nw := int(float64(w) * scale)
	nh := int(float64(h) * scale)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	scaled := image.NewRGBA(image.Rect(0, 0, nw, nh))
	xdraw.NearestNeighbor.Scale(scaled, scaled.Bounds(), img, bounds, xdraw.Over, nil)

	// Always output an exact square (maxSide x maxSide) so the frontend can
	// display it at a crisp 1:1 scale. Smaller/non-square textures are
	// upscaled with an integer factor and centered on a transparent canvas.
	dst := image.NewRGBA(image.Rect(0, 0, int(maxSide), int(maxSide)))
	offX := (int(maxSide) - nw) / 2
	offY := (int(maxSide) - nh) / 2
	xdraw.Draw(dst, image.Rect(offX, offY, offX+nw, offY+nh), scaled, image.Point{}, xdraw.Over)

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return "", err
	}
	uri := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	a.thumbCache.LoadOrStore(cacheKey, uri)
	return uri, nil
}

// SavePackTexture writes a PNG (base64 data URI payload) to a texture path
// inside an installed pack. Used for both "Replace" and "Save As".
func (a *App) SavePackTexture(packName string, relPath string, imageData string, basePath string) error {
	root, err := a.getPackRoot(basePath, packName)
	if err != nil {
		return err
	}
	clean, err := safeRelPath(relPath)
	if err != nil {
		return err
	}
	full := filepath.Join(root, clean)
	if !strings.HasPrefix(full, root+string(filepath.Separator)) {
		return fmt.Errorf("invalid path")
	}

	raw := strings.TrimSpace(imageData)
	if i := strings.IndexByte(raw, ','); i >= 0 && strings.HasPrefix(strings.ToLower(raw), "data:") {
		raw = raw[i+1:]
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return fmt.Errorf("invalid image data: %v", err)
	}
	os.MkdirAll(filepath.Dir(full), os.ModePerm)
	if err := os.WriteFile(full, decoded, 0644); err != nil {
		return fmt.Errorf("failed to write texture: %v", err)
	}
	a.thumbCache.Delete(packName + "\x00" + basePath + "\x00" + clean)
	a.logDebug(fmt.Sprintf("SavePackTexture: wrote %s", full))
	return nil
}
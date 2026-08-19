package main

import (
	"archive/zip"
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	xdraw "golang.org/x/image/draw"
)

//go:embed all:animator_assets
var animatorAssets embed.FS

const (
	animFrameWidth  = 352
	animFrameHeight = 332
	maxAnimFrames   = 40
)

const animManifestDescription = "Made with MEW"

func parseHexColor(hex string) color.RGBA {
	hex = strings.TrimSpace(hex)
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	val, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return color.RGBA{R: 0, G: 0, B: 0, A: 255}
	}
	return color.RGBA{R: uint8(val >> 16), G: uint8(val >> 8), B: uint8(val), A: 255}
}

func fitFrameToCanvas(src image.Image, transparent bool, fill color.RGBA) *image.RGBA {
	canvas := image.NewRGBA(image.Rect(0, 0, animFrameWidth, animFrameHeight))
	if !transparent {
		xdraw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: fill}, image.Point{}, xdraw.Src)
	}

	srcW := src.Bounds().Dx()
	srcH := src.Bounds().Dy()
	if srcW <= 0 || srcH <= 0 {
		return canvas
	}

	scale := math.Min(float64(animFrameWidth)/float64(srcW), float64(animFrameHeight)/float64(srcH))
	newW := int(math.Round(float64(srcW) * scale))
	newH := int(math.Round(float64(srcH) * scale))
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	scaled := image.NewRGBA(image.Rect(0, 0, newW, newH))
	xdraw.BiLinear.Scale(scaled, scaled.Bounds(), src, src.Bounds(), xdraw.Over, nil)

	offX := (animFrameWidth - newW) / 2
	offY := (animFrameHeight - newH) / 2
	xdraw.Draw(canvas, image.Rect(offX, offY, offX+newW, offY+newH), scaled, image.Point{}, xdraw.Over)
	return canvas
}

func buildFlipbook(frames []*image.RGBA) *image.RGBA {
	sheet := image.NewRGBA(image.Rect(0, 0, animFrameWidth, animFrameHeight*len(frames)))
	for i, f := range frames {
		xdraw.Draw(sheet, image.Rect(0, i*animFrameHeight, animFrameWidth, (i+1)*animFrameHeight), f, image.Point{}, xdraw.Over)
	}
	return sheet
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func copyEmbeddedAssets(destDir string) error {
	return fs.WalkDir(animatorAssets, "animator_assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(path, "animator_assets/")
		if strings.EqualFold(rel, filepath.Join("ui", "_global_variables.json")) {
			return nil
		}
		data, err := animatorAssets.ReadFile(path)
		if err != nil {
			return err
		}
		dest := filepath.Join(destDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), os.ModePerm); err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0644)
	})
}

func writeGlobalVariables(destDir string, frames int, frameDuration string) error {
	data, err := animatorAssets.ReadFile("animator_assets/ui/_global_variables.json")
	if err != nil {
		return err
	}
	content := string(data)
	framesString := strconv.Itoa(frames)
	if frames < 10 {
		framesString = "0" + framesString
	}
	content = strings.ReplaceAll(content, "num", framesString)
	content = strings.ReplaceAll(content, "fum_frames", frameDuration)
	dest := filepath.Join(destDir, "ui", "_global_variables.json")
	if err := os.MkdirAll(filepath.Dir(dest), os.ModePerm); err != nil {
		return err
	}
	return os.WriteFile(dest, []byte(content), 0644)
}

func zipDirToFile(srcDir, zipPath string) error {
	outFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	w := zip.NewWriter(outFile)
	defer w.Close()

	return filepath.Walk(srcDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		entry, err := w.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = entry.Write(data)
		return err
	})
}

func buildAnimatedPack(gifBytes []byte, fileName string, frameDuration string, overlayBytes []byte, useOverlay, transparentFill bool, fillColor, outDir, manifestDesc string, onFrame func(completed, total int)) (string, error) {
	decoded, err := gif.DecodeAll(bytes.NewReader(gifBytes))
	if err != nil {
		return "", fmt.Errorf("failed to read gif: %v", err)
	}
	if len(decoded.Image) == 0 {
		return "", fmt.Errorf("no frames found in the gif")
	}
	if len(decoded.Image) > maxAnimFrames {
		decoded.Image = decoded.Image[:maxAnimFrames]
	}
	frames := len(decoded.Image)

	fill := parseHexColor(fillColor)
	packName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	tempDir := getMewTempDir("anim_port_temp")
	os.RemoveAll(tempDir)
	os.MkdirAll(tempDir, os.ModePerm)
	defer os.RemoveAll(tempDir)

	bgDir := filepath.Join(tempDir, "textures", "animated_ui", "inventory_bg")
	os.MkdirAll(bgDir, os.ModePerm)

	var canvases []*image.RGBA
	for i, frame := range decoded.Image {
		canvases = append(canvases, fitFrameToCanvas(frame, transparentFill, fill))
		if onFrame != nil {
			onFrame(i+1, frames)
		}
	}

	if err := writePNG(filepath.Join(bgDir, "inventory_vertical_flipbook.png"), buildFlipbook(canvases)); err != nil {
		return "", fmt.Errorf("failed to write flipbook: %v", err)
	}

	overlayDest := filepath.Join(bgDir, "inventory_overlay.png")
	if useOverlay && len(overlayBytes) > 0 {
		overlayImg, err := png.Decode(bytes.NewReader(overlayBytes))
		if err != nil {
			return "", fmt.Errorf("failed to read overlay png: %v", err)
		}
		resized := image.NewRGBA(image.Rect(0, 0, animFrameWidth, animFrameHeight))
		xdraw.BiLinear.Scale(resized, resized.Bounds(), overlayImg, overlayImg.Bounds(), xdraw.Over, nil)
		if err := writePNG(overlayDest, resized); err != nil {
			return "", fmt.Errorf("failed to write overlay: %v", err)
		}
	} else {
		asset, err := animatorAssets.ReadFile("animator_assets/textures/animated_ui/inventory_bg/inventory_overlay.png")
		if err != nil {
			return "", fmt.Errorf("failed to load default overlay: %v", err)
		}
		if err := os.WriteFile(overlayDest, asset, 0644); err != nil {
			return "", fmt.Errorf("failed to write default overlay: %v", err)
		}
	}

	if err := writePNG(filepath.Join(tempDir, "pack_icon.png"), canvases[len(canvases)-1]); err != nil {
		return "", fmt.Errorf("failed to write pack icon: %v", err)
	}

	if err := copyEmbeddedAssets(tempDir); err != nil {
		return "", fmt.Errorf("failed to copy pack assets: %v", err)
	}

	if err := writeGlobalVariables(tempDir, frames, frameDuration); err != nil {
		return "", fmt.Errorf("failed to write global variables: %v", err)
	}

	manifest := map[string]interface{}{
		"format_version": 1,
		"header": map[string]interface{}{
			"description":        manifestDesc,
			"name":               packName,
			"uuid":               uuid.New().String(),
			"version":            []int{1, 0, 0},
			"min_engine_version": []int{1, 12, 1},
		},
		"modules": []interface{}{
			map[string]interface{}{
				"description": manifestDesc,
				"type":        "resources",
				"uuid":        uuid.New().String(),
				"version":     []int{1, 0, 0},
			},
		},
	}
	manifestData, err := json.MarshalIndent(manifest, "", "    ")
	if err != nil {
		return "", fmt.Errorf("failed to build manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "manifest.json"), manifestData, 0644); err != nil {
		return "", fmt.Errorf("failed to write manifest: %v", err)
	}

	mcpackPath := filepath.Join(outDir, packName+".mcpack")
	if err := zipDirToFile(tempDir, mcpackPath); err != nil {
		return "", fmt.Errorf("failed to create mcpack: %v", err)
	}
	return mcpackPath, nil
}

func (a *App) CreateAnimatedInventory(gifBytes []byte, fileName string, frameDuration string, overlayBytes []byte, useOverlay, transparentFill bool, mergePacks []string, fillColor string) (string, error) {
	a.logDebug(fmt.Sprintf("CreateAnimatedInventory: called, file=%s, size=%d, duration=%s, useOverlay=%v, transparent=%v, mergePacks=%v", fileName, len(gifBytes), frameDuration, useOverlay, transparentFill, mergePacks))

	if err := a.acquirePort(); err != nil {
		return "", err
	}
	defer a.releasePort()

	if len(gifBytes) == 0 {
		return "", fmt.Errorf("no gif data provided")
	}
	if !strings.HasSuffix(strings.ToLower(fileName), ".gif") {
		return "", fmt.Errorf("please provide a .gif file")
	}
	duration, err := strconv.ParseFloat(frameDuration, 64)
	if err != nil || duration < 0.05 || duration > 0.09 {
		return "", fmt.Errorf("frame duration must be between 0.05 and 0.09 seconds")
	}

	desc := animManifestDescription
	if custom := a.getStringSetting("manifestDescription"); custom != "" {
		desc = custom
	}

	outDir := a.getOutputDir()
	if a.getBoolSetting("deleteMcpack") {
		outDir = a.getTempDir("mcpack")
		os.MkdirAll(outDir, os.ModePerm)
	}
	a.emitProgress("Animating", "Processing GIF frames...", "info", fileName, 0, 0)

	mcpackPath, err := buildAnimatedPack(gifBytes, fileName, frameDuration, overlayBytes, useOverlay, transparentFill, fillColor, outDir, desc, func(completed, total int) {
		a.emitProgress("Animating", fmt.Sprintf("Frame %d/%d", completed, total), "info", fileName, total, completed)
	})
	if err != nil {
		return "", err
	}

	if len(mergePacks) > 0 {
		if err := a.mergeAnimatedPack(mcpackPath, mergePacks); err != nil {
			return "", err
		}
	} else if a.getBoolSetting("autoImport") {
		a.importToBedrock(mcpackPath)
	}
	if a.getBoolSetting("autoOpenFolder") {
		a.OpenFolder(outDir)
	}

	a.emitProgress("Done", mcpackPath, "success", fileName, 0, 0)
	a.AddRecentPack(fileName, mcpackPath)
	a.logDebug(fmt.Sprintf("CreateAnimatedInventory completed: %s", mcpackPath))
	return mcpackPath, nil
}

func (a *App) mergeAnimatedPack(mcpackPath string, packNames []string) error {
	bedrockPath := a.getStringSetting("resourcePacksPath")
	if bedrockPath == "" {
		bedrockPath = a.getDefaultResourcePacksPath()
	}
	a.logDebug(fmt.Sprintf("mergeAnimatedPack: bedrockPath=%s, packs=%v", bedrockPath, packNames))
	if st, err := os.Stat(bedrockPath); err != nil || !st.IsDir() {
		return fmt.Errorf("resource packs path not found: %s", bedrockPath)
	}

	extractDir := filepath.Join(getMewTempDir("anim_merge_temp"))
	os.RemoveAll(extractDir)
	os.MkdirAll(extractDir, os.ModePerm)
	defer os.RemoveAll(extractDir)

	if err := unzip(mcpackPath, extractDir); err != nil {
		return fmt.Errorf("failed to extract pack for merge: %w", err)
	}

	merged := 0
	for _, name := range packNames {
		if name == "" || strings.Contains(name, "..") || strings.ContainsAny(name, `\/`) {
			continue
		}
		destDir := filepath.Join(bedrockPath, name)
		if st, err := os.Stat(destDir); err != nil || !st.IsDir() {
			continue
		}

	stripTargetUIDX(destDir)
		a.logDebug(fmt.Sprintf("mergeAnimatedPack: stripped uidx from %s", destDir))

		filepath.Walk(extractDir, func(path string, info fs.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(extractDir, path)
			if err != nil {
				return nil
			}
			base := strings.ToLower(info.Name())
			if base == "manifest.json" || base == "pack_icon.png" {
				return nil
			}
			dst := filepath.Join(destDir, rel)
			os.MkdirAll(filepath.Dir(dst), os.ModePerm)
			copyFile(path, dst)
			return nil
		})
		merged++
		a.logDebug(fmt.Sprintf("Merged animated inventory into %s", destDir))
	}

	if merged == 0 {
		return fmt.Errorf("none of the selected packs were found in Minecraft")
	}
	return nil
}

func stripTargetUIDX(destDir string) {
	filepath.Walk(destDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		lower := strings.ToLower(info.Name())
		rel, relErr := filepath.Rel(destDir, path)
		if relErr != nil {
			return nil
		}
		relLower := strings.ToLower(filepath.ToSlash(rel))

		if strings.HasSuffix(lower, ".uidx") {
			if removeErr := os.Remove(path); removeErr != nil {
				debugLog(fmt.Sprintf("stripTargetUIDX: failed to remove %s: %v", path, removeErr))
			}
			return nil
		}
		if strings.HasPrefix(relLower, "ui/") && strings.HasSuffix(lower, "_screen.json") {
			if removeErr := os.Remove(path); removeErr != nil {
				debugLog(fmt.Sprintf("stripTargetUIDX: failed to remove %s: %v", path, removeErr))
			}
			return nil
		}
		if relLower == "ui/_global_variables.json" {
			if removeErr := os.Remove(path); removeErr != nil {
				debugLog(fmt.Sprintf("stripTargetUIDX: failed to remove %s: %v", path, removeErr))
			}
			return nil
		}
		if relLower == "ui/_ui_defs.json" {
			if writeErr := os.WriteFile(path, []byte("{\n  \"ui_defs\": []\n}\n"), 0644); writeErr != nil {
				debugLog(fmt.Sprintf("stripTargetUIDX: failed to write %s: %v", path, writeErr))
			}
			return nil
		}
		if strings.Contains(relLower, "textures/uidx/") {
			if removeErr := os.Remove(path); removeErr != nil {
				debugLog(fmt.Sprintf("stripTargetUIDX: failed to remove %s: %v", path, removeErr))
			}
			return nil
		}
		return nil
	})

	texturesUIDX := filepath.Join(destDir, "textures", "uidx")
	os.RemoveAll(texturesUIDX)
}

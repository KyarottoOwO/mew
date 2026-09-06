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
	_ "image/jpeg"
	"image/png"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	stripjsoncomments "github.com/trapcodeio/go-strip-json-comments"
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

	bounds := scaled.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			p := scaled.RGBAAt(x, y)
			if p.A == 0 {
				continue
			}
			if p.A < 255 {
				p.R = uint8(int(p.R) * 255 / int(p.A))
				p.G = uint8(int(p.G) * 255 / int(p.A))
				p.B = uint8(int(p.B) * 255 / int(p.A))
				p.A = 255
				scaled.SetRGBA(x, y, p)
			}
		}
	}

	offX := (animFrameWidth - newW) / 2
	offY := (animFrameHeight - newH) / 2
	xdraw.Draw(canvas, image.Rect(offX, offY, offX+newW, offY+newH), scaled, image.Point{}, xdraw.Over)
	return canvas
}

func copyRGBA(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	xdraw.Draw(dst, dst.Bounds(), src, image.Point{}, xdraw.Src)
	return dst
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
	content = strings.ReplaceAll(content, "$num", framesString)
	content = strings.ReplaceAll(content, "$fum_frames", frameDuration)
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
	canvas := image.NewRGBA(image.Rect(0, 0, decoded.Config.Width, decoded.Config.Height))
	var prevCanvas *image.RGBA

	for i, frame := range decoded.Image {
		if i == 0 {
			prevCanvas = image.NewRGBA(canvas.Bounds())
		} else {
			prevCanvas = copyRGBA(canvas)
		}

		bounds := frame.Bounds()
		xdraw.Draw(canvas, bounds, frame, bounds.Min, xdraw.Over)

		composited := copyRGBA(canvas)
		composited = fitFrameToCanvas(composited, transparentFill, fill)
		canvases = append(canvases, composited)

		switch {
		case i < len(decoded.Disposal) && decoded.Disposal[i] == gif.DisposalBackground:
			bg := color.RGBA{fill.R, fill.G, fill.B, 255}
			if transparentFill {
				bg = color.RGBA{0, 0, 0, 0}
			}
			xdraw.Draw(canvas, bounds, &image.Uniform{C: bg}, bounds.Min, xdraw.Src)
		case i < len(decoded.Disposal) && decoded.Disposal[i] == gif.DisposalPrevious:
			if prevCanvas != nil {
				xdraw.Draw(canvas, canvas.Bounds(), prevCanvas, image.Point{}, xdraw.Src)
			}
		}

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

func buildStaticImagePack(imageBytes []byte, fileName string, overlayBytes []byte, useOverlay, transparentFill bool, fillColor, outDir, manifestDesc string) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %v", err)
	}

	fill := parseHexColor(fillColor)
	packName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	tempDir := getMewTempDir("anim_port_temp")
	os.RemoveAll(tempDir)
	os.MkdirAll(tempDir, os.ModePerm)
	defer os.RemoveAll(tempDir)

	bgDir := filepath.Join(tempDir, "textures", "animated_ui", "inventory_bg")
	os.MkdirAll(bgDir, os.ModePerm)

	canvas := fitFrameToCanvas(img, transparentFill, fill)

	if err := writePNG(filepath.Join(bgDir, "inventory_vertical_flipbook.png"), buildFlipbook([]*image.RGBA{canvas})); err != nil {
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

	if err := writePNG(filepath.Join(tempDir, "pack_icon.png"), canvas); err != nil {
		return "", fmt.Errorf("failed to write pack icon: %v", err)
	}

	if err := copyEmbeddedAssets(tempDir); err != nil {
		return "", fmt.Errorf("failed to copy pack assets: %v", err)
	}

	if err := writeGlobalVariables(tempDir, 1, "0.06"); err != nil {
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
		return "", fmt.Errorf("no file data provided")
	}
	lower := strings.ToLower(fileName)
	isGif := strings.HasSuffix(lower, ".gif")
	isImage := strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg")
	if !isGif && !isImage {
		return "", fmt.Errorf("please provide a .gif, .png, .jpg, or .jpeg file")
	}

	desc := animManifestDescription
	if custom := a.getStringSetting("manifestDescription"); custom != "" {
		desc = custom
	}

	outDir := a.getOutputDir()
	if a.getBoolSetting("deleteMcpack") && a.getBoolSetting("autoImport") {
		outDir = a.getTempDir("mcpack")
		os.MkdirAll(outDir, os.ModePerm)
	}
	a.emitProgress("Animating", "Processing image...", "info", fileName, 0, 0)

	var mcpackPath string
	var err error
	if isGif {
		duration, parseErr := strconv.ParseFloat(frameDuration, 64)
		if parseErr != nil || duration < 0.05 || duration > 0.09 {
			return "", fmt.Errorf("frame duration must be between 0.05 and 0.09 seconds")
		}
		mcpackPath, err = buildAnimatedPack(gifBytes, fileName, frameDuration, overlayBytes, useOverlay, transparentFill, fillColor, outDir, desc, func(completed, total int) {
			a.emitProgress("Animating", fmt.Sprintf("Frame %d/%d", completed, total), "info", fileName, total, completed)
		})
	} else {
		mcpackPath, err = buildStaticImagePack(gifBytes, fileName, overlayBytes, useOverlay, transparentFill, fillColor, outDir, desc)
	}
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

	extractCount := 0
	filepath.Walk(extractDir, func(path string, info fs.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			rel, _ := filepath.Rel(extractDir, path)
			a.logDebug(fmt.Sprintf("mergeAnimatedPack: extract file: %s (%d bytes)", rel, info.Size()))
			extractCount++
		}
		return nil
	})
	a.logDebug(fmt.Sprintf("mergeAnimatedPack: extracted %d files from mcpack", extractCount))

	merged := 0
	for _, name := range packNames {
		if name == "" || strings.Contains(name, "..") || strings.ContainsAny(name, `\/`) {
			a.logDebug(fmt.Sprintf("mergeAnimatedPack: skipping invalid name: %q", name))
			continue
		}
		destDir := filepath.Join(bedrockPath, name)
		if st, err := os.Stat(destDir); err != nil || !st.IsDir() {
			a.logDebug(fmt.Sprintf("mergeAnimatedPack: pack dir NOT found, skipping: %s (err=%v)", destDir, err))
			continue
		}

	stripInventoryFiles(destDir)
		a.logDebug(fmt.Sprintf("mergeAnimatedPack: stripped inventory files from %s", destDir))

		originalUIDefs := readUIDefs(filepath.Join(destDir, "ui", "_ui_defs.json"))
		originalGlobals := readGlobalVars(filepath.Join(destDir, "ui", "_global_variables.json"))

		copied := 0
		filepath.Walk(extractDir, func(path string, info fs.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(extractDir, path)
			if err != nil {
				a.logDebug(fmt.Sprintf("mergeAnimatedPack: Rel error: %v", err))
				return nil
			}
			base := strings.ToLower(info.Name())
			if base == "manifest.json" || base == "pack_icon.png" {
				return nil
			}
			dst := filepath.Join(destDir, rel)
			if mkErr := os.MkdirAll(filepath.Dir(dst), os.ModePerm); mkErr != nil {
				a.logDebug(fmt.Sprintf("mergeAnimatedPack: MkdirAll failed for %s: %v", filepath.Dir(dst), mkErr))
				return nil
			}
			if cpErr := copyFile(path, dst); cpErr != nil {
				a.logDebug(fmt.Sprintf("mergeAnimatedPack: copyFile FAILED %s -> %s: %v", rel, dst, cpErr))
			}
			copied++
			return nil
		})
		if mergeErr := mergeUIDefs(filepath.Join(destDir, "ui", "_ui_defs.json"), originalUIDefs); mergeErr != nil {
			a.logDebug(fmt.Sprintf("mergeAnimatedPack: _ui_defs merge warning: %v", mergeErr))
		}
		if mergeErr := mergeGlobalVars(filepath.Join(destDir, "ui", "_global_variables.json"), originalGlobals); mergeErr != nil {
			a.logDebug(fmt.Sprintf("mergeAnimatedPack: _global_variables merge warning: %v", mergeErr))
		}
		merged++
		a.logDebug(fmt.Sprintf("mergeAnimatedPack: copied %d files into %s", copied, destDir))
	}

	if merged == 0 {
		return fmt.Errorf("none of the selected packs were found in Minecraft")
	}
	return nil
}

func stripInventoryFiles(destDir string) {
	stripped := 0
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

		if strings.Contains(lower, "inventory") && strings.HasSuffix(lower, ".uidx") {
			debugLog(fmt.Sprintf("stripInventoryFiles: removing uidx: %s", rel))
			if removeErr := os.Remove(path); removeErr != nil {
				debugLog(fmt.Sprintf("stripInventoryFiles: FAILED to remove %s: %v", rel, removeErr))
			} else {
				stripped++
			}
			return nil
		}
		if strings.HasPrefix(relLower, "ui/") && strings.Contains(lower, "inventory") && strings.HasSuffix(lower, "_screen.json") {
			debugLog(fmt.Sprintf("stripInventoryFiles: removing screen json: %s", rel))
			if removeErr := os.Remove(path); removeErr != nil {
				debugLog(fmt.Sprintf("stripInventoryFiles: FAILED to remove %s: %v", rel, removeErr))
			} else {
				stripped++
			}
			return nil
		}
		if strings.HasPrefix(relLower, "textures/uidx/") && strings.Contains(lower, "inventory") {
			debugLog(fmt.Sprintf("stripInventoryFiles: removing textures/uidx: %s", rel))
			if removeErr := os.Remove(path); removeErr != nil {
				debugLog(fmt.Sprintf("stripInventoryFiles: FAILED to remove %s: %v", rel, removeErr))
			} else {
				stripped++
			}
			return nil
		}
		return nil
	})
	debugLog(fmt.Sprintf("stripInventoryFiles: done — stripped=%d", stripped))
}

func readUIDefs(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	stripped := stripjsoncomments.Strip(string(data))
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(stripped), &parsed); err != nil {
		return nil
	}
	entries, ok := parsed["ui_defs"].([]interface{})
	if !ok {
		return nil
	}
	var result []string
	for _, e := range entries {
		if s, ok := e.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func readGlobalVars(path string) map[string]interface{} {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	stripped := stripjsoncomments.Strip(string(data))
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(stripped), &parsed); err != nil {
		return nil
	}
	return parsed
}

func mergeUIDefs(defsPath string, originalEntries []string) error {
	if len(originalEntries) == 0 {
		return nil
	}
	data, err := os.ReadFile(defsPath)
	if err != nil {
		return fmt.Errorf("failed to read _ui_defs.json for merge: %w", err)
	}
	stripped := stripjsoncomments.Strip(string(data))
	var current map[string]interface{}
	if err := json.Unmarshal([]byte(stripped), &current); err != nil {
		return fmt.Errorf("failed to parse _ui_defs.json for merge: %w", err)
	}
	currentEntries, _ := current["ui_defs"].([]interface{})

	for _, orig := range originalEntries {
		found := false
		for _, existing := range currentEntries {
			if s, ok := existing.(string); ok && s == orig {
				found = true
				break
			}
		}
		if !found {
			currentEntries = append(currentEntries, orig)
		}
	}

	current["ui_defs"] = currentEntries
	merged, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal merged _ui_defs.json: %w", err)
	}
	return os.WriteFile(defsPath, append(merged, '\n'), 0644)
}

func mergeGlobalVars(varsPath string, originalVars map[string]interface{}) error {
	if len(originalVars) == 0 {
		return nil
	}
	data, err := os.ReadFile(varsPath)
	if err != nil {
		return fmt.Errorf("failed to read _global_variables.json for merge: %w", err)
	}
	stripped := stripjsoncomments.Strip(string(data))
	var current map[string]interface{}
	if err := json.Unmarshal([]byte(stripped), &current); err != nil {
		return fmt.Errorf("failed to parse _global_variables.json for merge: %w", err)
	}

	inventoryKeys := map[string]bool{
		"$total_inventory_frames":       true,
		"$inventory_duration_per_frame": true,
		"$inventory_base_resolution":    true,
	}

	for key, val := range originalVars {
		if inventoryKeys[key] {
			continue
		}
		current[key] = val
	}

	merged, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal merged _global_variables.json: %w", err)
	}
	return os.WriteFile(varsPath, append(merged, '\n'), 0644)
}

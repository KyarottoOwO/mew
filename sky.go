package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	xdraw "golang.org/x/image/draw"
)

const skyManifestDescription = "Sky made with MEW"

type skyFace struct {
	name  string
	// direction function: given (s, t) in [-1,1], returns (x, y, z) direction
	dirFn func(s, t float64) (float64, float64, float64)
}

var skyFaces = []skyFace{
	{"cubemap_0", func(s, t float64) (float64, float64, float64) { return -1, -t, s }},    // -X left
	{"cubemap_1", func(s, t float64) (float64, float64, float64) { return s, -t, 1 }},     // +Z front
	{"cubemap_2", func(s, t float64) (float64, float64, float64) { return 1, -t, -s }},    // +X right
	{"cubemap_3", func(s, t float64) (float64, float64, float64) { return -s, -t, -1 }},   // -Z behind
	{"cubemap_4", func(s, t float64) (float64, float64, float64) { return s, 1, t }},      // +Y top
	{"cubemap_5", func(s, t float64) (float64, float64, float64) { return s, -1, -t }},    // -Y bottom
}

func equirectToCubemap(src image.Image, faceSize int) [6]*image.RGBA {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	var faces [6]*image.RGBA
	for fi, face := range skyFaces {
		faceImg := image.NewRGBA(image.Rect(0, 0, faceSize, faceSize))
		for y := 0; y < faceSize; y++ {
			for x := 0; x < faceSize; x++ {
				s := (2*float64(x)+1)/float64(faceSize) - 1
				t := (2*float64(y)+1)/float64(faceSize) - 1

				dx, dy, dz := face.dirFn(s, t)
				norm := math.Sqrt(dx*dx + dy*dy + dz*dz)
				dx /= norm
				dy /= norm
				dz /= norm

				lon := math.Atan2(dx, dz)
				lat := math.Asin(clamp(dy, -1, 1))

				u := (lon/math.Pi + 1) / 2 * float64(srcW)
				v := (0.5 - lat/math.Pi) * float64(srcH)

				r, g, b, a := sampleBilinear(src, bounds, u, v)
				faceImg.SetRGBA(x, y, color.RGBA{r, g, b, a})
			}
		}
		faces[fi] = faceImg
	}
	return faces
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func sampleBilinear(src image.Image, bounds image.Rectangle, u, v float64) (uint8, uint8, uint8, uint8) {
	srcW := float64(bounds.Dx())
	srcH := float64(bounds.Dy())

	u = math.Mod(u, srcW)
	if u < 0 {
		u += srcW
	}
	v = math.Mod(v, srcH)
	if v < 0 {
		v += srcH
	}

	x0 := int(u) % int(srcW)
	y0 := int(v) % int(srcH)
	x1 := (x0 + 1) % int(srcW)
	y1 := (y0 + 1) % int(srcH)

	fu := u - float64(int(u))
	fv := v - float64(int(v))

	c00 := src.At(x0+bounds.Min.X, y0+bounds.Min.Y)
	c10 := src.At(x1+bounds.Min.X, y0+bounds.Min.Y)
	c01 := src.At(x0+bounds.Min.X, y1+bounds.Min.Y)
	c11 := src.At(x1+bounds.Min.X, y1+bounds.Min.Y)

	r00, g00, b00, a00 := c00.RGBA()
	r10, g10, b10, a10 := c10.RGBA()
	r01, g01, b01, a01 := c01.RGBA()
	r11, g11, b11, a11 := c11.RGBA()

	lf := func(a, b, t float64) float64 { return a*(1-t) + b*t }

	r := uint8(lf(lf(float64(r00>>8), float64(r10>>8), fu), lf(float64(r01>>8), float64(r11>>8), fu), fv))
	g := uint8(lf(lf(float64(g00>>8), float64(g10>>8), fu), lf(float64(g01>>8), float64(g11>>8), fu), fv))
	b := uint8(lf(lf(float64(b00>>8), float64(b10>>8), fu), lf(float64(b01>>8), float64(b11>>8), fu), fv))
	a := uint8(lf(lf(float64(a00>>8), float64(a10>>8), fu), lf(float64(a01>>8), float64(a11>>8), fu), fv))

	return r, g, b, a
}

func buildSkyPack(imageBytes []byte, fileName string, faceSize int, outDir, manifestDesc string) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %v", err)
	}

	packName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	tempDir := getMewTempDir("sky_port_temp")
	os.RemoveAll(tempDir)
	os.MkdirAll(tempDir, os.ModePerm)
	defer os.RemoveAll(tempDir)

	cubemapDir := filepath.Join(tempDir, "textures", "environment", "overworld_cubemap")
	if err := os.MkdirAll(cubemapDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create cubemap dir: %v", err)
	}

	faces := equirectToCubemap(img, faceSize)
	for i, faceImg := range faces {
		name := skyFaces[i].name + ".png"
		var buf bytes.Buffer
		if err := png.Encode(&buf, faceImg); err != nil {
			return "", fmt.Errorf("failed to encode face %d: %v", i, err)
		}
		if err := os.WriteFile(filepath.Join(cubemapDir, name), buf.Bytes(), 0644); err != nil {
			return "", fmt.Errorf("failed to write face %s: %v", name, err)
		}
	}

	bounds := img.Bounds()
	iconW := 256
	iconH := 256
	if bounds.Dx() > 0 && bounds.Dy() > 0 {
		aspect := float64(bounds.Dx()) / float64(bounds.Dy())
		if aspect > 1 {
			iconH = int(float64(iconW) / aspect)
		} else {
			iconW = int(float64(iconH) * aspect)
		}
	}
	iconImg := image.NewRGBA(image.Rect(0, 0, iconW, iconH))
	xdraw.BiLinear.Scale(iconImg, iconImg.Bounds(), img, img.Bounds(), xdraw.Over, nil)
	var iconBuf bytes.Buffer
	if err := png.Encode(&iconBuf, iconImg); err != nil {
		return "", fmt.Errorf("failed to encode icon: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "pack_icon.png"), iconBuf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("failed to write pack icon: %v", err)
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

func (a *App) CreateSkyPack(imageBytes []byte, fileName string, faceSize int, mergePacks []string) (string, error) {
	a.logDebug(fmt.Sprintf("CreateSkyPack: called, file=%s, size=%d, faceSize=%d, mergePacks=%v", fileName, len(imageBytes), faceSize, mergePacks))

	if err := a.acquirePort(); err != nil {
		return "", err
	}
	defer a.releasePort()

	if len(imageBytes) == 0 {
		return "", fmt.Errorf("no image data provided")
	}
	lower := strings.ToLower(fileName)
	if !strings.HasSuffix(lower, ".png") && !strings.HasSuffix(lower, ".jpg") && !strings.HasSuffix(lower, ".jpeg") {
		return "", fmt.Errorf("please provide a .png, .jpg, or .jpeg file")
	}

	if faceSize < 128 {
		faceSize = 128
	}
	if faceSize > 4096 {
		faceSize = 4096
	}

	desc := skyManifestDescription
	if custom := a.getStringSetting("manifestDescription"); custom != "" {
		desc = custom
	}

	outDir := a.getOutputDir()
	if a.getBoolSetting("deleteMcpack") {
		outDir = a.getTempDir("mcpack")
		os.MkdirAll(outDir, os.ModePerm)
	}
	a.emitProgress("Sky Converter", "Generating cubemap faces...", "info", fileName, 0, 0)

	mcpackPath, err := buildSkyPack(imageBytes, fileName, faceSize, outDir, desc)
	if err != nil {
		return "", err
	}

	if len(mergePacks) > 0 {
		if err := a.mergeSkyPack(mcpackPath, mergePacks); err != nil {
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
	a.logDebug(fmt.Sprintf("CreateSkyPack completed: %s", mcpackPath))
	return mcpackPath, nil
}

func buildSkyPackFromFaces(faceData [6][]byte, rotations [6]int, flips [6]bool, outDir, manifestDesc, packName string) (string, error) {
	tempDir := getMewTempDir("sky_port_temp")
	os.RemoveAll(tempDir)
	os.MkdirAll(tempDir, os.ModePerm)
	defer os.RemoveAll(tempDir)

	cubemapDir := filepath.Join(tempDir, "textures", "environment", "overworld_cubemap")
	if err := os.MkdirAll(cubemapDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create cubemap dir: %v", err)
	}

	for i := 0; i < 6; i++ {
		if len(faceData[i]) == 0 {
			continue
		}
		img, _, err := image.Decode(bytes.NewReader(faceData[i]))
		if err != nil {
			return "", fmt.Errorf("failed to decode face %d: %v", i, err)
		}
		processed := processFace(img, rotations[i], flips[i])
		var buf bytes.Buffer
		if err := png.Encode(&buf, processed); err != nil {
			return "", fmt.Errorf("failed to encode face %d: %v", i, err)
		}
		if err := os.WriteFile(filepath.Join(cubemapDir, skyFaces[i].name+".png"), buf.Bytes(), 0644); err != nil {
			return "", fmt.Errorf("failed to write face %s: %v", skyFaces[i].name, err)
		}
	}

	iconImg := processFace(func() image.Image {
		for i := 0; i < 6; i++ {
			if len(faceData[i]) > 0 {
				img, _, _ := image.Decode(bytes.NewReader(faceData[i]))
				return img
			}
		}
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}(), 0, false)
	var iconBuf bytes.Buffer
	if err := png.Encode(&iconBuf, iconImg); err != nil {
		return "", fmt.Errorf("failed to encode icon: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "pack_icon.png"), iconBuf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("failed to write pack icon: %v", err)
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

func processFace(src image.Image, rotation int, flipH bool) *image.RGBA {
	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	var rotated image.Image = src
	if rotation == 90 || rotation == 270 {
		rotated = rotateImage(src, rotation)
	} else if rotation == 180 {
		rotated = rotateImage(src, 180)
	}

	if flipH {
		rotated = flipImageH(rotated)
	}

	rb := rotated.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, rb.Dx(), rb.Dy()))
	xdraw.BiLinear.Scale(out, out.Bounds(), rotated, rb, xdraw.Over, nil)
	_ = w
	_ = h
	return out
}

func rotateImage(src image.Image, degrees int) *image.RGBA {
	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	var outW, outH int
	if degrees == 90 || degrees == 270 {
		outW = h
		outH = w
	} else {
		outW = w
		outH = h
	}

	out := image.NewRGBA(image.Rect(0, 0, outW, outH))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := src.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			var nx, ny int
			switch degrees {
			case 90:
				nx = h - 1 - y
				ny = x
			case 180:
				nx = w - 1 - x
				ny = h - 1 - y
			case 270:
				nx = y
				ny = w - 1 - x
			default:
				nx = x
				ny = y
			}
			out.SetRGBA(nx, ny, color.RGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: uint8(a >> 8),
			})
		}
	}
	return out
}

func flipImageH(src image.Image) *image.RGBA {
	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := src.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			out.SetRGBA(w-1-x, y, color.RGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: uint8(a >> 8),
			})
		}
	}
	return out
}

func (a *App) CreateSkyPackFromFaces(faceData [6][]byte, rotations [6]int, flips [6]bool, mergePacks []string) (string, error) {
	a.logDebug(fmt.Sprintf("CreateSkyPackFromFaces: called, mergePacks=%v", mergePacks))

	if err := a.acquirePort(); err != nil {
		return "", err
	}
	defer a.releasePort()

	hasAny := false
	for i := 0; i < 6; i++ {
		if len(faceData[i]) > 0 {
			hasAny = true
			break
		}
	}
	if !hasAny {
		return "", fmt.Errorf("at least one face image is required")
	}

	desc := skyManifestDescription
	if custom := a.getStringSetting("manifestDescription"); custom != "" {
		desc = custom
	}

	outDir := a.getOutputDir()
	if a.getBoolSetting("deleteMcpack") {
		outDir = a.getTempDir("mcpack")
		os.MkdirAll(outDir, os.ModePerm)
	}
	a.emitProgress("Sky Converter", "Building sky pack...", "info", "sky", 0, 0)

	packName := "Sky Pack"
	mcpackPath, err := buildSkyPackFromFaces(faceData, rotations, flips, outDir, desc, packName)
	if err != nil {
		return "", err
	}

	if len(mergePacks) > 0 {
		if err := a.mergeSkyPack(mcpackPath, mergePacks); err != nil {
			return "", err
		}
	} else if a.getBoolSetting("autoImport") {
		a.importToBedrock(mcpackPath)
	}
	if a.getBoolSetting("autoOpenFolder") {
		a.OpenFolder(outDir)
	}

	a.emitProgress("Done", mcpackPath, "success", "sky", 0, 0)
	a.AddRecentPack(packName, mcpackPath)
	a.logDebug(fmt.Sprintf("CreateSkyPackFromFaces completed: %s", mcpackPath))
	return mcpackPath, nil
}

func (a *App) mergeSkyPack(mcpackPath string, packNames []string) error {
	bedrockPath := a.getStringSetting("resourcePacksPath")
	if bedrockPath == "" {
		bedrockPath = a.getDefaultResourcePacksPath()
	}
	a.logDebug(fmt.Sprintf("mergeSkyPack: bedrockPath=%s, packs=%v", bedrockPath, packNames))
	if st, err := os.Stat(bedrockPath); err != nil || !st.IsDir() {
		return fmt.Errorf("resource packs path not found: %s", bedrockPath)
	}

	extractDir := filepath.Join(getMewTempDir("sky_merge_temp"))
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

		cubemapSrc := filepath.Join(extractDir, "textures", "environment", "overworld_cubemap")
		cubemapDst := filepath.Join(destDir, "textures", "environment", "overworld_cubemap")
		if err := os.MkdirAll(cubemapDst, os.ModePerm); err != nil {
			a.logDebug(fmt.Sprintf("mergeSkyPack: MkdirAll failed: %v", err))
			continue
		}

		copied := 0
		filepath.Walk(cubemapSrc, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(cubemapSrc, path)
			dst := filepath.Join(cubemapDst, rel)
			if cpErr := copyFile(path, dst); cpErr != nil {
				a.logDebug(fmt.Sprintf("mergeSkyPack: copyFile FAILED %s: %v", rel, cpErr))
			}
			copied++
			return nil
		})
		merged++
		a.logDebug(fmt.Sprintf("mergeSkyPack: copied %d cubemap faces into %s", copied, destDir))
	}

	if merged == 0 {
		return fmt.Errorf("none of the selected packs were found in Minecraft")
	}
	return nil
}

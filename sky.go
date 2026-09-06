package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/swim-services/swim_porter/cubemap"
	xdraw "golang.org/x/image/draw"
)

const skyManifestDescription = "Sky made with MEW"

type skyFace struct {
	name  string
	dirFn func(s, t float64) (float64, float64, float64)
}

var skyFaces = []skyFace{
	{"cubemap_0", func(s, t float64) (float64, float64, float64) { return -1, -t, s }},
	{"cubemap_1", func(s, t float64) (float64, float64, float64) { return s, -t, 1 }},
	{"cubemap_2", func(s, t float64) (float64, float64, float64) { return 1, -t, -s }},
	{"cubemap_3", func(s, t float64) (float64, float64, float64) { return -s, -t, -1 }},
	{"cubemap_4", func(s, t float64) (float64, float64, float64) { return s, 1, t }},
	{"cubemap_5", func(s, t float64) (float64, float64, float64) { return s, -1, -t }},
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

func cropCrossToCubemap(src image.Image, mode string, customMap [][2]int) [6]*image.RGBA {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	faceSize := 0
	is3x2 := false

	if srcH >= 3 && srcW >= 4 {
		fs := srcH / 3
		if fs*3 == srcH && fs*4 == srcW {
			faceSize = fs
		}
	}
	if faceSize == 0 && srcH >= 2 && srcW >= 3 {
		fs := srcH / 2
		if fs*2 == srcH && fs*3 == srcW {
			faceSize = fs
			is3x2 = true
		}
	}

	var faces [6]*image.RGBA
	var crossMap [6][2]int
	if mode == "custom" && len(customMap) == 6 {
		for i := 0; i < 6; i++ {
			crossMap[i] = customMap[i]
		}
	} else if is3x2 {
		if mode == "cross_b" {
			crossMap = [6][2]int{
				{2, 1}, {2, 0}, {1, 1}, {0, 1}, {1, 0}, {0, 0},
			}
		} else if mode == "cross_c" {
			crossMap = [6][2]int{
				{1, 1}, {2, 0}, {2, 1}, {0, 1}, {1, 0}, {0, 0},
			}
		} else if mode == "cross_e" {
			crossMap = [6][2]int{
				{2, 0}, {1, 1}, {2, 1}, {0, 1}, {1, 0}, {0, 0},
			}
		} else if mode == "cross_d" {
			crossMap = [6][2]int{
				{2, 1}, {0, 1}, {1, 1}, {2, 0}, {1, 0}, {0, 0},
			}
		} else {
			crossMap = [6][2]int{
				{1, 1}, {2, 1}, {2, 0}, {0, 1}, {1, 0}, {0, 0},
			}
		}
	} else {
		crossMap = [6][2]int{
			{0, 1}, {1, 1}, {2, 1}, {3, 1}, {1, 0}, {1, 2},
		}
	}

	for i, pos := range crossMap {
		col, row := pos[0], pos[1]
		sx := bounds.Min.X + col*faceSize
		sy := bounds.Min.Y + row*faceSize
		rect := image.Rect(0, 0, faceSize, faceSize)
		faceImg := image.NewRGBA(rect)
		xdraw.Draw(faceImg, rect, src, image.Point{X: sx, Y: sy}, xdraw.Src)
		faces[i] = faceImg
	}
	return faces
}

func rotateSquareRGBA(src *image.RGBA, deg int) *image.RGBA {
	n := src.Bounds().Dx()
	times := (((deg % 360) + 360) % 360) / 90
	cur := src
	for i := 0; i < times; i++ {
		next := image.NewRGBA(image.Rect(0, 0, n, n))
		for y := 0; y < n; y++ {
			for x := 0; x < n; x++ {
				r, g, b, a := cur.At(x, y).RGBA()
				next.SetRGBA(n-1-y, x, color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)})
			}
		}
		cur = next
	}
	return cur
}

func buildSkyPack(imageBytes []byte, fileName string, faceSize int, outDir, manifestDesc, mode string, customMap [][2]int) (string, error) {
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

	var faces [6]*image.RGBA
	if mode == "custom" || strings.HasPrefix(mode, "custom") {
		faces = cropCrossToCubemap(img, "custom", customMap)
	} else if strings.HasPrefix(mode, "cross") {
		faces = cropCrossToCubemap(img, mode, customMap)
	} else {
		faces = equirectToCubemap(img, faceSize)
		faces[4] = rotateSquareRGBA(faces[4], 270)
		faces[5] = rotateSquareRGBA(faces[5], 90)
	}
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

func (a *App) CreateSkyPack(imageBytes []byte, fileName string, faceSize int, mergePacks []string, mode string, customMap [][]int) (string, error) {
	a.logDebug(fmt.Sprintf("CreateSkyPack: called, file=%s, size=%d, faceSize=%d, mergePacks=%v, mode=%s", fileName, len(imageBytes), faceSize, mergePacks, mode))

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
	if a.getBoolSetting("deleteMcpack") && a.getBoolSetting("autoImport") {
		outDir = a.getTempDir("mcpack")
		os.MkdirAll(outDir, os.ModePerm)
	}
	a.emitProgress("Sky Converter", "Generating cubemap faces...", "info", fileName, 0, 0)

	var mapFixed [][2]int
	for _, m := range customMap {
		if len(m) == 2 {
			mapFixed = append(mapFixed, [2]int{m[0], m[1]})
		}
	}

	mcpackPath, err := buildSkyPack(imageBytes, fileName, faceSize, outDir, desc, mode, mapFixed)
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

const skySubpackDir = "assets/minecraft/mcpatcher/sky/world0"

type skyboxImage struct {
	name string
	img  image.Image
}

func scanSkyboxNames(reader *zip.Reader) []string {
	var names []string
	seen := map[string]bool{}
	for _, f := range reader.File {
		n := path.Clean(f.Name)
		if !strings.HasPrefix(n, skySubpackDir+"/") {
			continue
		}
		if !strings.EqualFold(path.Ext(n), ".png") {
			continue
		}
		base := path.Base(n)
		if seen[base] {
			continue
		}
		seen[base] = true
		names = append(names, f.Name)
	}
	sort.Strings(names)
	return names
}

func decodeSkybox(reader *zip.Reader, name string) (skyboxImage, bool) {
	rc, err := reader.Open(name)
	if err != nil {
		return skyboxImage{}, false
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return skyboxImage{}, false
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return skyboxImage{}, false
	}
	stem := strings.TrimSuffix(path.Base(name), path.Ext(name))
	return skyboxImage{name: stem, img: img}, true
}

func extractAllSkyboxes(javaBytes []byte) ([]skyboxImage, error) {
	reader, err := zip.NewReader(bytes.NewReader(javaBytes), int64(len(javaBytes)))
	if err != nil {
		return nil, err
	}

	var skies []skyboxImage
	for _, name := range scanSkyboxNames(reader) {
		if sky, ok := decodeSkybox(reader, name); ok {
			skies = append(skies, sky)
		}
	}
	return skies, nil
}

func extractPresetSkyboxes(javaBytes []byte, presets []string) ([]skyboxImage, error) {
	reader, err := zip.NewReader(bytes.NewReader(javaBytes), int64(len(javaBytes)))
	if err != nil {
		return nil, err
	}

	found := map[string]skyboxImage{}
	for _, name := range scanSkyboxNames(reader) {
		stem := strings.ToLower(strings.TrimSuffix(path.Base(name), path.Ext(name)))
		if sky, ok := decodeSkybox(reader, name); ok {
			found[stem] = sky
		}
	}

	var skies []skyboxImage
	for _, preset := range presets {
		key := strings.ToLower(strings.TrimSpace(preset))
		if key == "" {
			continue
		}
		if sky, ok := found[key]; ok {
			skies = append(skies, sky)
		}
	}
	return skies, nil
}

func buildSkyCubemapPngs(sky image.Image) ([6][]byte, error) {
	faces := cubemap.BuildCubemap(sky)
	var out [6][]byte
	for i, face := range faces {
		var buf bytes.Buffer
		if err := png.Encode(&buf, face); err != nil {
			return out, fmt.Errorf("failed to encode sky face %d: %v", i, err)
		}
		out[i] = buf.Bytes()
	}
	return out, nil
}

func addSkySubpacks(portOut []byte, skies []skyboxImage) ([]byte, error) {
	if len(skies) == 0 {
		return portOut, nil
	}

	reader, err := zip.NewReader(bytes.NewReader(portOut), int64(len(portOut)))
	if err != nil {
		return nil, err
	}

	var manifest map[string]interface{}
	manifestData, err := readZipFile(reader, "manifest.json")
	if err != nil {
		return nil, err
	}

	remaining := map[string][]byte{}
	for _, f := range reader.File {
		if path.Clean(f.Name) == "manifest.json" {
			continue
		}
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}
		remaining[f.Name] = data
	}

	var subpacks []interface{}
	for _, sky := range skies {
		faces, err := buildSkyCubemapPngs(sky.img)
		if err != nil {
			continue
		}
		prefix := "subpacks/" + sky.name + "/textures/environment/overworld_cubemap/"
		for i, face := range faces {
			remaining[fmt.Sprintf("%scubemap_%d.png", prefix, i)] = face
		}
		subpacks = append(subpacks, map[string]interface{}{
			"folder_name": sky.name,
			"name":        sky.name,
			"memory_tier": 0,
		})
	}

	if len(subpacks) > 0 {
		if err := json.Unmarshal(manifestData, &manifest); err != nil {
			return nil, err
		}
		manifest["subpacks"] = subpacks
		manifestData, err = json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return nil, err
		}
		remaining["manifest.json"] = manifestData
	}

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	names := make([]string, 0, len(remaining))
	for name := range remaining {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		f, err := writer.Create(name)
		if err != nil {
			writer.Close()
			return nil, err
		}
		if _, err := f.Write(remaining[name]); err != nil {
			writer.Close()
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (a *App) maybeAddSkySubpacks(javaBytes, portOut []byte) ([]byte, error) {
	if len(javaBytes) == 0 {
		return portOut, nil
	}

	var skies []skyboxImage
	if a.getBoolSetting("portAllSkies") {
		found, err := extractAllSkyboxes(javaBytes)
		if err != nil {
			a.logDebug(fmt.Sprintf("maybeAddSkySubpacks: extract all failed: %v", err))
			return portOut, nil
		}
		skies = found
	} else if a.getBoolSetting("skyPresets") {
		presets := a.getStringArraySetting("skyPresetNames")
		if len(presets) == 0 {
			return portOut, nil
		}
		found, err := extractPresetSkyboxes(javaBytes, presets)
		if err != nil {
			a.logDebug(fmt.Sprintf("maybeAddSkySubpacks: extract presets failed: %v", err))
			return portOut, nil
		}
		skies = found
	} else {
		return portOut, nil
	}

	if len(skies) == 0 {
		return portOut, nil
	}
	withSubpacks, err := addSkySubpacks(portOut, skies)
	if err != nil {
		a.logDebug(fmt.Sprintf("maybeAddSkySubpacks: add failed: %v", err))
		return portOut, nil
	}
	a.logDebug(fmt.Sprintf("maybeAddSkySubpacks: added %d sky subpack(s)", len(skies)))
	return withSubpacks, nil
}

func readZipFile(reader *zip.Reader, name string) ([]byte, error) {
	for _, f := range reader.File {
		if path.Clean(f.Name) == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fs.ErrNotExist
}

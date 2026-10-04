package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/woozymasta/tga"
)

type ResourcePacksInfo struct {
	Path  string   `json:"path"`
	Found bool     `json:"found"`
	Packs []string `json:"packs"`
}

type PackInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

type DetailedPacksInfo struct {
	Path  string     `json:"path"`
	Found bool       `json:"found"`
	Packs []PackInfo `json:"packs"`
}

type PackListEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IconURI     string `json:"iconURI"`
	Size        int64  `json:"size"`
	ModTime     string `json:"modTime"`
	DirName     string `json:"dirName"`
}

type PackPreviewInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IconURI     string `json:"iconURI"`
}

type ArmorTextures struct {
	Layer1 string `json:"layer1"`
	Layer2 string `json:"layer2"`
}

type ItemTexture struct {
	Name    string `json:"name"`
	DataURI string `json:"dataURI"`
}

type SkyTextures struct {
	Cubemap0 string `json:"cubemap0"`
	Cubemap1 string `json:"cubemap1"`
	Cubemap2 string `json:"cubemap2"`
	Cubemap3 string `json:"cubemap3"`
	Cubemap4 string `json:"cubemap4"`
	Cubemap5 string `json:"cubemap5"`
}

type SkySubpack struct {
	FolderName string `json:"folderName"`
	Name       string `json:"name"`
}
type FolderImage struct {
	Filename string `json:"filename"`
	DataURI  string `json:"dataURI"`
	RelPath  string `json:"relPath"`
	Size     int64  `json:"size"`
}

type CheckResult struct {
	SessionId string   `json:"sessionId"`
	Folders   []string `json:"folders"`
	Valid     bool     `json:"valid"`
	ErrorMsg  string   `json:"errorMsg,omitempty"`
}

type SaveImageRequest struct {
	ImageName string `json:"imageName"`
	ImagePath string `json:"imagePath"`
	RelPath   string `json:"relPath"`
	ImageData string `json:"imageData"`
	Done      bool   `json:"done"`
}

func (a *App) GetInstalledPacks() ResourcePacksInfo {
	path := a.getResourcePacksPath()
	info := ResourcePacksInfo{Path: path}
	if st, err := os.Stat(path); err != nil || !st.IsDir() {
		return info
	}
	info.Found = true
	entries, err := os.ReadDir(path)
	if err != nil {
		return info
	}
	for _, e := range entries {
		if e.IsDir() || strings.HasSuffix(strings.ToLower(e.Name()), ".mcpack") {
			info.Packs = append(info.Packs, e.Name())
		}
	}
	sort.Strings(info.Packs)
	return info
}

type CachePackSource struct {
	Name  string   `json:"name"`
	Path  string   `json:"path"`
	Found bool     `json:"found"`
	Packs []string `json:"packs"`
}

func (a *App) GetPackCache() []CachePackSource {
	path := a.getStringSetting("packCachePath")
	if path == "" {
		path = a.getDefaultPackCachePath()
	}
	src := CachePackSource{Name: "Minecraft Pack Cache", Path: path}
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		src.Found = true
		entries, err := os.ReadDir(path)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					src.Packs = append(src.Packs, e.Name())
				}
			}
			sort.Strings(src.Packs)
		}
	}
	return []CachePackSource{src}
}

func dirSize(path string) int64 {
	var size int64
	filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err == nil {
			size += info.Size()
		}
		return nil
	})
	return size
}

func (a *App) listPacksWithInfo(base string) ([]PackListEntry, error) {
	if st, err := os.Stat(base); err != nil || !st.IsDir() {
		return nil, nil
	}

	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}

	var result []PackListEntry
	for _, e := range entries {
		if !e.IsDir() && !strings.HasSuffix(strings.ToLower(e.Name()), ".mcpack") {
			continue
		}

		dir := filepath.Join(base, e.Name())
		st, err := os.Stat(dir)
		if err != nil {
			continue
		}

		entry := PackListEntry{
			Name:    e.Name(),
			DirName: e.Name(),
			ModTime: st.ModTime().Format("2006-01-02 15:04:05"),
		}

		entry.Size = dirSize(dir)

		manifestPath := filepath.Join(dir, "manifest.json")
		if data, err := os.ReadFile(manifestPath); err == nil {
			var manifest struct {
				Header struct {
					Name        string `json:"name"`
					Description string `json:"description"`
				} `json:"header"`
			}
			if json.Unmarshal(data, &manifest) == nil {
				if manifest.Header.Name != "" {
					entry.Name = manifest.Header.Name
				}
				entry.Description = manifest.Header.Description
			}
		}

		iconPath := findFirstImage(dir, "pack_icon")
		if iconPath != "" {
			entry.IconURI = a.readImageAsDataURI(iconPath)
		}

		result = append(result, entry)
	}

	return result, nil
}

func (a *App) GetPackListWithInfo() ([]PackListEntry, error) {
	path := a.getResourcePacksPath()
	return a.listPacksWithInfo(path)
}

func (a *App) GetPackCacheList(basePath string) ([]PackListEntry, error) {
	if basePath == "" {
		basePath = a.getDefaultPackCachePath()
	}
	return a.listPacksWithInfo(basePath)
}

func (a *App) GetInstalledPacksDetailed() DetailedPacksInfo {
	path := a.getResourcePacksPath()
	info := DetailedPacksInfo{Path: path}
	if st, err := os.Stat(path); err != nil || !st.IsDir() {
		return info
	}
	info.Found = true
	entries, err := os.ReadDir(path)
	if err != nil {
		return info
	}
	for _, e := range entries {
		if !e.IsDir() && !strings.HasSuffix(strings.ToLower(e.Name()), ".mcpack") {
			continue
		}
		packDir := filepath.Join(path, e.Name())
		pi := PackInfo{Name: e.Name()}

		manifestPath := filepath.Join(packDir, "manifest.json")
		if mData, mErr := os.ReadFile(manifestPath); mErr == nil {
			var manifest map[string]interface{}
			if json.Unmarshal(mData, &manifest) == nil {
				if header, ok := manifest["header"].(map[string]interface{}); ok {
					if desc, ok := header["description"].(string); ok {
						pi.Description = desc
					}
				}
			}
		}

		for _, iconBase := range []string{"pack_icon.png", "pack_icon.jpeg", "pack_icon.jpg"} {
			iconPath := filepath.Join(packDir, iconBase)
			if iconData, iErr := os.ReadFile(iconPath); iErr == nil {
				pi.Icon = "data:image/png;base64," + base64.StdEncoding.EncodeToString(iconData)
				break
			}
		}

		info.Packs = append(info.Packs, pi)
	}
	sort.Slice(info.Packs, func(i, j int) bool {
		return info.Packs[i].Name < info.Packs[j].Name
	})
	return info
}

func (a *App) DeleteInstalledPack(packName string) error {
	path := a.getResourcePacksPath()
	if packName == "" || strings.Contains(packName, "..") || strings.ContainsAny(packName, `\/`) {
		return fmt.Errorf("invalid pack name")
	}
	packDir := filepath.Join(path, packName)
	st, err := os.Stat(packDir)
	if err != nil || !st.IsDir() {
		return fmt.Errorf("pack not found: %s", packName)
	}
	a.logDebug(fmt.Sprintf("DeleteInstalledPack: removing %s", packDir))
	return os.RemoveAll(packDir)
}

func (a *App) getPackDir(packName string) string {
	base := a.getResourcePacksPath()
	return filepath.Join(base, packName)
}

func findFirstImage(dir string, base string) string {
	exts := []string{".png", ".tga", ".jpg", ".jpeg"}
	for _, ext := range exts {
		p := filepath.Join(dir, base+ext)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func (a *App) readImageAsDataURI(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	ext := strings.ToLower(filepath.Ext(path))

	if ext == ".tga" {
		img, err := tga.Decode(bytes.NewReader(data))
		if err != nil {
			return ""
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return ""
		}
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	}

	mime := "image/png"
	if ext == ".jpg" || ext == ".jpeg" {
		mime = "image/jpeg"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func (a *App) GetPackPreviewInfo(packName string) (PackPreviewInfo, error) {
	dir := a.getPackDir(packName)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return PackPreviewInfo{}, fmt.Errorf("pack not found: %s", packName)
	}

	info := PackPreviewInfo{Name: packName}

	manifestPath := filepath.Join(dir, "manifest.json")
	if data, err := os.ReadFile(manifestPath); err == nil {
		var manifest struct {
			Header struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"header"`
		}
		if json.Unmarshal(data, &manifest) == nil {
			if manifest.Header.Name != "" {
				info.Name = manifest.Header.Name
			}
			info.Description = manifest.Header.Description
		}
	}

	iconPath := findFirstImage(dir, "pack_icon")
	if iconPath != "" {
		info.IconURI = a.readImageAsDataURI(iconPath)
	}

	return info, nil
}

func (a *App) GetPackArmorTextures(packName string, material string) (ArmorTextures, error) {
	dir := a.getPackDir(packName)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return ArmorTextures{}, fmt.Errorf("pack not found: %s", packName)
	}

	texturesDir := filepath.Join(dir, "textures", "models", "armor")
	if st, err := os.Stat(texturesDir); err != nil || !st.IsDir() {
		return ArmorTextures{}, nil
	}

	result := ArmorTextures{}

	layer1Path := findFirstImage(texturesDir, material+"_1")
	if layer1Path != "" {
		result.Layer1 = a.readImageAsDataURI(layer1Path)
	}

	layer2Path := findFirstImage(texturesDir, material+"_2")
	if layer2Path != "" {
		result.Layer2 = a.readImageAsDataURI(layer2Path)
	}

	return result, nil
}

func (a *App) GetPackSkyTextures(packName string) (SkyTextures, error) {
	dir := a.getPackDir(packName)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return SkyTextures{}, fmt.Errorf("pack not found: %s", packName)
	}

	envDir := filepath.Join(dir, "textures", "environment")
	if st, err := os.Stat(envDir); err != nil || !st.IsDir() {
		return SkyTextures{}, nil
	}

	cubemapDir := filepath.Join(envDir, "overworld_cubemap")

	result := SkyTextures{}
	for i := 0; i <= 5; i++ {
		p := findFirstImage(cubemapDir, fmt.Sprintf("cubemap_%d", i))
		if p == "" {
			p = findFirstImage(envDir, fmt.Sprintf("cubemap_%d", i))
		}
		if p == "" {
			continue
		}
		uri := a.readImageAsDataURI(p)
		switch i {
		case 0:
			result.Cubemap0 = uri
		case 1:
			result.Cubemap1 = uri
		case 2:
			result.Cubemap2 = uri
		case 3:
			result.Cubemap3 = uri
		case 4:
			result.Cubemap4 = uri
		case 5:
			result.Cubemap5 = uri
		}
	}
	return result, nil
}

func (a *App) readSkyCubemap(cubemapDir string, envDir string) SkyTextures {
	result := SkyTextures{}
	for i := 0; i <= 5; i++ {
		p := findFirstImage(cubemapDir, fmt.Sprintf("cubemap_%d", i))
		if p == "" {
			p = findFirstImage(envDir, fmt.Sprintf("cubemap_%d", i))
		}
		if p == "" {
			continue
		}
		uri := a.readImageAsDataURI(p)
		if uri == "" {
			continue
		}
		switch i {
		case 0:
			result.Cubemap0 = uri
		case 1:
			result.Cubemap1 = uri
		case 2:
			result.Cubemap2 = uri
		case 3:
			result.Cubemap3 = uri
		case 4:
			result.Cubemap4 = uri
		case 5:
			result.Cubemap5 = uri
		}
	}
	return result
}

func hasSkyCubemap(cubemapDir string, envDir string) bool {
	for i := 0; i <= 5; i++ {
		if findFirstImage(cubemapDir, fmt.Sprintf("cubemap_%d", i)) != "" {
			return true
		}
		if findFirstImage(envDir, fmt.Sprintf("cubemap_%d", i)) != "" {
			return true
		}
	}
	return false
}

func (a *App) GetPackSkySubpacks(packName string) ([]SkySubpack, error) {
	dir := a.getPackDir(packName)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("pack not found: %s", packName)
	}

	manifestPath := filepath.Join(dir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, nil
	}

	var manifest struct {
		Subpacks []struct {
			FolderName string `json:"folder_name"`
			Name       string `json:"name"`
		} `json:"subpacks"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, nil
	}
	out := make([]SkySubpack, 0, len(manifest.Subpacks))
	for _, sp := range manifest.Subpacks {
		if sp.FolderName == "" {
			continue
		}
		subpackDir := filepath.Join(dir, "subpacks", sp.FolderName)
		envDir := filepath.Join(subpackDir, "textures", "environment")
		cubemapDir := filepath.Join(envDir, "overworld_cubemap")
		if !hasSkyCubemap(cubemapDir, envDir) {
			continue
		}
		out = append(out, SkySubpack{FolderName: sp.FolderName, Name: sp.Name})
	}
	return out, nil
}

func (a *App) GetPackSkySubpackTextures(packName string, subpackName string) (SkyTextures, error) {
	dir := a.getPackDir(packName)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return SkyTextures{}, fmt.Errorf("pack not found: %s", packName)
	}

	subpackDir := filepath.Join(dir, "subpacks", subpackName)
	if st, err := os.Stat(subpackDir); err != nil || !st.IsDir() {
		return SkyTextures{}, nil
	}

	envDir := filepath.Join(subpackDir, "textures", "environment")
	if st, err := os.Stat(envDir); err != nil || !st.IsDir() {
		return SkyTextures{}, nil
	}
	cubemapDir := filepath.Join(envDir, "overworld_cubemap")
	return a.readSkyCubemap(cubemapDir, envDir), nil
}


func (a *App) GetPackItemTextures(packName string, material string) ([]ItemTexture, error) {
	dir := a.getPackDir(packName)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("pack not found: %s", packName)
	}

	itemsDir := filepath.Join(dir, "textures", "items")
	if st, err := os.Stat(itemsDir); err != nil || !st.IsDir() {
		return nil, nil
	}

	type itemEntry struct {
		name  string
		alias string
	}

	tierMap := map[string]string{
		"cloth":     "wood",
		"chain":     "stone",
		"iron":      "iron",
		"gold":      "gold",
		"diamond":   "diamond",
		"netherite": "netherite",
	}
	tier := tierMap[material]

	var armorPrefix string
	switch material {
	case "cloth":
		armorPrefix = "leather"
	case "chain":
		armorPrefix = "chainmail"
	default:
		armorPrefix = material
	}

	itemEntries := []itemEntry{
		{armorPrefix + "_helmet", ""},
		{armorPrefix + "_chestplate", ""},
		{armorPrefix + "_leggings", ""},
		{armorPrefix + "_boots", ""},
		{tier + "_sword", ""},
		{tier + "_pickaxe", ""},
		{tier + "_axe", ""},
		{tier + "_shovel", ""},
		{"ender_pearl", ""},
		{"splash_potion", ""},
		{"golden_apple", ""},
		{"apple_golden", ""},
		{"potion_bottle_splash_heal", "splash_potion_heal"},
		{"bow_standby", "bow"},
		{"fishing_rod_uncast", "fishing_rod"},
	}

	var items []ItemTexture
	removed := a.GetRemovedItems()
	removedSet := make(map[string]bool, len(removed))
	for _, r := range removed {
		removedSet[r] = true
	}
	for _, entry := range itemEntries {
		if removedSet[entry.name] {
			continue
		}
		imgPath := findFirstImage(itemsDir, entry.name)
		if imgPath == "" && entry.alias != "" {
			imgPath = findFirstImage(itemsDir, entry.alias)
		}
		if imgPath == "" {
			continue
		}
		uri := a.readImageAsDataURI(imgPath)
		if uri == "" {
			continue
		}
		items = append(items, ItemTexture{Name: entry.name, DataURI: uri})
	}

	customItems := a.GetCustomItems()
	for _, customName := range customItems {
		if removedSet[customName] {
			continue
		}
		imgPath := findFirstImage(itemsDir, customName)
		if imgPath == "" {
			continue
		}
		uri := a.readImageAsDataURI(imgPath)
		if uri == "" {
			continue
		}
		items = append(items, ItemTexture{Name: customName, DataURI: uri})
	}

	return items, nil
}

func (a *App) GetPackAllItemTextures(packName string) ([]ItemTexture, error) {
	dir := a.getPackDir(packName)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("pack not found: %s", packName)
	}

	itemsDir := filepath.Join(dir, "textures", "items")
	if st, err := os.Stat(itemsDir); err != nil || !st.IsDir() {
		return nil, nil
	}

	entries, err := os.ReadDir(itemsDir)
	if err != nil {
		return nil, err
	}

	exts := map[string]bool{".png": true, ".tga": true, ".jpg": true, ".jpeg": true}
	var items []ItemTexture
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !exts[ext] {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ext)
		fullPath := filepath.Join(itemsDir, entry.Name())
		uri := a.readImageAsDataURI(fullPath)
		if uri == "" {
			continue
		}
		items = append(items, ItemTexture{Name: name, DataURI: uri})
	}

	return items, nil
}

func (a *App) GetPackItemTextureNames(packName string) ([]string, error) {
	dir := a.getPackDir(packName)
	itemsDir := filepath.Join(dir, "textures", "items")
	if st, err := os.Stat(itemsDir); err != nil || !st.IsDir() {
		return nil, nil
	}

	entries, err := os.ReadDir(itemsDir)
	if err != nil {
		return nil, err
	}

	exts := map[string]bool{".png": true, ".tga": true, ".jpg": true, ".jpeg": true}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !exts[ext] {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ext)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

func (a *App) GetPackItemTexture(packName string, name string) (string, error) {
	dir := a.getPackDir(packName)
	itemsDir := filepath.Join(dir, "textures", "items")
	if st, err := os.Stat(itemsDir); err != nil || !st.IsDir() {
		return "", nil
	}

	guard := filepath.Clean(name)
	if strings.ContainsAny(guard, "/\\") || guard == "." || guard == ".." || strings.HasPrefix(guard, "..") {
		return "", nil
	}

	imgPath := findFirstImage(itemsDir, name)
	if imgPath == "" {
		return "", nil
	}
	return a.readImageAsDataURI(imgPath), nil
}

func (a *App) GetPlayerSkinTexture(packName string) string {
	dir := a.getPackDir(packName)

	candidates := []string{
		filepath.Join(dir, "textures", "entity", "player", "steve.png"),
		filepath.Join(dir, "textures", "entity", "player.png"),
		filepath.Join(dir, "textures", "entity", "steve.png"),
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return a.readImageAsDataURI(p)
		}
	}

	return ""
}

func (a *App) GetDefaultSkin() string {
	p := filepath.Join(os.Getenv("LOCALAPPDATA"), "mew", "default_skin.png")
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return a.readImageAsDataURI(p)
}

func (a *App) SaveDefaultSkin(dataURI string) error {
	parts := strings.SplitN(dataURI, ",", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid data URI")
	}
	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return err
	}
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "mew")
	os.MkdirAll(dir, os.ModePerm)
	return os.WriteFile(filepath.Join(dir, "default_skin.png"), decoded, 0644)
}

func (a *App) GetCustomItems() []string {
	p := filepath.Join(os.Getenv("LOCALAPPDATA"), "mew", "custom_items.json")
	data, err := os.ReadFile(p)
	if err != nil {
		return []string{}
	}
	var items []string
	if err := json.Unmarshal(data, &items); err != nil {
		return []string{}
	}
	return items
}

func (a *App) SaveCustomItem(name string) error {
	items := a.GetCustomItems()
	for _, existing := range items {
		if existing == name {
			return nil
		}
	}
	items = append(items, name)
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "mew")
	os.MkdirAll(dir, os.ModePerm)
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "custom_items.json"), data, 0644)
}

func (a *App) RemoveCustomItem(name string) error {
	items := a.GetCustomItems()
	var filtered []string
	for _, existing := range items {
		if existing != name {
			filtered = append(filtered, existing)
		}
	}
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "mew")
	os.MkdirAll(dir, os.ModePerm)
	data, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "custom_items.json"), data, 0644)
}

func (a *App) GetRemovedItems() []string {
	p := filepath.Join(os.Getenv("LOCALAPPDATA"), "mew", "removed_items.json")
	data, err := os.ReadFile(p)
	if err != nil {
		return []string{}
	}
	var items []string
	if err := json.Unmarshal(data, &items); err != nil {
		return []string{}
	}
	return items
}

func (a *App) SaveRemovedItem(name string) error {
	items := a.GetRemovedItems()
	for _, existing := range items {
		if existing == name {
			return nil
		}
	}
	items = append(items, name)
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "mew")
	os.MkdirAll(dir, os.ModePerm)
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "removed_items.json"), data, 0644)
}

func (a *App) RestoreRemovedItem(name string) error {
	items := a.GetRemovedItems()
	var filtered []string
	for _, existing := range items {
		if existing != name {
			filtered = append(filtered, existing)
		}
	}
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "mew")
	os.MkdirAll(dir, os.ModePerm)
	data, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "removed_items.json"), data, 0644)
}

func (a *App) GetImages(sessionId string, folder string) ([]FolderImage, error) {
	re := regexp.MustCompile(`\\\s`)
	folder = re.ReplaceAllString(folder, `\`)

	sessionDir, err := a.sessionDir(sessionId)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(sessionDir, filepath.FromSlash(folder))

	var images []FolderImage

	err = filepath.Walk(fullPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.Contains(path, filepath.Join("textures", "entity")) ||
			strings.Contains(path, "textures\\entity") {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
			imageData, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			filename := filepath.Base(path)
			nameWithoutExt := strings.TrimSuffix(filename, ext)

			mimeType := "image/png"
			if ext == ".jpg" || ext == ".jpeg" {
				mimeType = "image/jpeg"
			}

			encoded := base64.StdEncoding.EncodeToString(imageData)
			dataURI := "data:" + mimeType + ";base64," + encoded

			relPath, err := filepath.Rel(sessionDir, path)
			if err != nil {
				relPath = path
			}
			relPath = filepath.ToSlash(relPath)

			images = append(images, FolderImage{
				Filename: nameWithoutExt,
				DataURI:  dataURI,
				RelPath:  relPath,
				Size:     info.Size(),
			})
		}
		return nil
	})

	return images, err
}

func (a *App) SaveImage(sessionId string, msg SaveImageRequest) (string, error) {
	if msg.Done {
		return "done", nil
	}

	imageData, err := base64.StdEncoding.DecodeString(msg.ImageData)
	if err != nil {
		return "", fmt.Errorf("error decoding base64 image: %v", err)
	}

	sessionDir, err := a.sessionDir(sessionId)
	if err != nil {
		return "", err
	}

	var savePath string
	if msg.RelPath != "" {
		clean, err := safeRelPath(msg.RelPath)
		if err != nil {
			return "", fmt.Errorf("invalid image path: %v", err)
		}
		savePath = filepath.Join(sessionDir, clean)
	} else {
		var foundPath string
		filepath.Walk(sessionDir, func(path string, info fs.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if filepath.Base(path) == msg.ImageName {
				foundPath = path
				return fmt.Errorf("found")
			}
			return nil
		})

		if foundPath != "" {
			savePath = foundPath
		} else {
			savePath = filepath.Join(sessionDir, "textures", filepath.Base(msg.ImageName))
		}
	}

	if err := os.MkdirAll(filepath.Dir(savePath), 0755); err != nil {
		return "", fmt.Errorf("failed to create texture folder: %v", err)
	}

	if err := os.WriteFile(savePath, imageData, 0644); err != nil {
		return "", fmt.Errorf("failed to save image: %v", err)
	}

	a.touchSession(sessionId)
	return "success", nil
}

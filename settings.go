package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) getSettingsPath() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
	}
	dir := filepath.Join(localAppData, "mew", "Settings")
	os.MkdirAll(dir, os.ModePerm)
	return filepath.Join(dir, "settings.json")
}

func (a *App) GetSettings() map[string]interface{} {
	data, err := os.ReadFile(a.getSettingsPath())
	if err != nil {
		return a.settings
	}
	var loaded map[string]interface{}
	if err := json.Unmarshal(data, &loaded); err != nil {
		return a.settings
	}
	for k, v := range loaded {
		a.settings[k] = v
	}
	return a.settings
}

func (a *App) SaveSettings(settings map[string]interface{}) {
	a.settings = settings
	jsonData, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(a.getSettingsPath(), jsonData, 0644)
}

func (a *App) ClearCache() error {
	os.Remove(a.getRecentPacksPath())
	os.RemoveAll(filepath.Join(os.Getenv("LOCALAPPDATA"), "mew", "temp"))
	return nil
}

func getMewTempDir(subdir string) string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "mew", "temp", subdir)
}

func (a *App) getTempDir(subdir string) string {
	return getMewTempDir(subdir)
}

func (a *App) getOutputDir() string {
	if dir, ok := a.settings["customOutputDir"].(string); ok && dir != "" {
		return dir
	}
	return "."
}

// resolveOutputDir turns the configured output dir into a directory we can
// actually write to. The app is often launched from a protected folder (e.g.
// Program Files), which used to make every export fail with "access is denied".
func (a *App) resolveOutputDir() (string, error) {
	candidate := a.getOutputDir()
	if candidate == "" {
		candidate = "."
	}
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("invalid output directory %q: %v", candidate, err)
	}
	if err := os.MkdirAll(abs, 0755); err == nil && dirWritable(abs) {
		return abs, nil
	}

	if home, err := os.UserHomeDir(); err == nil {
		fallback := filepath.Join(home, "Downloads")
		if err := os.MkdirAll(fallback, 0755); err == nil && dirWritable(fallback) {
			a.logDebug(fmt.Sprintf("output dir %q not writable, falling back to %q", abs, fallback))
			return fallback, nil
		}
	}
	return "", fmt.Errorf("no writable output folder (tried %s and your Downloads folder)", abs)
}

func dirWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".mew-write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}

func (a *App) getBoolSetting(key string) bool {
	if val, ok := a.settings[key].(bool); ok {
		return val
	}
	return false
}

func (a *App) getStringSetting(key string) string {
	if val, ok := a.settings[key].(string); ok {
		return val
	}
	return ""
}

func (a *App) getStringArraySetting(key string) []string {
	if val, ok := a.settings[key].([]string); ok {
		return val
	}
	if val, ok := a.settings[key].([]interface{}); ok {
		var out []string
		for _, v := range val {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

type MinecraftPath struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// leviVersionPath returns the version label embedded in a levilauncher
// resource_packs path, or "" when the path is not under levilauncher's versions.
func leviVersionFromPath(p string) string {
	marker := "/levilauncher.exe/versions/"
	s := filepath.ToSlash(p)
	idx := strings.Index(strings.ToLower(s), marker)
	if idx < 0 {
		return ""
	}
	rest := s[idx+len(marker):]
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	return parts[0]
}

func parseVersion(v string) [4]int {
	var out [4]int
	parts := strings.Split(strings.TrimLeft(v, "vV"), ".")
	for i := 0; i < len(parts) && i < 4; i++ {
		n, err := strconv.Atoi(parts[i])
		if err == nil {
			out[i] = n
		}
	}
	return out
}

func compareVersions(a, b string) int {
	pa := parseVersion(a)
	pb := parseVersion(b)
	for i := 0; i < 4; i++ {
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// detectLeviResourcePacksPaths returns the levilauncher resource_packs
// folders, newest version first.
func detectLeviResourcePacksPaths() []MinecraftPath {
	base := filepath.Join(os.Getenv("APPDATA"), "levilauncher.exe", "versions")
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	type vp struct {
		ver  [4]int
		path MinecraftPath
	}
	var found []vp
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		ver := e.Name()
		p := filepath.Join(base, ver, "Minecraft Bedrock", "Users", "Shared", "games", "com.mojang", "resource_packs")
		if st, err := os.Stat(p); err != nil || !st.IsDir() {
			continue
		}
		found = append(found, vp{ver: parseVersion(ver), path: MinecraftPath{Name: "Minecraft (LeviLauncher " + ver + ")", Path: p}})
	}
	sort.Slice(found, func(i, j int) bool {
		for k := 0; k < 4; k++ {
			if found[i].ver[k] != found[j].ver[k] {
				return found[i].ver[k] > found[j].ver[k]
			}
		}
		return false
	})
	var out []MinecraftPath
	for _, f := range found {
		out = append(out, f.path)
	}
	return out
}

// newestLeviResourcePacks returns the newest installed levilauncher
// resource_packs folder (path), or "" when none exists.
func newestLeviResourcePacks() string {
	paths := detectLeviResourcePacksPaths()
	if len(paths) > 0 {
		return paths[0].Path
	}
	return ""
}

func (a *App) DetectMinecraftPaths() []MinecraftPath {
	var paths []MinecraftPath
	appData := os.Getenv("APPDATA")
	localAppData := os.Getenv("LOCALAPPDATA")

	addIfExists := func(name, path string) {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			paths = append(paths, MinecraftPath{Name: name, Path: path})
		}
	}

	addIfExists("Minecraft (Windows)", filepath.Join(appData, "Minecraft Bedrock", "Users", "Shared", "games", "com.mojang", "resource_packs"))
	addIfExists("Minecraft (Legacy UWP)", filepath.Join(localAppData, "Packages", "Microsoft.MinecraftUWP_8wekyb3d8bbwe", "LocalState", "games", "com.mojang", "resource_packs"))
	addIfExists("Minecraft Preview", filepath.Join(appData, "Minecraft Bedrock Preview", "Users", "Shared", "games", "com.mojang", "resource_packs"))

	paths = append(paths, detectLeviResourcePacksPaths()...)

	return paths
}

// getResourcePacksPath resolves the active Bedrock resource_packs folder.
// A configured path is honored unless it points at an outdated levilauncher
// version — in that case the newest installed version wins.
func (a *App) getResourcePacksPath() string {
	if p, ok := a.settings["resourcePacksPath"].(string); ok && p != "" {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			if ver := leviVersionFromPath(p); ver != "" {
				if newest := newestLeviResourcePacks(); newest != "" && newest != p && compareVersions(ver, leviVersionFromPath(newest)) < 0 {
					a.logDebug(fmt.Sprintf("mew: resourcePacksPath %q points to older levilauncher version %s, switching to %s", p, ver, newest))
					a.settings["resourcePacksPath"] = newest
					a.SaveSettings(a.settings)
					return newest
				}
			}
			return p
		}
		a.logDebug(fmt.Sprintf("mew: configured resourcePacksPath %q not found, falling back to detection", p))
	}
	return a.getDefaultResourcePacksPath()
}

func (a *App) getDefaultResourcePacksPath() string {
	detected := a.DetectMinecraftPaths()
	if len(detected) > 0 {
		return detected[0].Path
	}
	return filepath.Join(os.Getenv("APPDATA"), "Minecraft Bedrock", "Users", "Shared", "games", "com.mojang", "resource_packs")
}

func (a *App) getDefaultPackCachePath() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "Temp", "Minecraft Bedrock", "minecraftpe", "packcache", "resource")
}

func (a *App) SelectDirectory() (string, error) {
	dir, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Output Directory",
	})
	if err != nil {
		return "", err
	}
	return dir, nil
}

func (a *App) SetResourcePacksPath(path string) (ResourcePacksInfo, error) {
	st, err := os.Stat(path)
	if err != nil || !st.IsDir() {
		return ResourcePacksInfo{}, fmt.Errorf("please choose a valid folder")
	}
	a.settings["resourcePacksPath"] = path
	a.SaveSettings(a.settings)
	return a.GetInstalledPacks(), nil
}

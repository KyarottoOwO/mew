package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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

type MinecraftPath struct {
	Name string `json:"name"`
	Path string `json:"path"`
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

	return paths
}

func (a *App) getDefaultResourcePacksPath() string {
	detected := a.DetectMinecraftPaths()
	if len(detected) > 0 {
		return detected[0].Path
	}
	return filepath.Join(os.Getenv("APPDATA"), "Minecraft Bedrock", "Users", "Shared", "games", "com.mojang", "resource_packs")
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

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/swim-services/swim_porter/port"
	"github.com/swim-services/swim_porter/porterror"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var currentVersion = "1.1.1"
const githubReleasesURL = "https://api.github.com/repos/KyarottoOwO/mew/releases/latest"

type UpdateInfo struct {
	NeedsUpdate    bool   `json:"needsUpdate"`
	LatestVersion  string `json:"latestVersion"`
	DownloadURL    string `json:"downloadUrl"`
}

type RecentPack struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Timestamp string `json:"timestamp"`
}

type App struct {
	ctx             context.Context
	cancelFolder    context.CancelFunc
	settings        map[string]interface{}
	debug           bool
	portMu          sync.Mutex
	activePort      bool
	discordStop      chan struct{}
	discordConns     map[string]net.Conn
	discordState     string
	discordDetails   string
	discordMu        sync.Mutex
	discordIpcMu     sync.Mutex
	discordStart     time.Time
}

func NewApp(debug bool) *App {
	return &App{
		debug: debug,
		settings: map[string]interface{}{
			"autoImport":            false,
			"autoOpenFolder":        false,
			"deleteOriginals":       false,
			"deleteMcpack":          false,
			"discordRPC":            true,
			"customOutputDir":       "",
			"manifestDescription":   "",
			"resourcePacksPath":     "",
		},
		discordConns: map[string]net.Conn{},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.debug {
		log.Println("[startup] App context initialized")
	}
	a.startDiscordRPC()
}

func (a *App) logDebug(msg string) {
	if a.debug {
		log.Println("[DEBUG]", msg)
	}
}

func (a *App) acquirePort() error {
	a.portMu.Lock()
	if a.activePort {
		a.portMu.Unlock()
		a.logDebug("acquirePort: BLOCKED - another porting operation is in progress")
		return fmt.Errorf("another porting operation is in progress. Please wait for it to finish.")
	}
	a.activePort = true
	a.portMu.Unlock()
	a.logDebug("acquirePort: acquired")
	return nil
}

func (a *App) releasePort() {
	a.portMu.Lock()
	a.activePort = false
	a.portMu.Unlock()
	a.logDebug("releasePort: released")
}

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

type ResourcePacksInfo struct {
	Path  string   `json:"path"`
	Found bool     `json:"found"`
	Packs []string `json:"packs"`
}

func (a *App) GetInstalledPacks() ResourcePacksInfo {
	path := a.getStringSetting("resourcePacksPath")
	if path == "" {
		path = a.getDefaultResourcePacksPath()
	}
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

func (a *App) SetResourcePacksPath(path string) (ResourcePacksInfo, error) {
	st, err := os.Stat(path)
	if err != nil || !st.IsDir() {
		return ResourcePacksInfo{}, fmt.Errorf("please choose a valid folder")
	}
	a.settings["resourcePacksPath"] = path
	a.SaveSettings(a.settings)
	return a.GetInstalledPacks(), nil
}

func (a *App) importToBedrock(mcpackPath string) {
	bedrockPath := a.getStringSetting("resourcePacksPath")
	if bedrockPath == "" {
		bedrockPath = a.getDefaultResourcePacksPath()
	}

	if st, err := os.Stat(bedrockPath); err != nil || !st.IsDir() {
		log.Printf("Skipping import: resource packs path not found: %s", bedrockPath)
		return
	}

	packName := strings.TrimSuffix(filepath.Base(mcpackPath), filepath.Ext(mcpackPath))
	destDir := filepath.Join(bedrockPath, packName)

	extractDir := filepath.Join(a.getTempDir("import"), packName)
	os.MkdirAll(extractDir, os.ModePerm)
	defer os.RemoveAll(a.getTempDir("import"))

	if err := unzip(mcpackPath, extractDir); err != nil {
		log.Printf("Failed to unzip mcpack for import: %v", err)
		return
	}

	os.MkdirAll(destDir, os.ModePerm)
	filepath.Walk(extractDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(extractDir, path)
		if err != nil {
			return nil
		}
		dst := filepath.Join(destDir, rel)
		os.MkdirAll(filepath.Dir(dst), os.ModePerm)
		copyFile(path, dst)
		return nil
	})

	log.Printf("Imported pack to %s", destDir)

	if a.getBoolSetting("deleteMcpack") {
		if err := os.Remove(mcpackPath); err == nil {
			log.Printf("Deleted mcpack after import: %s", mcpackPath)
		}
	}
}

func (a *App) importFromBytes(mcpackBytes []byte, packName string) {
	bedrockPath := a.getStringSetting("resourcePacksPath")
	if bedrockPath == "" {
		bedrockPath = a.getDefaultResourcePacksPath()
	}

	if st, err := os.Stat(bedrockPath); err != nil || !st.IsDir() {
		log.Printf("Skipping import: resource packs path not found: %s", bedrockPath)
		return
	}

	packName = strings.TrimSuffix(packName, filepath.Ext(packName))
	destDir := filepath.Join(bedrockPath, packName)

	extractDir := filepath.Join(a.getTempDir("import"), packName)
	os.MkdirAll(extractDir, os.ModePerm)
	defer os.RemoveAll(a.getTempDir("import"))

	tmpFile := filepath.Join(extractDir, "pack.mcpack")
	if err := os.WriteFile(tmpFile, mcpackBytes, 0644); err != nil {
		log.Printf("Failed to write mcpack to temp for import: %v", err)
		return
	}

	if err := unzip(tmpFile, extractDir); err != nil {
		log.Printf("Failed to unzip mcpack for import: %v", err)
		return
	}

	os.MkdirAll(destDir, os.ModePerm)
	filepath.Walk(extractDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if path == tmpFile {
			return nil
		}
		rel, err := filepath.Rel(extractDir, path)
		if err != nil {
			return nil
		}
		dst := filepath.Join(destDir, rel)
		os.MkdirAll(filepath.Dir(dst), os.ModePerm)
		copyFile(path, dst)
		return nil
	})

	log.Printf("Imported pack to %s", destDir)
}

func (a *App) OpenFolder(dir string) {
	switch runtime.GOOS {
	case "windows":
		exec.Command("explorer", dir).Start()
	case "darwin":
		exec.Command("open", dir).Start()
	default:
		exec.Command("xdg-open", dir).Start()
	}
}

func (a *App) emitProgress(title, message, icon, fileName string, total, completed int) {
	a.logDebug(fmt.Sprintf("emitProgress: title=%q msg=%q icon=%q file=%q total=%d done=%d", title, message, icon, fileName, total, completed))
	wailsRuntime.EventsEmit(a.ctx, "progress", map[string]interface{}{
		"title":     title,
		"message":   message,
		"icon":      icon,
		"total":     total,
		"completed": completed,
		"fileName":  fileName,
	})
}

func (a *App) emitFinished(title, message, icon string) {
	wailsRuntime.EventsEmit(a.ctx, "finished", map[string]interface{}{
		"title":   title,
		"message": message,
		"icon":    icon,
	})
}

func buildMcpackBytes(zipBytes []byte, fileName string, manifestDesc string) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create zip reader: %v", err)
	}

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)

	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			writer.Close()
			return nil, fmt.Errorf("failed to open file in zip: %v", err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			writer.Close()
			return nil, fmt.Errorf("failed to read file data: %v", err)
		}

		newName := file.Name
		newName = strings.Replace(newName, "netherite_layer_1", "netherite_1", 1)
		newName = strings.Replace(newName, "netherite_layer_2", "netherite_2", 1)
		newName = strings.Replace(newName, "totem_of_undying", "totem", 1)

		f, err := writer.Create(newName)
		if err != nil {
			writer.Close()
			return nil, fmt.Errorf("failed to create file in zip: %v", err)
		}

		if manifestDesc != "" && strings.EqualFold(file.Name, "manifest.json") {
			var manifest map[string]interface{}
			if json.Unmarshal(data, &manifest) == nil {
				manifest["description"] = manifestDesc
				if header, ok := manifest["header"].(map[string]interface{}); ok {
					header["description"] = manifestDesc
				}
				if modules, ok := manifest["modules"].([]interface{}); ok && len(modules) > 0 {
					if mod, ok := modules[0].(map[string]interface{}); ok {
						mod["description"] = manifestDesc
					}
				}
				if updated, err := json.MarshalIndent(manifest, "", "  "); err == nil {
					data = updated
				}
			}
		}

		if _, err = f.Write(data); err != nil {
			writer.Close()
			return nil, fmt.Errorf("failed to write to file in zip: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %v", err)
	}

	return buf.Bytes(), nil
}

func writeMcpack(zipBytes []byte, fileName string, outDir string, manifestDesc string) (string, error) {
	data, err := buildMcpackBytes(zipBytes, fileName, manifestDesc)
	if err != nil {
		return "", err
	}

	outFile := filepath.Join(outDir, strings.TrimSuffix(fileName, filepath.Ext(fileName))+".mcpack")
	if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create output directory: %v", err)
	}
	if err := os.WriteFile(outFile, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write output file: %v", err)
	}

	fmt.Println("Porting finished:", outFile)
	return outFile, nil
}

func (a *App) PortPack(bytes []byte, fileName string) (string, error) {
	a.logDebug(fmt.Sprintf("PortPack: called, file=%s, size=%d", fileName, len(bytes)))
	if err := a.acquirePort(); err != nil {
		a.logDebug(fmt.Sprintf("PortPack: acquirePort FAILED: %v", err))
		return "", err
	}
	defer a.releasePort()

	a.logDebug(fmt.Sprintf("PortPack: starting port for %s", fileName))

	lower := strings.ToLower(fileName)
	if !strings.HasSuffix(lower, ".zip") && !strings.HasSuffix(lower, ".rar") {
		return "", fmt.Errorf("please provide a .zip or .rar file")
	}

	if strings.HasSuffix(lower, ".rar") {
		a.logDebug("Delegating to portRarPack")
		return a.portRarPack(bytes, fileName)
	}

	a.emitProgress("Porting", "Porting pack...", "info", fileName, 0, 0)
	a.logDebug("Starting port with swim_porter...")

	skyboxOverride := ""
	out, err := port.Port(bytes, fileName, port.PortOptions{ShowCredits: false, SkyboxOverride: skyboxOverride})
	if err != nil {
		errMsg := err.Error()
		a.logDebug(fmt.Sprintf("port.Port failed: %s", errMsg))
		logError(fmt.Sprintf("port.Port failed for %s: %s", fileName, errMsg))
		log.Println(errMsg)

		var portError *porterror.PortError
		if errors.As(err, &portError) {
			if strings.Contains(errMsg, "pack.mcmeta not found") {
				return "", fmt.Errorf("please provide a valid pack")
			}
			fmt.Println(portError.StackTrace())
		}
		return "", err
	}

	a.logDebug(fmt.Sprintf("port.Port succeeded, output size: %d bytes", len(out)))

	mcpackDesc := a.getStringSetting("manifestDescription")

	if a.getBoolSetting("deleteMcpack") && a.getBoolSetting("autoImport") {
		a.logDebug("deleteMcpack + autoImport: importing directly from memory, no .mcpack on disk")
		mcpackBytes, err := buildMcpackBytes(out, fileName, mcpackDesc)
		if err != nil {
			logError(formatError(fmt.Sprintf("buildMcpackBytes failed for %s", fileName), err))
			return "", fmt.Errorf("failed to build mcpack: %v", err)
		}
		a.importFromBytes(mcpackBytes, fileName)
	} else {
		mcpackDir := a.getOutputDir()
		if a.getBoolSetting("deleteMcpack") {
			mcpackDir = a.getTempDir("mcpack")
			os.MkdirAll(mcpackDir, os.ModePerm)
		}
		outFile, err := writeMcpack(out, fileName, mcpackDir, mcpackDesc)
		if err != nil {
			a.logDebug(fmt.Sprintf("writeMcpack failed: %v", err))
			logError(formatError(fmt.Sprintf("writeMcpack failed for %s", fileName), err))
			return "", fmt.Errorf("failed to write mcpack: %v", err)
		}
		a.logDebug(fmt.Sprintf("writeMcpack succeeded: %s", outFile))
		if a.getBoolSetting("autoImport") {
			a.logDebug("Auto-import enabled, importing to Bedrock...")
			a.importToBedrock(outFile)
		}
	}

	if a.getBoolSetting("autoOpenFolder") {
		a.logDebug("Auto-open enabled, opening output folder...")
		a.OpenFolder(a.getOutputDir())
	}

	a.emitProgress("Done", fileName, "success", fileName, 0, 0)
	a.AddRecentPack(fileName, fileName)
	a.logDebug("PortPack completed successfully")
	return fileName, nil
}

func (a *App) portRarPack(rarBytes []byte, fileName string) (string, error) {
	a.logDebug(fmt.Sprintf("portRarPack called: file=%s, size=%d bytes", fileName, len(rarBytes)))
	a.emitProgress("Porting", "Extracting RAR archive...", "info", fileName, 0, 0)

	tempDir := a.getTempDir("rar_port_temp")
	os.MkdirAll(tempDir, os.ModePerm)
	defer os.RemoveAll(tempDir)

	rarPath := filepath.Join(tempDir, fileName)
	if err := os.WriteFile(rarPath, rarBytes, 0644); err != nil {
		a.logDebug(fmt.Sprintf("Failed to write RAR to temp: %v", err))
		return "", fmt.Errorf("failed to write rar file: %v", err)
	}

	a.logDebug("Extracting RAR archive...")
	extractDir := filepath.Join(tempDir, "extracted")
	if err := unrar(rarPath, extractDir); err != nil {
		a.logDebug(fmt.Sprintf("RAR extraction failed: %v", err))
		logError(formatError(fmt.Sprintf("RAR extraction failed for %s", fileName), err))
		return "", fmt.Errorf("failed to extract rar: %v", err)
	}

	var innerZips []string
	filepath.Walk(extractDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && isZipFile(path) {
			innerZips = append(innerZips, path)
		}
		return nil
	})

	a.logDebug(fmt.Sprintf("Found %d inner zip(s) in RAR", len(innerZips)))

	if len(innerZips) == 0 {
		logError(fmt.Sprintf("RAR archive %s contained no .zip packs", fileName))
		return "", fmt.Errorf("no .zip packs found inside the rar archive")
	}

	var results []string

	for _, zPath := range innerZips {
		a.logDebug(fmt.Sprintf("Porting inner zip: %s", filepath.Base(zPath)))
		zipData, err := os.ReadFile(zPath)
		if err != nil {
			a.logDebug(fmt.Sprintf("Failed to read %s: %v", zPath, err))
			logError(formatError(fmt.Sprintf("read inner zip failed: %s", filepath.Base(zPath)), err))
			continue
		}
		out, err := port.Port(zipData, filepath.Base(zPath), port.PortOptions{ShowCredits: false})
		if err != nil {
			a.logDebug(fmt.Sprintf("Failed to port %s: %v", filepath.Base(zPath), err))
			logError(formatError(fmt.Sprintf("port inner zip failed: %s", filepath.Base(zPath)), err))
			log.Printf("Failed to port %s: %v", filepath.Base(zPath), err)
			continue
		}
		zName := filepath.Base(zPath)
		if a.getBoolSetting("deleteMcpack") && a.getBoolSetting("autoImport") {
			mcpackBytes, err := buildMcpackBytes(out, zName, a.getStringSetting("manifestDescription"))
			if err != nil {
				logError(formatError(fmt.Sprintf("buildMcpackBytes failed: %s", zName), err))
				continue
			}
			a.importFromBytes(mcpackBytes, zName)
			a.AddRecentPack(zName, zName)
			results = append(results, zName)
		} else {
			mcpackDir := a.getOutputDir()
			if a.getBoolSetting("deleteMcpack") {
				mcpackDir = a.getTempDir("mcpack")
				os.MkdirAll(mcpackDir, os.ModePerm)
			}
			outFile, err := writeMcpack(out, zName, mcpackDir, a.getStringSetting("manifestDescription"))
			if err != nil {
				a.logDebug(fmt.Sprintf("Failed to write mcpack for %s: %v", zName, err))
				logError(formatError(fmt.Sprintf("write mcpack failed: %s", zName), err))
				continue
			}
			a.logDebug(fmt.Sprintf("Successfully ported: %s", outFile))
			if a.getBoolSetting("autoImport") {
				a.importToBedrock(outFile)
			}
			a.AddRecentPack(zName, outFile)
			results = append(results, outFile)
		}
	}

	if a.getBoolSetting("autoOpenFolder") && len(results) > 0 {
		a.OpenFolder(a.getOutputDir())
	}

	if len(results) == 0 {
		a.logDebug("No packs were successfully ported from RAR")
		return "", fmt.Errorf("failed to port any packs from the rar archive")
	}

	a.emitProgress("Done", strings.Join(results, ", "), "success", fileName, 0, 0)
	a.logDebug(fmt.Sprintf("portRarPack completed: %d pack(s) ported", len(results)))
	return strings.Join(results, ", "), nil
}

func (a *App) PortPackFromURL(url string) (string, error) {
	url = strings.TrimSpace(url)
	a.logDebug(fmt.Sprintf("PortPackFromURL called: url=%s", url))
	if url == "" {
		return "", fmt.Errorf("no URL provided")
	}

	a.emitProgress("Downloading", "Fetching file from URL...", "info", "", 0, 0)
	a.logDebug("Downloading from URL...")

	data, filename, err := downloadFromURL(url)
	if err != nil {
		a.logDebug(fmt.Sprintf("Download failed: %v", err))
		a.emitProgress("Error", fmt.Sprintf("Failed to download: %v", err), "error", "", 0, 0)
		return "", fmt.Errorf("failed to download: %v", err)
	}

	a.logDebug(fmt.Sprintf("Downloaded: file=%s, size=%d bytes", filename, len(data)))

	lower := strings.ToLower(filename)
	if !isArchive(lower) {
		a.logDebug(fmt.Sprintf("Rejected: downloaded file '%s' is not a supported archive", filename))
		logError(fmt.Sprintf("downloaded file not an archive: %s (from %s)", filename, url))
		a.emitProgress("Error", "Downloaded file is not a .zip or .rar", "error", "", 0, 0)
		return "", fmt.Errorf("downloaded file is not a supported archive")
	}

	a.emitProgress("Porting", "Porting pack...", "info", "", 0, 0)

	result, err := a.PortPack(data, filename)
	if err != nil {
		a.emitProgress("Error", err.Error(), "error", "", 0, 0)
		return "", err
	}

	a.logDebug("PortPackFromURL completed successfully")
	return result, nil
}

func (a *App) CancelPortFolder() {
	if a.cancelFolder != nil {
		a.cancelFolder()
	}
}

func (a *App) PortLocalArchive(data []byte, fileName string) error {
	a.logDebug(fmt.Sprintf("PortLocalArchive: called, file=%s, size=%d", fileName, len(data)))
	if err := a.acquirePort(); err != nil {
		a.logDebug(fmt.Sprintf("PortLocalArchive: acquirePort FAILED: %v", err))
		a.emitProgress("Error", err.Error(), "error", "", 0, 0)
		return err
	}
	defer a.releasePort()

	if len(data) == 0 {
		a.emitProgress("Error", "No file data provided.", "error", "", 0, 0)
		return fmt.Errorf("no file data provided")
	}

	lower := strings.ToLower(fileName)
	if !isArchive(lower) {
		a.emitProgress("Error", "Please provide a .zip or .rar file.", "error", "", 0, 0)
		return fmt.Errorf("not a supported archive")
	}

	ctx, cancel := context.WithCancel(a.ctx)
	a.cancelFolder = cancel
	defer func() {
		a.cancelFolder = nil
		cancel()
	}()

	tempDir := a.getTempDir("folder_port_temp")
	os.MkdirAll(tempDir, os.ModePerm)
	defer os.RemoveAll(tempDir)

	outputDir := a.getOutputDir()
	deleteOriginals := a.getBoolSetting("deleteOriginals")

	var javaDir, bedrockDir string
	if deleteOriginals {
		bedrockDir = outputDir
	} else {
		javaDir = filepath.Join(outputDir, "Java")
		bedrockDir = filepath.Join(outputDir, "Bedrock")
		os.MkdirAll(javaDir, os.ModePerm)
		os.MkdirAll(bedrockDir, os.ModePerm)
	}

	archivePath := filepath.Join(tempDir, fileName)
	if err := os.WriteFile(archivePath, data, 0644); err != nil {
		logError(formatError(fmt.Sprintf("write archive to temp failed: %s", fileName), err))
		a.emitProgress("Error", fmt.Sprintf("Failed to write file: %v", err), "error", "", 0, 0)
		return err
	}

	a.emitProgress("Extracting", "Extracting archive...", "info", "", 0, 0)

	extractDir := filepath.Join(tempDir, "extracted")
	if err := extractArchive(archivePath, extractDir); err != nil {
		logError(formatError(fmt.Sprintf("extract archive failed: %s", fileName), err))
		a.emitProgress("Error", fmt.Sprintf("Failed to extract: %v", err), "error", "", 0, 0)
		return err
	}

	type fileCandidate struct {
		Name    string
		ZipPath string
	}

	var candidates []fileCandidate
	filepath.Walk(extractDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if isArchive(path) {
			candidates = append(candidates, fileCandidate{Name: filepath.Base(path), ZipPath: path})
		}
		return nil
	})

	if len(candidates) == 0 {
		logError(fmt.Sprintf("no pack archives found inside: %s", fileName))
		a.emitProgress("Error", "No pack archives found inside the file.", "error", "", 0, 0)
		return fmt.Errorf("no pack archives found")
	}

	totalFiles := len(candidates)
	successCount := 0
	failCount := 0

	a.emitProgress("Found", fmt.Sprintf("Found %d pack(s). Porting...", totalFiles), "info", "", totalFiles, 0)

	for i, c := range candidates {
		if ctx.Err() != nil {
			a.emitProgress("Cancelled", fmt.Sprintf("Cancelled after porting %d/%d pack(s).", successCount, totalFiles), "warning", "", totalFiles, i)
			return fmt.Errorf("cancelled")
		}

		a.emitProgress("Porting", fmt.Sprintf("[%d/%d] %s", i+1, totalFiles, c.Name), "info", c.Name, totalFiles, i)

		rawData, err := os.ReadFile(c.ZipPath)
		if err != nil {
			logError(formatError(fmt.Sprintf("read pack failed: %s", c.Name), err))
			failCount++
			continue
		}

		out, err := port.Port(rawData, c.Name, port.PortOptions{ShowCredits: false})
		if err != nil {
			logError(formatError(fmt.Sprintf("port failed: %s", c.Name), err))
			log.Printf("Failed to port %s: %v", c.Name, err)
			failCount++
			continue
		}

		if mcpackPath, err := writeMcpack(out, c.Name, bedrockDir, a.getStringSetting("manifestDescription")); err != nil {
			logError(formatError(fmt.Sprintf("write mcpack failed: %s", c.Name), err))
			failCount++
			continue
		} else {
			a.AddRecentPack(c.Name, mcpackPath)
		}

		if !deleteOriginals {
			javaDst := filepath.Join(javaDir, c.Name)
			copyFile(c.ZipPath, javaDst)
		}

		if a.getBoolSetting("autoImport") {
			mcpackPath := filepath.Join(bedrockDir, strings.TrimSuffix(c.Name, filepath.Ext(c.Name))+".mcpack")
			a.importToBedrock(mcpackPath)
		}

		successCount++
	}

	if failCount > 0 && successCount > 0 {
		a.emitProgress("Partial", fmt.Sprintf("Ported %d pack(s), %d failed.", successCount, failCount), "warning", "", totalFiles, totalFiles)
	} else if failCount > 0 {
		a.emitProgress("Error", fmt.Sprintf("All %d pack(s) failed to port.", failCount), "error", "", totalFiles, totalFiles)
	} else {
		if deleteOriginals {
			a.emitProgress("Done", fmt.Sprintf("Ported %d pack(s) to %s.", successCount, outputDir), "success", "", totalFiles, totalFiles)
		} else {
			a.emitProgress("Done", fmt.Sprintf("Ported %d pack(s). Originals in Java/, ported in Bedrock/.", successCount), "success", "", totalFiles, totalFiles)
		}
	}

	if a.getBoolSetting("autoOpenFolder") && successCount > 0 {
		a.OpenFolder(outputDir)
	}

	return nil
}

func (a *App) PortFolder(url string) error {
	a.logDebug(fmt.Sprintf("PortFolder: called, url=%s", url))
	if err := a.acquirePort(); err != nil {
		a.logDebug(fmt.Sprintf("PortFolder: acquirePort FAILED: %v", err))
		a.emitProgress("Error", err.Error(), "error", "", 0, 0)
		return err
	}
	defer a.releasePort()

	url = strings.TrimSpace(url)
	if url == "" {
		a.emitProgress("Error", "No URL provided.", "error", "", 0, 0)
		return fmt.Errorf("no URL provided")
	}

	ctx, cancel := context.WithCancel(a.ctx)
	a.cancelFolder = cancel
	defer func() {
		a.cancelFolder = nil
		cancel()
	}()

	client := createHTTPClient()
	isFolder := isMediaFireURL(url) && strings.Contains(url, "/folder/")

	totalFiles := 0
	successCount := 0
	failCount := 0

	outputDir := a.getOutputDir()
	deleteOriginals := a.getBoolSetting("deleteOriginals")

	var javaDir, bedrockDir string
	if deleteOriginals {
		bedrockDir = outputDir
	} else {
		javaDir = filepath.Join(outputDir, "Java")
		bedrockDir = filepath.Join(outputDir, "Bedrock")
		os.MkdirAll(javaDir, os.ModePerm)
		os.MkdirAll(bedrockDir, os.ModePerm)
	}

	if isFolder {
		a.emitProgress("Starting", "Reading folder page...", "info", "", 0, 0)

		fileLinks, err := extractMediaFireFolderLinks(client, url)
		if err != nil {
			logError(formatError(fmt.Sprintf("read folder failed: %s", url), err))
			a.emitProgress("Error", fmt.Sprintf("Failed to read folder: %v", err), "error", "", 0, 0)
			return err
		}

		if len(fileLinks) == 0 {
			logError(fmt.Sprintf("no files found in MediaFire folder: %s", url))
			a.emitProgress("Error", "No files found in the MediaFire folder.", "error", "", 0, 0)
			return fmt.Errorf("no files found in folder")
		}

		totalFiles = len(fileLinks)
		a.emitProgress("Found", fmt.Sprintf("Found %d file(s). Downloading & porting each one...", totalFiles), "info", "", totalFiles, 0)

		for i, fl := range fileLinks {
			if ctx.Err() != nil {
				a.emitProgress("Cancelled", fmt.Sprintf("Cancelled after porting %d/%d pack(s). Already-ported packs are saved.", successCount, totalFiles), "warning", "", totalFiles, i)
				return fmt.Errorf("cancelled")
			}

			a.emitProgress("Downloading", fmt.Sprintf("[%d/%d] %s", i+1, totalFiles, fl.Name), "info", fl.Name, totalFiles, i)

			data, _, err := downloadDirect(client, fl.URL)
			if err != nil {
				logError(formatError(fmt.Sprintf("[%d/%d] download failed: %s", i+1, totalFiles, fl.Name), err))
				log.Printf("Failed to download %s: %v", fl.Name, err)
				a.emitProgress("Skipping", fmt.Sprintf("[%d/%d] %s — download failed", i+1, totalFiles, fl.Name), "fail", fl.Name, totalFiles, i+1)
				failCount++
				continue
			}

			packOk := a.processPack(ctx, fl.Name, data, totalFiles, i, bedrockDir, javaDir)
			if packOk {
				successCount++
			} else {
				failCount++
			}
		}
	} else {
		a.emitProgress("Starting", "Downloading file...", "info", "", 0, 0)

		data, filename, err := downloadFromURL(url)
		if err != nil {
			a.emitProgress("Error", fmt.Sprintf("Failed to download: %v", err), "error", "", 0, 0)
			return err
		}

		totalFiles = 1
		a.emitProgress("Found", "Downloaded. Porting...", "info", "", 1, 0)

		packOk := a.processPack(ctx, filename, data, 1, 0, bedrockDir, javaDir)
		if packOk {
			successCount = 1
		} else {
			failCount = 1
		}
	}

	if failCount > 0 && successCount > 0 {
		a.emitProgress("Partial", fmt.Sprintf("Ported %d pack(s), %d failed.", successCount, failCount), "warning", "", totalFiles, totalFiles)
	} else if failCount > 0 {
		a.emitProgress("Error", fmt.Sprintf("All %d pack(s) failed to port.", failCount), "error", "", totalFiles, totalFiles)
	} else {
		if deleteOriginals {
			a.emitProgress("Done", fmt.Sprintf("Ported %d pack(s) to %s.", successCount, outputDir), "success", "", totalFiles, totalFiles)
		} else {
			a.emitProgress("Done", fmt.Sprintf("Ported %d pack(s) to Bedrock/. Originals in Java/.", successCount), "success", "", totalFiles, totalFiles)
		}
	}

	if a.getBoolSetting("autoOpenFolder") && successCount > 0 {
		a.OpenFolder(outputDir)
	}

	return nil
}

func (a *App) processPack(ctx context.Context, name string, data []byte, totalFiles, index int, bedrockDir, javaDir string) bool {
	if ctx.Err() != nil {
		return false
	}

	a.emitProgress("Porting", fmt.Sprintf("[%d/%d] %s", index+1, totalFiles, name), "info", name, totalFiles, index)

	type fileCandidate struct {
		Name    string
		ZipPath string
		IsZip   bool
	}

	tempDir := a.getTempDir("process_pack_temp")
	os.MkdirAll(tempDir, os.ModePerm)
	defer os.RemoveAll(tempDir)

	var candidates []fileCandidate

	lower := strings.ToLower(name)
	if !isArchive(lower) {
		logError(fmt.Sprintf("skipping %s: not a .zip or .rar file", name))
		log.Printf("Skipping %s: not a .zip or .rar file", name)
		a.emitProgress("Skipping", fmt.Sprintf("[%d/%d] %s — not a .zip or .rar", index+1, totalFiles, name), "fail", name, totalFiles, index+1)
		return false
	}

	zipPath := filepath.Join(tempDir, name)
	if err := os.WriteFile(zipPath, data, 0644); err != nil {
		logError(formatError(fmt.Sprintf("write temp file failed: %s", name), err))
		log.Printf("Failed to write %s: %v", name, err)
		a.emitProgress("Skipping", fmt.Sprintf("[%d/%d] %s — write failed", index+1, totalFiles, name), "fail", name, totalFiles, index+1)
		return false
	}

	extractDir := filepath.Join(tempDir, strings.TrimSuffix(name, filepath.Ext(name)))
	if err := extractArchive(zipPath, extractDir); err != nil {
		candidates = append(candidates, fileCandidate{Name: name, ZipPath: zipPath, IsZip: true})
	} else {
		var innerArchives []string
		filepath.Walk(extractDir, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && isArchive(path) {
				innerArchives = append(innerArchives, path)
			}
			return nil
		})

		if len(innerArchives) > 0 {
			for _, iz := range innerArchives {
				candidates = append(candidates, fileCandidate{Name: filepath.Base(iz), ZipPath: iz, IsZip: true})
			}
		} else {
			candidates = append(candidates, fileCandidate{Name: name, ZipPath: zipPath, IsZip: true})
		}
	}

	if len(candidates) == 0 {
		a.emitProgress("Skipping", fmt.Sprintf("[%d/%d] %s — no .zip packs found", index+1, totalFiles, name), "fail", name, totalFiles, index+1)
		return false
	}

	packOk := false
	for _, c := range candidates {
		if ctx.Err() != nil {
			return false
		}

		rawData, err := os.ReadFile(c.ZipPath)
		if err != nil {
			log.Printf("Failed to read %s: %v", c.Name, err)
			a.emitProgress("Progress", c.Name, "fail", c.Name, totalFiles, index+1)
			continue
		}

		out, err := port.Port(rawData, c.Name, port.PortOptions{ShowCredits: false})
		if err != nil {
			logError(formatError(fmt.Sprintf("port failed: %s", c.Name), err))
			log.Printf("Failed to port %s: %v", c.Name, err)
			a.emitProgress("Progress", c.Name, "fail", c.Name, totalFiles, index+1)
			continue
		}

		javaDst := filepath.Join(javaDir, c.Name)
		if !a.getBoolSetting("deleteOriginals") {
			if err := copyFile(c.ZipPath, javaDst); err != nil {
				log.Printf("Failed to copy %s to Java/: %v", c.Name, err)
			}
		}

		if a.getBoolSetting("deleteMcpack") && a.getBoolSetting("autoImport") {
			mcpackBytes, err := buildMcpackBytes(out, c.Name, a.getStringSetting("manifestDescription"))
			if err != nil {
				logError(formatError(fmt.Sprintf("buildMcpackBytes failed: %s", c.Name), err))
				a.emitProgress("Progress", c.Name, "fail", c.Name, totalFiles, index+1)
				continue
			}
			a.importFromBytes(mcpackBytes, c.Name)
			a.AddRecentPack(c.Name, c.Name)
		} else {
			mcpackDir := bedrockDir
			if a.getBoolSetting("deleteMcpack") {
				mcpackDir = a.getTempDir("mcpack")
				os.MkdirAll(mcpackDir, os.ModePerm)
			}
			reportOut, reportErr := writeMcpack(out, c.Name, mcpackDir, a.getStringSetting("manifestDescription"))
			if reportErr != nil {
				logError(formatError(fmt.Sprintf("write mcpack failed: %s", c.Name), reportErr))
				log.Printf("Failed to write mcpack %s: %v", c.Name, reportErr)
				a.emitProgress("Progress", c.Name, "fail", c.Name, totalFiles, index+1)
				continue
			}
			if a.getBoolSetting("autoImport") {
				a.importToBedrock(reportOut)
			}
			a.AddRecentPack(c.Name, reportOut)
		}

		a.emitProgress("Progress", c.Name, "done", c.Name, totalFiles, index+1)
		packOk = true
	}

	return packOk
}

type FolderImage struct {
	Filename string `json:"filename"`
	DataURI  string `json:"dataURI"`
	RelPath  string `json:"relPath"`
	Size     int64  `json:"size"`
}

type CheckResult struct {
	Folders  []string `json:"folders"`
	Valid    bool     `json:"valid"`
	ErrorMsg string   `json:"errorMsg,omitempty"`
}

func (a *App) CheckPack(bytes []byte) (*CheckResult, error) {
	tmpFile, err := os.CreateTemp("", "srm-check-*.zip")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Write(bytes)
	tmpFile.Close()
	defer os.Remove(tmpPath)

	checkDir := a.getTempDir("check_unzip")
	os.RemoveAll(checkDir)
	if err := unzip(tmpPath, checkDir); err != nil {
		return nil, fmt.Errorf("failed to unzip: %v", err)
	}

	if _, err := os.Stat(filepath.Join(checkDir, "manifest.json")); os.IsNotExist(err) {
		os.RemoveAll(checkDir)
		return &CheckResult{Valid: false, ErrorMsg: "Please provide a valid pack."}, nil
	}

	texturesPath := filepath.Join(checkDir, "textures")
	var folders []string
	err = filepath.WalkDir(texturesPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != texturesPath {
			rel, err := filepath.Rel(checkDir, path)
			if err != nil {
				return err
			}
			if strings.Contains(rel, "entity") {
				return nil
			}
			folders = append(folders, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk dir error: %v", err)
	}

	return &CheckResult{Folders: folders, Valid: true}, nil
}

func (a *App) GetImages(folder string) ([]FolderImage, error) {
	re := regexp.MustCompile(`\\\s`)
	folder = re.ReplaceAllString(folder, `\`)

	fullPath := filepath.Join(a.getTempDir("check_unzip"), folder)

	var images []FolderImage

	err := filepath.Walk(fullPath, func(path string, info fs.FileInfo, err error) error {
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

			relPath, err := filepath.Rel(a.getTempDir("check_unzip"), path)
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

type SaveImageRequest struct {
	ImageName string `json:"imageName"`
	ImagePath string `json:"imagePath"`
	RelPath   string `json:"relPath"`
	ImageData string `json:"imageData"`
	Done      bool   `json:"done"`
}

func (a *App) SaveImage(msg SaveImageRequest) (string, error) {
	if msg.Done {
		return "done", nil
	}

	imageData, err := base64.StdEncoding.DecodeString(msg.ImageData)
	if err != nil {
		return "", fmt.Errorf("error decoding base64 image: %v", err)
	}

	var savePath string
	if msg.RelPath != "" {
		savePath = filepath.Join(a.getTempDir("check_unzip"), filepath.FromSlash(msg.RelPath))
	} else {
		var foundPath string
		filepath.Walk(a.getTempDir("check_unzip"), func(path string, info fs.FileInfo, err error) error {
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
			savePath = filepath.Join(a.getTempDir("check_unzip"), "textures", msg.ImageName)
		}
	}

	os.MkdirAll(filepath.Dir(savePath), os.ModePerm)

	err = os.WriteFile(savePath, imageData, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to save image: %v", err)
	}

	return "success", nil
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

func (a *App) ExportPack(name string) (string, error) {
	baseName := "exported_pack"
	if name != "" {
		baseName = strings.TrimSuffix(name, filepath.Ext(name))
	}

	exportName := baseName + "-recolored.mcpack"
	zipName := baseName + "-recolored.zip"

	if err := createZipFromFolder(a.getTempDir("check_unzip"), zipName); err != nil {
		return "", fmt.Errorf("failed to create export: %v", err)
	}

	if err := os.Rename(zipName, exportName); err != nil {
		return "", fmt.Errorf("failed to finalize export: %v", err)
	}

	return fmt.Sprintf("Exported pack to %s", exportName), nil
}

func (a *App) DeleteTemp() {
	os.RemoveAll(a.getTempDir("check_unzip"))
}

func (a *App) OpenDiscordLink() {
	exec.Command("cmd", "/c", "start", "https://discord.gg/AA8MSTDjB").Start()
}

func (a *App) OpenMewDataDir() {
	a.OpenFolder(filepath.Join(os.Getenv("LOCALAPPDATA"), "mew"))
}

func (a *App) OpenDonationLink() {
	exec.Command("cmd", "/c", "start", "https://www.paypal.me/spotters1").Start()
}

func (a *App) OpenDownloadLink(url string) {
	exec.Command("cmd", "/c", "start", url).Start()
}

func (a *App) CheckForUpdate() UpdateInfo {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", githubReleasesURL, nil)
	if err != nil {
		return UpdateInfo{NeedsUpdate: false}
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return UpdateInfo{NeedsUpdate: false}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return UpdateInfo{NeedsUpdate: false}
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return UpdateInfo{NeedsUpdate: false}
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	if latestVersion == "" || latestVersion == currentVersion {
		return UpdateInfo{NeedsUpdate: false}
	}

	downloadURL := ""
	for _, asset := range release.Assets {
		if strings.HasSuffix(strings.ToLower(asset.Name), ".exe") {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	return UpdateInfo{
		NeedsUpdate:   true,
		LatestVersion: latestVersion,
		DownloadURL:   downloadURL,
	}
}

func (a *App) GetVersion() string {
	return currentVersion
}

type ChangelogEntry struct {
	Tag  string `json:"tag"`
	URL  string `json:"url"`
	Date string `json:"date"`
	Body string `json:"body"`
}

func (a *App) GetChangelog() []ChangelogEntry {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", "https://api.github.com/repos/KyarottoOwO/mew/releases", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var releases []struct {
		TagName   string `json:"tag_name"`
		HTMLURL   string `json:"html_url"`
		Published string `json:"published_at"`
		Body      string `json:"body"`
	}
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil
	}

	var entries []ChangelogEntry
	for _, r := range releases {
		date := ""
		if t, err := time.Parse(time.RFC3339, r.Published); err == nil {
			date = t.Format("Jan 02, 2006")
		}
		entries = append(entries, ChangelogEntry{
			Tag:  strings.TrimPrefix(r.TagName, "v"),
			URL:  r.HTMLURL,
			Date: date,
			Body: r.Body,
		})
	}
	return entries
}

func (a *App) getRecentPacksPath() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
	}
	dir := filepath.Join(localAppData, "mew", "Recent_Packs")
	os.MkdirAll(dir, os.ModePerm)
	newPath := filepath.Join(dir, "recent_packs.json")

	exePath, err := os.Executable()
	if err == nil {
		oldPath := filepath.Join(filepath.Dir(exePath), "recent_packs.json")
		if _, statErr := os.Stat(oldPath); statErr == nil {
			if _, statErr := os.Stat(newPath); os.IsNotExist(statErr) {
				a.logDebug(fmt.Sprintf("Migrating recent_packs from %s to %s", oldPath, newPath))
				data, readErr := os.ReadFile(oldPath)
				if readErr == nil {
					os.WriteFile(newPath, data, 0644)
				}
				os.Remove(oldPath)
			}
		}
	}

	return newPath
}

func (a *App) GetRecentPacks() []RecentPack {
	return a.loadRecentPacks(5)
}

func (a *App) GetAllRecentPacks() []RecentPack {
	return a.loadRecentPacks(50)
}

func (a *App) loadRecentPacks(max int) []RecentPack {
	data, err := os.ReadFile(a.getRecentPacksPath())
	if err != nil {
		return []RecentPack{}
	}

	var packs []RecentPack
	if err := json.Unmarshal(data, &packs); err != nil {
		return []RecentPack{}
	}

	if len(packs) > max {
		packs = packs[:max]
	}
	return packs
}

func (a *App) AddRecentPack(name, path string) {
	data, err := os.ReadFile(a.getRecentPacksPath())
	var packs []RecentPack
	if err == nil {
		json.Unmarshal(data, &packs)
	}

	newPack := RecentPack{
		Name:      name,
		Path:      path,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	}

	packs = append([]RecentPack{newPack}, packs...)
	if len(packs) > 10 {
		packs = packs[:10]
	}

	jsonData, err := json.MarshalIndent(packs, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(a.getRecentPacksPath(), jsonData, 0644)
	wailsRuntime.EventsEmit(a.ctx, "recent-packs-changed")
}

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kyarottoOwO/mew/internal/mediafire"
	"github.com/swim-services/swim_porter/port"
	"github.com/swim-services/swim_porter/porterror"
)

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

func cleanPackName(name string) string {
	base := strings.TrimSpace(name)
	for {
		ext := filepath.Ext(base)
		switch strings.ToLower(ext) {
		case ".zip", ".rar":
			base = strings.TrimSuffix(base, ext)
		default:
			return base
		}
	}
}

func (a *App) buildMcpackBytes(zipBytes []byte, fileName string, manifestDesc string) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create zip reader: %v", err)
	}

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)

	var promo map[string][]byte
	if a.getBoolSetting("addPromoTexts") {
		promo, err = promoFiles()
		if err != nil {
			return nil, fmt.Errorf("failed to load promo files: %v", err)
		}
	}

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

		if strings.EqualFold(file.Name, "manifest.json") {
			var manifest map[string]interface{}
			if json.Unmarshal(data, &manifest) == nil {
				changed := false
				if manifestDesc != "" {
					manifest["description"] = manifestDesc
					changed = true
				}
				if header, ok := manifest["header"].(map[string]interface{}); ok {
					if name, ok := header["name"].(string); ok && name != "" {
						if cleaned := cleanPackName(name); cleaned != "" && cleaned != name {
							header["name"] = cleaned
							changed = true
						}
					}
					if manifestDesc != "" {
						header["description"] = manifestDesc
						changed = true
					}
				}
				if modules, ok := manifest["modules"].([]interface{}); ok {
					for _, m := range modules {
						mod, ok := m.(map[string]interface{})
						if !ok {
							continue
						}
						if name, ok := mod["name"].(string); ok && name != "" {
							if cleaned := cleanPackName(name); cleaned != "" && cleaned != name {
								mod["name"] = cleaned
								changed = true
							}
						}
						if manifestDesc != "" {
							mod["description"] = manifestDesc
							changed = true
						}
					}
				}
				if changed {
					if updated, err := json.MarshalIndent(manifest, "", "  "); err == nil {
						data = updated
					}
				}
			}
		}

		if _, err = f.Write(data); err != nil {
			writer.Close()
			return nil, fmt.Errorf("failed to write to file in zip: %v", err)
		}
	}

	if promo != nil {
		names := make([]string, 0, len(promo))
		for name := range promo {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			f, err := writer.Create(name)
			if err != nil {
				writer.Close()
				return nil, fmt.Errorf("failed to create promo file %s in zip: %v", name, err)
			}
			if _, err := f.Write(promo[name]); err != nil {
				writer.Close()
				return nil, fmt.Errorf("failed to write promo file %s to zip: %v", name, err)
			}
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %v", err)
	}

	return buf.Bytes(), nil
}

func (a *App) writeMcpack(zipBytes []byte, fileName string, outDir string, manifestDesc string) (string, error) {
	data, err := a.buildMcpackBytes(zipBytes, fileName, manifestDesc)
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

func (a *App) ExportInstalledPack(packName string) (string, error) {
	a.logDebug(fmt.Sprintf("ExportInstalledPack: called, pack=%s", packName))
	if packName == "" || strings.Contains(packName, "..") || strings.ContainsAny(packName, `\/`) {
		return "", fmt.Errorf("invalid pack name")
	}

	packDir := a.getPackDir(packName)
	st, err := os.Stat(packDir)
	if err != nil || !st.IsDir() {
		return "", fmt.Errorf("pack not found: %s", packName)
	}

	outDir := a.getOutputDir()
	if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create output directory: %v", err)
	}

	outFile := filepath.Join(outDir, packName+".mcpack")
	if err := zipDirToFile(packDir, outFile); err != nil {
		return "", fmt.Errorf("failed to export pack: %v", err)
	}

	a.logDebug(fmt.Sprintf("ExportInstalledPack: wrote %s", outFile))
	a.AddRecentPack(packName, outFile)
	return outFile, nil
}

// ExportInstalledPacks exports multiple installed packs to the output directory
// and returns the list of written .mcpack file paths.
func (a *App) ExportInstalledPacks(packNames []string) ([]string, error) {
	a.logDebug(fmt.Sprintf("ExportInstalledPacks: called, packs=%d", len(packNames)))
	if len(packNames) == 0 {
		return nil, fmt.Errorf("no packs selected")
	}

	outDir := a.getOutputDir()
	if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %v", err)
	}

	var written []string
	for _, packName := range packNames {
		if packName == "" || strings.Contains(packName, "..") || strings.ContainsAny(packName, `\/`) {
			return nil, fmt.Errorf("invalid pack name: %s", packName)
		}
		packDir := a.getPackDir(packName)
		st, err := os.Stat(packDir)
		if err != nil || !st.IsDir() {
			return nil, fmt.Errorf("pack not found: %s", packName)
		}
		outFile := filepath.Join(outDir, packName+".mcpack")
		if err := zipDirToFile(packDir, outFile); err != nil {
			return nil, fmt.Errorf("failed to export pack %s: %v", packName, err)
		}
		a.logDebug(fmt.Sprintf("ExportInstalledPacks: wrote %s", outFile))
		a.AddRecentPack(packName, outFile)
		written = append(written, outFile)
	}

	return written, nil
}

// GetExportOutputDir returns the directory where exported packs are written.
func (a *App) GetExportOutputDir() string {
	return a.getOutputDir()
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

	withSubpacks, err := a.maybeAddSkySubpacks(bytes, out)
	if err != nil {
		a.logDebug(fmt.Sprintf("maybeAddSkySubpacks failed: %v", err))
	}
	out = withSubpacks

	mcpackDesc := a.getStringSetting("manifestDescription")

	if a.getBoolSetting("deleteMcpack") && a.getBoolSetting("autoImport") {
		a.logDebug("deleteMcpack + autoImport: importing directly from memory, no .mcpack on disk")
		mcpackBytes, err := a.buildMcpackBytes(out, fileName, mcpackDesc)
		if err != nil {
			logError(formatError(fmt.Sprintf("buildMcpackBytes failed for %s", fileName), err))
			return "", fmt.Errorf("failed to build mcpack: %v", err)
		}
		a.importFromBytes(mcpackBytes, fileName)
	} else {
		mcpackDir := a.getOutputDir()
		if a.getBoolSetting("deleteMcpack") && a.getBoolSetting("autoImport") {
			mcpackDir = a.getTempDir("mcpack")
			os.MkdirAll(mcpackDir, os.ModePerm)
		}
		outFile, err := a.writeMcpack(out, fileName, mcpackDir, mcpackDesc)
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
		if withSub, subErr := a.maybeAddSkySubpacks(zipData, out); subErr == nil {
			out = withSub
		}
		zName := filepath.Base(zPath)
		if a.getBoolSetting("deleteMcpack") && a.getBoolSetting("autoImport") {
			mcpackBytes, err := a.buildMcpackBytes(out, zName, a.getStringSetting("manifestDescription"))
			if err != nil {
				logError(formatError(fmt.Sprintf("buildMcpackBytes failed: %s", zName), err))
				continue
			}
			a.importFromBytes(mcpackBytes, zName)
			a.AddRecentPack(zName, zName)
			results = append(results, zName)
		} else {
			mcpackDir := a.getOutputDir()
			if a.getBoolSetting("deleteMcpack") && a.getBoolSetting("autoImport") {
				mcpackDir = a.getTempDir("mcpack")
				os.MkdirAll(mcpackDir, os.ModePerm)
			}
			outFile, err := a.writeMcpack(out, zName, mcpackDir, a.getStringSetting("manifestDescription"))
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

	data, filename, err := mediafire.DownloadFromURL(url)
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
		if withSub, subErr := a.maybeAddSkySubpacks(rawData, out); subErr == nil {
			out = withSub
		}

		if mcpackPath, err := a.writeMcpack(out, c.Name, bedrockDir, a.getStringSetting("manifestDescription")); err != nil {
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

	client := mediafire.CreateHTTPClient()
	isFolder := mediafire.IsMediaFireURL(url) && strings.Contains(url, "/folder/")

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

		fileLinks, err := mediafire.ExtractMediaFireFolderLinks(client, url)
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

			data, _, err := mediafire.DownloadDirect(client, fl.URL)
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

		data, filename, err := mediafire.DownloadFromURL(url)
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
		if withSub, subErr := a.maybeAddSkySubpacks(rawData, out); subErr == nil {
			out = withSub
		}

		javaDst := filepath.Join(javaDir, c.Name)
		if !a.getBoolSetting("deleteOriginals") {
			if err := copyFile(c.ZipPath, javaDst); err != nil {
				log.Printf("Failed to copy %s to Java/: %v", c.Name, err)
			}
		}

		if a.getBoolSetting("deleteMcpack") && a.getBoolSetting("autoImport") {
			mcpackBytes, err := a.buildMcpackBytes(out, c.Name, a.getStringSetting("manifestDescription"))
			if err != nil {
				logError(formatError(fmt.Sprintf("buildMcpackBytes failed: %s", c.Name), err))
				a.emitProgress("Progress", c.Name, "fail", c.Name, totalFiles, index+1)
				continue
			}
			a.importFromBytes(mcpackBytes, c.Name)
			a.AddRecentPack(c.Name, c.Name)
		} else {
			mcpackDir := bedrockDir
			if a.getBoolSetting("deleteMcpack") && a.getBoolSetting("autoImport") {
				mcpackDir = a.getTempDir("mcpack")
				os.MkdirAll(mcpackDir, os.ModePerm)
			}
			reportOut, reportErr := a.writeMcpack(out, c.Name, mcpackDir, a.getStringSetting("manifestDescription"))
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

func (a *App) importToBedrock(mcpackPath string) {
	bedrockPath := a.getResourcePacksPath()

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
	bedrockPath := a.getResourcePacksPath()

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

func (a *App) CheckPack(bytes []byte, packName string) (*CheckResult, error) {
	a.logDebug(fmt.Sprintf("CheckPack: called, size=%d", len(bytes)))

	tmpFile, err := os.CreateTemp("", "srm-check-*.zip")
	if err != nil {
		a.logDebug(fmt.Sprintf("CheckPack: temp file FAILED: %v", err))
		return nil, fmt.Errorf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Write(bytes)
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Every upload gets its own working directory so no other tool can wipe it.
	sessionID, err := a.createSessionDir()
	if err != nil {
		a.logDebug(fmt.Sprintf("CheckPack: create session FAILED: %v", err))
		return nil, fmt.Errorf("failed to create session: %v", err)
	}
	checkDir, err := a.sessionDir(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to open session: %v", err)
	}
	discard := true
	defer func() {
		if discard {
			a.DeleteSession(sessionID)
		}
	}()

	if err := unzip(tmpPath, checkDir); err != nil {
		a.logDebug(fmt.Sprintf("CheckPack: unzip FAILED: %v", err))
		return nil, fmt.Errorf("failed to unzip: %v", err)
	}
	a.logDebug(fmt.Sprintf("CheckPack: unzipped to %s (session %s)", checkDir, sessionID))

	rootDir, findErr := findManifestRoot(checkDir)
	if findErr != nil {
		a.logDebug(fmt.Sprintf("CheckPack: findManifestRoot FAILED: %v", findErr))
		return nil, fmt.Errorf("failed to find manifest: %v", findErr)
	}
	if rootDir == "" {
		a.logDebug("CheckPack: no manifest.json found, invalid pack")
		return &CheckResult{Valid: false, ErrorMsg: "Please provide a valid pack."}, nil
	}
	if rootDir != checkDir {
		a.logDebug(fmt.Sprintf("CheckPack: manifest found inside wrapper folder %q", rootDir))
	}

	texturesPath := filepath.Join(rootDir, "textures")
	var folders []string
	if _, texStatErr := os.Stat(texturesPath); texStatErr != nil {
		a.logDebug(fmt.Sprintf("CheckPack: no textures dir at %s, folders empty", texturesPath))
	} else {
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
			a.logDebug(fmt.Sprintf("CheckPack: walk textures FAILED: %v", err))
			return nil, fmt.Errorf("walk dir error: %v", err)
		}
	}

	a.setSessionMeta(sessionID, packName, folders)
	discard = false
	a.logDebug(fmt.Sprintf("CheckPack: valid, session %s, %d folder(s): %v", sessionID, len(folders), folders))
	return &CheckResult{SessionId: sessionID, Folders: folders, Valid: true}, nil
}

func findManifestRoot(checkDir string) (string, error) {
	if _, err := os.Stat(filepath.Join(checkDir, "manifest.json")); err == nil {
		return checkDir, nil
	}

	var root string
	walkErr := filepath.WalkDir(checkDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && path != checkDir {
			rel, rerr := filepath.Rel(checkDir, path)
			if rerr != nil {
				return nil
			}
			if strings.Count(filepath.ToSlash(rel), "/") > 5 {
				return filepath.SkipDir
			}
		}
		if !d.IsDir() && strings.EqualFold(d.Name(), "manifest.json") {
			root = filepath.Dir(path)
			return filepath.SkipAll
		}
		return nil
	})
	if walkErr != nil {
		return "", walkErr
	}
	return root, nil
}

// sanitizeExportBaseName turns an uploaded file name into a safe base name:
// no path separators, no characters Windows rejects, no repeated "-recolored".
func sanitizeExportBaseName(name string) string {
	name = strings.TrimSpace(name)
	if ext := filepath.Ext(name); ext != "" {
		name = strings.TrimSuffix(name, ext)
	}
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 0x20 {
			return -1
		}
		return r
	}, name)
	for {
		trimmed := strings.TrimSpace(strings.TrimSuffix(name, "-recolored"))
		if trimmed == name {
			break
		}
		name = trimmed
	}
	name = strings.Trim(name, " .")
	if name == "" {
		return "exported_pack"
	}
	return name
}

func (a *App) ExportPack(sessionId, name string) (string, error) {
	srcDir, err := a.sessionDir(sessionId)
	if err != nil {
		return "", err
	}

	outDir, err := a.resolveOutputDir()
	if err != nil {
		return "", err
	}

	exportPath := filepath.Join(outDir, sanitizeExportBaseName(name)+"-recolored.mcpack")
	if err := createZipFromFolder(srcDir, exportPath); err != nil {
		return "", fmt.Errorf("failed to create export in %s: %s", outDir, exportWriteError(err, outDir))
	}

	a.touchSession(sessionId)
	if a.getBoolSetting("autoOpenFolder") {
		a.OpenFolder(outDir)
	}
	return fmt.Sprintf("Exported pack to %s", exportPath), nil
}

func exportWriteError(err error, outDir string) string {
	if errors.Is(err, fs.ErrPermission) {
		return fmt.Sprintf("access denied - pick a different output folder in Settings (currently %s)", outDir)
	}
	return err.Error()
}

// ExportCachePacks exports server/cached packs to the output directory.
func (a *App) ExportCachePacks(packNames []string, basePath string) ([]string, error) {
	a.logDebug(fmt.Sprintf("ExportCachePacks: called, packs=%d", len(packNames)))
	if len(packNames) == 0 {
		return nil, fmt.Errorf("no packs selected")
	}
	if basePath == "" {
		basePath = a.getDefaultPackCachePath()
	}
	outDir := a.getOutputDir()
	if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %v", err)
	}
	var written []string
	for _, packName := range packNames {
		if packName == "" || strings.Contains(packName, "..") || strings.ContainsAny(packName, `\/`) {
			return nil, fmt.Errorf("invalid pack name: %s", packName)
		}
		packDir := filepath.Join(basePath, packName)
		st, err := os.Stat(packDir)
		if err != nil || !st.IsDir() {
			return nil, fmt.Errorf("pack not found: %s", packName)
		}
		outFile := filepath.Join(outDir, packName+".mcpack")
		if err := zipDirToFile(packDir, outFile); err != nil {
			return nil, fmt.Errorf("failed to export pack %s: %v", packName, err)
		}
		a.logDebug(fmt.Sprintf("ExportCachePacks: wrote %s", outFile))
		written = append(written, outFile)
	}
	return written, nil
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
)

type ManifestPackInfo struct {
	DirName     string `json:"dirName"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IconURI     string `json:"iconURI"`
	HasManifest bool   `json:"hasManifest"`
	ModuleType  string `json:"moduleType"`
	PackVersion string `json:"packVersion"`
}

func versionSliceToText(v interface{}) string {
	arr, ok := v.([]interface{})
	if !ok {
		return ""
	}
	parts := make([]string, 0, len(arr))
	for _, p := range arr {
		switch n := p.(type) {
		case float64:
			parts = append(parts, fmt.Sprintf("%v", int(n)))
		case int:
			parts = append(parts, fmt.Sprintf("%v", n))
		default:
			parts = append(parts, fmt.Sprintf("%v", p))
		}
	}
	return strings.Join(parts, ".")
}

func (a *App) GetInstalledManifestPacks() ([]ManifestPackInfo, error) {
	base := a.getResourcePacksPath()
	if st, err := os.Stat(base); err != nil || !st.IsDir() {
		return nil, nil
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}
	var result []ManifestPackInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(base, e.Name())
		info := ManifestPackInfo{DirName: e.Name(), Name: e.Name()}
		if iconPath := findFirstImage(dir, "pack_icon"); iconPath != "" {
			info.IconURI = a.readImageAsDataURI(iconPath)
		}
		manifestPath := filepath.Join(dir, "manifest.json")
		if data, err := os.ReadFile(manifestPath); err == nil {
			var m map[string]interface{}
			if json.Unmarshal(data, &m) == nil && m != nil {
				info.HasManifest = true
				if header, ok := m["header"].(map[string]interface{}); ok {
					if name, ok := header["name"].(string); ok && name != "" {
						info.Name = name
					}
					if desc, ok := header["description"].(string); ok {
						info.Description = desc
					}
					info.PackVersion = versionSliceToText(header["version"])
				}
				if modules, ok := m["modules"].([]interface{}); ok {
					for _, mod := range modules {
						if mo, ok := mod.(map[string]interface{}); ok {
							if t, ok := mo["type"].(string); ok && t != "" {
								info.ModuleType = t
								break
							}
						}
					}
				}
			}
		}
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result, nil
}

func (a *App) GetPackManifest(packName string) (map[string]interface{}, error) {
	dir := a.getPackDir(packName)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("pack not found: %s", packName)
	}
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("no manifest.json in %s", packName)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("invalid manifest.json: %v", err)
	}
	return m, nil
}

func (a *App) NewManifestTemplate() (map[string]interface{}, error) {
	desc := "Created with MEW"
	return map[string]interface{}{
		"format_version": 1,
		"header": map[string]interface{}{
			"name":               "My Pack",
			"description":        desc,
			"uuid":               uuid.New().String(),
			"version":            []int{1, 0, 0},
			"min_engine_version": []int{1, 12, 1},
		},
		"modules": []interface{}{
			map[string]interface{}{
				"description": desc,
				"type":        "resources",
				"uuid":        uuid.New().String(),
				"version":     []int{1, 0, 0},
			},
		},
	}, nil
}

func (a *App) RegenerateUuid() (string, error) {
	return uuid.New().String(), nil
}

func sanitizeFolderName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Map(func(r rune) rune {
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			return ' '
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" {
		return "new_pack"
	}
	return name
}

func (a *App) CreatePackFromManifest(manifest map[string]interface{}) (string, error) {
	if err := validateManifest(manifest); err != nil {
		return "", err
	}
	header, _ := manifest["header"].(map[string]interface{})
	name, _ := header["name"].(string)
	folderName := sanitizeFolderName(name)
	if folderName == "" {
		return "", fmt.Errorf("pack name is required")
	}
	base := a.getResourcePacksPath()
	dir := filepath.Join(base, folderName)
	if _, err := os.Stat(dir); err == nil {
		for i := 2; ; i++ {
			candidate := filepath.Join(base, fmt.Sprintf("%s (%d)", folderName, i))
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				dir = candidate
				break
			}
		}
	}
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create pack folder: %v", err)
	}
	data, err := json.MarshalIndent(manifest, "", "    ")
	if err != nil {
		return "", fmt.Errorf("failed to build manifest: %v", err)
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write manifest: %v", err)
	}
	a.logDebug(fmt.Sprintf("CreatePackFromManifest: created pack at %s", dir))
	return filepath.Base(dir), nil
}

func (a *App) ApplyManifestToPack(packName string, manifest map[string]interface{}) error {
	dir := a.getPackDir(packName)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return fmt.Errorf("pack not found: %s", packName)
	}
	if manifest == nil {
		return fmt.Errorf("manifest is empty")
	}
	if err := validateManifest(manifest); err != nil {
		return err
	}
	if err := writeManifestWithBackup(dir, manifest); err != nil {
		return err
	}
	a.logDebug(fmt.Sprintf("ApplyManifestToPack: wrote manifest to %s", filepath.Join(dir, "manifest.json")))
	return nil
}

// GetPackManifestFromDir reads the manifest from an absolute pack directory.
// This is used for pack cache sources that live outside the resource packs folder.
func (a *App) GetPackManifestFromDir(dir string) (map[string]interface{}, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("no pack directory provided")
	}
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return nil, fmt.Errorf("pack directory not found: %s", dir)
	}
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("no manifest.json in %s", dir)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("invalid manifest.json: %v", err)
	}
	return m, nil
}

// ApplyManifestToDir writes a manifest back into an absolute pack directory
// (pack cache sources). The old file is backed up as manifest.json.bak.
func (a *App) ApplyManifestToDir(dir string, manifest map[string]interface{}) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return fmt.Errorf("no pack directory provided")
	}
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return fmt.Errorf("pack directory not found: %s", dir)
	}
	if manifest == nil {
		return fmt.Errorf("manifest is empty")
	}
	if err := validateManifest(manifest); err != nil {
		return err
	}
	if err := writeManifestWithBackup(dir, manifest); err != nil {
		return err
	}
	a.logDebug(fmt.Sprintf("ApplyManifestToDir: wrote manifest to %s", filepath.Join(dir, "manifest.json")))
	return nil
}

// GetManifestFromUpload extracts an uploaded .mcpack and returns its manifest.
// The extracted folder is kept so ApplyManifestToUpload can write it back later.
func (a *App) GetManifestFromUpload(bytes []byte) (map[string]interface{}, error) {
	if len(bytes) == 0 {
		return nil, fmt.Errorf("no file data provided")
	}
	dir := a.getTempDir("manifest_unzip")
	os.RemoveAll(dir)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to prepare temp folder: %v", err)
	}
	tmpFile := filepath.Join(os.TempDir(), "mew-manifest-pack.zip")
	if err := os.WriteFile(tmpFile, bytes, 0644); err != nil {
		return nil, fmt.Errorf("failed to write temp file: %v", err)
	}
	defer os.Remove(tmpFile)
	if err := unzip(tmpFile, dir); err != nil {
		return nil, fmt.Errorf("failed to extract pack: %v", err)
	}
	root, err := findManifestRoot(dir)
	if err != nil || root == "" {
		return nil, fmt.Errorf("no manifest.json found in the pack")
	}
	data, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest.json: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("invalid manifest.json: %v", err)
	}
	return m, nil
}

// ApplyManifestToUpload writes the edited manifest into the last uploaded
// .mcpack (see GetManifestFromUpload) and exports a new pack to the output folder.
func (a *App) ApplyManifestToUpload(manifest map[string]interface{}, fileName string) (string, error) {
	dir := a.getTempDir("manifest_unzip")
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return "", fmt.Errorf("upload an .mcpack first")
	}
	if manifest == nil {
		return "", fmt.Errorf("manifest is empty")
	}
	if err := validateManifest(manifest); err != nil {
		return "", err
	}
	root, err := findManifestRoot(dir)
	if err != nil || root == "" {
		root = dir
	}
	data, err := json.MarshalIndent(manifest, "", "    ")
	if err != nil {
		return "", fmt.Errorf("failed to build manifest: %v", err)
	}
	manifestPath := filepath.Join(root, "manifest.json")
	if old, err := os.ReadFile(manifestPath); err == nil {
		_ = os.WriteFile(dir+"-backup.json", old, 0644)
	}
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write manifest: %v", err)
	}
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	outPath := filepath.Join(a.getOutputDir(), base+"-updated.mcpack")
	if err := zipDirToFile(dir, outPath); err != nil {
		return "", fmt.Errorf("failed to create pack: %v", err)
	}
	a.logDebug(fmt.Sprintf("ApplyManifestToUpload: exported %s", outPath))
	return outPath, nil
}

// writeManifestWithBackup backs up an existing manifest.json (as .bak) and
// writes the new one into the pack directory.
func writeManifestWithBackup(dir string, manifest map[string]interface{}) error {
	manifestPath := filepath.Join(dir, "manifest.json")
	if _, err := os.Stat(manifestPath); err == nil {
		if data, err := os.ReadFile(manifestPath); err == nil {
			if err := os.WriteFile(manifestPath+".bak", data, 0644); err != nil {
				return fmt.Errorf("failed to back up existing manifest: %v", err)
			}
		} else {
			return fmt.Errorf("failed to read existing manifest for backup: %v", err)
		}
	}
	data, err := json.MarshalIndent(manifest, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to build manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %v", err)
	}
	return nil
}

func validateManifest(m map[string]interface{}) error {
	if _, ok := m["format_version"]; !ok {
		return fmt.Errorf("manifest is missing format_version")
	}
	header, ok := m["header"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("manifest is missing a header object")
	}
	if name, _ := header["name"].(string); strings.TrimSpace(name) == "" {
		return fmt.Errorf("pack name is required")
	}
	if u, _ := header["uuid"].(string); strings.TrimSpace(u) == "" {
		return fmt.Errorf("header uuid is required (use regenerate)")
	}
	return nil
}

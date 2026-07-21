package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/swim-services/swim_porter/port"
	"github.com/swim-services/swim_porter/porterror"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx          context.Context
	cancelFolder context.CancelFunc
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) emitProgress(title, message, icon, fileName string, total, completed int) {
	runtime.EventsEmit(a.ctx, "progress", map[string]interface{}{
		"title":     title,
		"message":   message,
		"icon":      icon,
		"total":     total,
		"completed": completed,
		"fileName":  fileName,
	})
}

func (a *App) emitFinished(title, message, icon string) {
	runtime.EventsEmit(a.ctx, "finished", map[string]interface{}{
		"title":   title,
		"message": message,
		"icon":    icon,
	})
}

func writeMcpack(zipBytes []byte, fileName string, outDir string) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return "", fmt.Errorf("failed to create zip reader: %v", err)
	}

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)

	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			writer.Close()
			return "", fmt.Errorf("failed to open file in zip: %v", err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			writer.Close()
			return "", fmt.Errorf("failed to read file data: %v", err)
		}

		newName := file.Name
		newName = strings.Replace(newName, "netherite_layer_1", "netherite_1", 1)
		newName = strings.Replace(newName, "netherite_layer_2", "netherite_2", 1)
		newName = strings.Replace(newName, "totem_of_undying", "totem", 1)

		f, err := writer.Create(newName)
		if err != nil {
			writer.Close()
			return "", fmt.Errorf("failed to create file in zip: %v", err)
		}
		if _, err = f.Write(data); err != nil {
			writer.Close()
			return "", fmt.Errorf("failed to write to file in zip: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close zip writer: %v", err)
	}

	outFile := filepath.Join(outDir, strings.TrimSuffix(fileName, filepath.Ext(fileName))+".mcpack")
	if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create output directory: %v", err)
	}
	if err := os.WriteFile(outFile, buf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("failed to write output file: %v", err)
	}

	fmt.Println("Porting finished:", outFile)
	return outFile, nil
}

func (a *App) PortPack(bytes []byte, fileName string) (string, error) {
	if !strings.HasSuffix(fileName, ".zip") {
		return "", fmt.Errorf("please provide a .zip file")
	}

	skyboxOverride := ""
	out, err := port.Port(bytes, fileName, port.PortOptions{ShowCredits: false, SkyboxOverride: skyboxOverride})
	if err != nil {
		errMsg := err.Error()
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

	outFile, err := writeMcpack(out, fileName, ".")
	if err != nil {
		return "", fmt.Errorf("failed to write mcpack: %v", err)
	}

	a.emitFinished("Porting finished", outFile, "success")
	return outFile, nil
}

func (a *App) CancelPortFolder() {
	if a.cancelFolder != nil {
		a.cancelFolder()
	}
}

func (a *App) PortFolder(url string) error {
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

	javaDir := filepath.Join(".", "Java")
	bedrockDir := filepath.Join(".", "Bedrock")
	os.MkdirAll(javaDir, os.ModePerm)
	os.MkdirAll(bedrockDir, os.ModePerm)

	if isFolder {
		a.emitProgress("Starting", "Reading folder page...", "info", "", 0, 0)

		fileLinks, err := extractMediaFireFolderLinks(client, url)
		if err != nil {
			a.emitProgress("Error", fmt.Sprintf("Failed to read folder: %v", err), "error", "", 0, 0)
			return err
		}

		if len(fileLinks) == 0 {
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
		a.emitProgress("Partial", fmt.Sprintf("Ported %d pack(s), %d failed. Check Java/ and Bedrock/ folders.", successCount, failCount), "warning", "", totalFiles, totalFiles)
	} else if failCount > 0 {
		a.emitProgress("Error", fmt.Sprintf("All %d pack(s) failed to port.", failCount), "error", "", totalFiles, totalFiles)
	} else {
		a.emitProgress("Done", fmt.Sprintf("Ported %d pack(s) to Bedrock/. Originals in Java/.", successCount), "success", "", totalFiles, totalFiles)
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

	tempDir := filepath.Join(".", "folder_port_temp")
	os.MkdirAll(tempDir, os.ModePerm)
	defer os.RemoveAll(tempDir)

	var candidates []fileCandidate

	lower := strings.ToLower(name)
	if !strings.HasSuffix(lower, ".zip") {
		log.Printf("Skipping %s: not a .zip file", name)
		a.emitProgress("Skipping", fmt.Sprintf("[%d/%d] %s — not a .zip", index+1, totalFiles, name), "fail", name, totalFiles, index+1)
		return false
	}

	zipPath := filepath.Join(tempDir, name)
	if err := os.WriteFile(zipPath, data, 0644); err != nil {
		log.Printf("Failed to write %s: %v", name, err)
		a.emitProgress("Skipping", fmt.Sprintf("[%d/%d] %s — write failed", index+1, totalFiles, name), "fail", name, totalFiles, index+1)
		return false
	}

	extractDir := filepath.Join(tempDir, strings.TrimSuffix(name, filepath.Ext(name)))
	if err := unzip(zipPath, extractDir); err != nil {
		candidates = append(candidates, fileCandidate{Name: name, ZipPath: zipPath, IsZip: true})
	} else {
		var innerZips []string
		filepath.Walk(extractDir, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".zip") {
				innerZips = append(innerZips, path)
			}
			return nil
		})

		if len(innerZips) > 0 {
			for _, iz := range innerZips {
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
			log.Printf("Failed to port %s: %v", c.Name, err)
			a.emitProgress("Progress", c.Name, "fail", c.Name, totalFiles, index+1)
			continue
		}

		reportOut, reportErr := writeMcpack(out, c.Name, bedrockDir)
		if reportErr != nil {
			log.Printf("Failed to write mcpack %s: %v", c.Name, reportErr)
			a.emitProgress("Progress", c.Name, "fail", c.Name, totalFiles, index+1)
			continue
		}

		javaDst := filepath.Join(javaDir, c.Name)
		if err := copyFile(c.ZipPath, javaDst); err != nil {
			log.Printf("Failed to copy %s to Java/: %v", c.Name, err)
		}

		_ = reportOut
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

	if err := unzip(tmpPath, "temp_unzip"); err != nil {
		return nil, fmt.Errorf("failed to unzip: %v", err)
	}

	if _, err := os.Stat(filepath.Join("temp_unzip", "manifest.json")); os.IsNotExist(err) {
		os.RemoveAll("./temp_unzip")
		return &CheckResult{Valid: false, ErrorMsg: "Please provide a valid pack."}, nil
	}

	texturesPath := filepath.Join("temp_unzip", "textures")
	var folders []string
	err = filepath.WalkDir(texturesPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != texturesPath {
			rel, err := filepath.Rel("temp_unzip", path)
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

	var images []FolderImage

	err := filepath.Walk(folder, func(path string, info fs.FileInfo, err error) error {
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

			relPath, err := filepath.Rel("temp_unzip", path)
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
	Hue       int    `json:"hue"`
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
		savePath = filepath.Join("temp_unzip", filepath.FromSlash(msg.RelPath))
	} else {
		var foundPath string
		filepath.Walk("temp_unzip", func(path string, info fs.FileInfo, err error) error {
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
			savePath = filepath.Join("temp_unzip", "textures", msg.ImageName)
		}
	}

	os.MkdirAll(filepath.Dir(savePath), os.ModePerm)

	err = os.WriteFile(savePath, imageData, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to save image: %v", err)
	}

	return "success", nil
}

func (a *App) ExportPack(name string) (string, error) {
	baseName := "exported_pack"
	if name != "" {
		baseName = strings.TrimSuffix(name, filepath.Ext(name))
	}

	exportName := baseName + "-recolored.mcpack"
	zipName := baseName + "-recolored.zip"

	if err := createZipFromFolder("temp_unzip", zipName); err != nil {
		return "", fmt.Errorf("failed to create export: %v", err)
	}

	if err := os.Rename(zipName, exportName); err != nil {
		return "", fmt.Errorf("failed to finalize export: %v", err)
	}

	return fmt.Sprintf("Exported pack to %s", exportName), nil
}

func (a *App) DeleteTemp() {
	os.RemoveAll("./temp_unzip")
}

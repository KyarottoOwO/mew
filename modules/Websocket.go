package modules

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/swim-services/swim_porter/port"
	"github.com/swim-services/swim_porter/porterror"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
var finishedPackConnections = make(map[*websocket.Conn]struct{})
var zz = make(map[*websocket.Conn]struct{})
var folderProgressConnections = make(map[*websocket.Conn]struct{})

type safeConn struct {
	conn  *websocket.Conn
	write sync.Mutex
}

type Message struct {
	Text  string `json:"text"`
	Bytes []byte `json:"bytes"`
}

type HueShiftMessage struct {
	ImageName string `json:"imageName"`
	ImagePath string `json:"imagePath"`
	Hue       int    `json:"hue"`
}

type SaveImageMessage struct {
	ImageName string `json:"imageName"`
	ImagePath string `json:"imagePath"`
	RelPath   string `json:"relPath"`
	ImageData string `json:"imageData"`
	Hue       int    `json:"hue"`
	Done      bool   `json:"done"`
}

func init() {
	help()
	RegisterMessageHandler()
	Registercheck()
	FinishedPack()
	Registerfolder()
	z()
	RegisterDeletetemp()
	RegisterHueShift()
	RegisterSaveImage()
	RegisterExportPack()
	RegisterPortFolder()
	RegisterFolderProgress()
}

func RegisterWebsocketHandler(path string, handler func(*websocket.Conn)) {
	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade error: %v", err)
			return
		}
		handler(conn)
	})
}

func handler(conn *websocket.Conn) {
	defer func() {
		log.Println("Closing connection...")
		conn.Close()
		log.Printf("Connection closed: %v", conn.RemoteAddr())
	}()
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Unexpected connection closure from %v: %v", conn.RemoteAddr(), err)
			} else {
				log.Printf("Read error from %v: %v", conn.RemoteAddr(), err)
			}
			return
		}
		log.Printf("Received message from %v: %s", conn.RemoteAddr(), message)
	}
}

func FinishedPack() {
	RegisterWebsocketHandler("/ws/FinishedPack", func(conn *websocket.Conn) {
		finishedPackConnections[conn] = struct{}{}
		log.Println("New connection established for FinishedPack")

		defer func() {
			delete(finishedPackConnections, conn)
			log.Println("Connection removed from finishedPackConnections")
			conn.Close()
		}()

		handler(conn)
	})
}

func SendMessage(conn *websocket.Conn, title string, message string, icon string) {
	fmt.Println("sending message")
	if icon == "success" {
		message = message + " Has been ported !"
	}

	msg := map[string]string{
		"title":   title,
		"message": message,
		"icon":    icon,
	}
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		log.Printf("JSON marshal error: %v", err)
		return
	}

	err = conn.WriteMessage(websocket.TextMessage, jsonMsg)
	if err != nil {
		log.Printf("write error: %v", err)
	}
}

func Port(FileName string, Bytes []byte) {
	if !strings.HasSuffix(FileName, ".zip") {
		return
	}
	skyboxOverride := ""
	out, err := port.Port(Bytes, FileName, port.PortOptions{ShowCredits: false, SkyboxOverride: skyboxOverride})
	if err != nil {
		errMsg := err.Error()
		log.Println(errMsg)

		var portError *porterror.PortError
		if errors.As(err, &portError) {
			if strings.Contains(errMsg, "pack.mcmeta not found") {
				for conn := range finishedPackConnections {
					SendMessage(conn, "Error", "Please provide a valid pack.", "error")
				}
				return
			}
			fmt.Println(portError.StackTrace())
		}
		return
	}
	Report(out, FileName)
}

func Report(zipBytes []byte, FileName string) {
	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		log.Println("Failed to create zip reader:", err)
		return
	}

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)

	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			log.Println("Failed to open file in zip:", err)
			return
		}
		data, err := ioutil.ReadAll(rc)
		rc.Close()
		if err != nil {
			log.Println("Failed to read file data:", err)
			return
		}

		newName := file.Name
		if strings.Contains(file.Name, "netherite_layer_1") {
			newName = strings.Replace(newName, "netherite_layer_1", "netherite_1", 1)
			fmt.Println("Renaming:", file.Name, "to", newName)
		}

		if strings.Contains(file.Name, "netherite_layer_2") {
			newName = strings.Replace(newName, "netherite_layer_2", "netherite_2", 1)
			fmt.Println("Renaming:", file.Name, "to", newName)
		}

		if strings.Contains(file.Name, "totem_of_undying") {
			newName = strings.Replace(file.Name, "totem_of_undying", "totem", 1)
			fmt.Println("Renaming:", file.Name, "to", newName)
		}

		f, err := writer.Create(newName)
		if err != nil {
			log.Println("Failed to create file in zip:", err)
			return
		}
		_, err = f.Write(data)
		if err != nil {
			log.Println("Failed to write to file in zip:", err)
			return
		}
	}

	err = writer.Close()
	if err != nil {
		log.Println("Failed to close zip writer:", err)
		return
	}

	outFile := filepath.Join(".", strings.TrimSuffix(FileName, filepath.Ext(FileName))) + ".mcpack"

	if err := os.WriteFile(outFile, buf.Bytes(), 0644); err != nil {
		log.Fatalln("Failed to write output file:", err)
	}

	fmt.Println("Porting finished:", outFile)

	for conn := range finishedPackConnections {
		SendMessage(conn, "porting finished", outFile, "success")
	}
}

func help() {
	RegisterWebsocketHandler("/ws/help", func(conn *websocket.Conn) {
		defer conn.Close()
		message := "hi"
		conn.WriteJSON(message)
	})
}

func listenForMessages(conn *websocket.Conn) {
	defer func() {
		log.Println("Closing connection...")
		conn.Close()
	}()

	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Unexpected connection closure: %v", err)
			} else {
				log.Printf("read error: %v", err)
			}
			return
		}

		Port(msg.Text, msg.Bytes)
	}
}

func RegisterMessageHandler() {
	RegisterWebsocketHandler("/ws/PortPack", func(conn *websocket.Conn) {
		listenForMessages(conn)
	})
}

func listenforcheck(conn *websocket.Conn) {
	defer func() {
		log.Println("Closing connection...")
		conn.Close()
	}()

	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Unexpected connection closure: %v", err)
			} else {
				log.Printf("read error: %v", err)
			}
			return
		}

		CheckPack(msg.Bytes)
	}
}

func Registercheck() {
	RegisterWebsocketHandler("/ws/CheckPack", func(conn *websocket.Conn) {
		listenforcheck(conn)
	})
}

func CheckPack(bytes []byte) {
	err := os.WriteFile("temp.zip", bytes, 0644)
	if err != nil {
		log.Fatalf("write zip file: %s", err)
	}
	fmt.Println("Wrote zip file")
	err = unzip("temp.zip", "temp_unzip")
	if err != nil {
		log.Fatalf("unzip file: %s", err)
	}
	err = os.Remove("temp.zip")
	if err != nil {
		log.Fatalf("delete zip file: %s", err)
	}

	err = filepath.WalkDir("temp_unzip\\textures", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != "temp_unzip\\textures" {
			// Remove only the prefix and the OS-specific separator
			rel, err := filepath.Rel("temp_unzip", path)
			if err != nil {
				return err
			}
			if strings.Contains(rel, "entity") {
				return nil
			}
			fmt.Println("Sending path:", rel)

			for conn := range zz {
				SendMessagez(conn, rel)
			}
		}
		return nil
	})

	if err != nil {
		log.Fatalf("walk dir: %s", err)
	}

	_, err = os.Stat(filepath.Join("temp_unzip", "manifest.json"))
	if os.IsNotExist(err) {
		for conn := range finishedPackConnections {
			SendMessage(conn, "Error", "Please provide a valid pack.", "error")
		}
		os.RemoveAll("./temp_unzip")
	}
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return &os.PathError{Op: "extract", Path: fpath, Err: os.ErrInvalid}
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func z() {
	RegisterWebsocketHandler("/ws/z", func(conn *websocket.Conn) {
		zz[conn] = struct{}{}
		log.Println("New connection established for CheckPack")

		defer func() {
			delete(zz, conn)
			log.Println("Connection removed from zz")
			conn.Close()
		}()

		handler(conn)
	})
}

func listenDeletetemp(conn *websocket.Conn) {
	defer func() {
		log.Println("Closing connection...")
		conn.Close()
	}()

	for {
		var msg Message

		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Unexpected connection closure: %v", err)
			} else {
				log.Printf("read error: %v", err)
			}
			return
		}

		if !strings.Contains(msg.Text, "export") {
			os.RemoveAll("./temp_unzip")
		}
	}
}

func RegisterDeletetemp() {
	RegisterWebsocketHandler("/ws/Deletetemp", func(conn *websocket.Conn) {
		listenDeletetemp(conn)
	})
}

func SendMessagez(conn *websocket.Conn, message string) {
	msg := map[string]string{
		"message": message,
	}
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		log.Printf("JSON marshal error: %v", err)
		return
	}

	err = conn.WriteMessage(websocket.TextMessage, jsonMsg)
	if err != nil {
		log.Printf("write error: %v", err)
	}
}

func listenfolder(conn *websocket.Conn) {
	defer func() {
		log.Println("Closing connection...")
		conn.Close()
	}()

	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Unexpected connection closure: %v", err)
			} else {
				log.Printf("read error: %v", err)
			}
			return
		}
		fmt.Println("Received:", msg.Text)
		GetInnerFolder(fmt.Sprintf("temp_unzip\\%s", msg.Text))
	}
}

func GetInnerFolder(folder string) {
	re := regexp.MustCompile(`\\\s`)
	folder = re.ReplaceAllString(folder, `\`)
	fmt.Println(folder)

	filepath.Walk(folder, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.Contains(path, "textures\\entity") {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
			// Read the image file
			imageData, err := os.ReadFile(path)
			if err != nil {
				fmt.Printf("Error reading %s: %v\n", path, err)
				return nil
			}

			// Get filename without extension
			filename := filepath.Base(path)
			nameWithoutExt := strings.TrimSuffix(filename, ext)

			// Get file size
			fileSize := info.Size()

			// Determine correct MIME type
			mimeType := "image/png"
			if ext == ".jpg" || ext == ".jpeg" {
				mimeType = "image/jpeg"
			}

			// Encode to base64
			encoded := base64.StdEncoding.EncodeToString(imageData)
			dataURI := "data:" + mimeType + ";base64," + encoded

			// Send in chunks
			const chunkSize = 50000
			totalChunks := (len(dataURI) + chunkSize - 1) / chunkSize

			// Get relative path from temp_unzip
			relPath, err := filepath.Rel("temp_unzip", path)
			if err != nil {
				relPath = path
			}
			// Normalize path separators for cross-platform compatibility
			relPath = filepath.ToSlash(relPath)

			for conn := range zz {
				// Send metadata with size, name without extension, and relative path
				metadata := fmt.Sprintf(`{"type":"start","filename":"%s","size":%d,"chunks":%d,"relPath":"%s"}`,
					nameWithoutExt, fileSize, totalChunks, relPath)
				SendMessagez(conn, metadata)

				// Send chunks
				for i := 0; i < len(dataURI); i += chunkSize {
					end := i + chunkSize
					if end > len(dataURI) {
						end = len(dataURI)
					}

					chunkNum := i / chunkSize
					chunk := dataURI[i:end]

					chunkMsg := fmt.Sprintf(`{"type":"chunk","index":%d,"data":"%s"}`, chunkNum, chunk)
					SendMessagez(conn, chunkMsg)

					fmt.Printf("Sent chunk %d/%d for %s (size: %d bytes)\n",
						chunkNum+1, totalChunks, nameWithoutExt, fileSize)
				}

				endMsg := fmt.Sprintf(`{"type":"end","filename":"%s","size":%d,"relPath":"%s"}`,
					nameWithoutExt, fileSize, relPath)
				SendMessagez(conn, endMsg)
			}
		}
		return nil
	})
}

func Registerfolder() {
	RegisterWebsocketHandler("/ws/folder", func(conn *websocket.Conn) {
		listenfolder(conn)
	})
}

func listenHueShift(conn *websocket.Conn) {
	defer func() {
		log.Println("Closing hue shift connection...")
		conn.Close()
	}()

	for {
		var msg HueShiftMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Unexpected connection closure: %v", err)
			} else {
				log.Printf("read error: %v", err)
			}
			return
		}
		log.Printf("Received hue shift request: ImageName=%s, Hue=%d", msg.ImageName, msg.Hue)
		// TODO: Implement hue shift logic here
		// This is where you would process the image and apply the hue shift
	}
}

func RegisterHueShift() {
	RegisterWebsocketHandler("/ws/HueShift", func(conn *websocket.Conn) {
		listenHueShift(conn)
	})
}

func RegisterExportPack() {
	RegisterWebsocketHandler("/ws/ExportPack", func(conn *websocket.Conn) {
		defer conn.Close()

		type ExportRequest struct {
			Action string `json:"action"`
			Name   string `json:"name"`
		}

		var req ExportRequest
		if err := conn.ReadJSON(&req); err != nil {
			log.Printf("read export request error: %v", err)
			return
		}

		baseName := "exported_pack"
		if req.Name != "" {
			baseName = strings.TrimSuffix(req.Name, filepath.Ext(req.Name))
		}

		exportName := baseName + "-recolored.mcpack"
		zipName := baseName + "-recolored.zip"

		if err := createZipFromFolder("temp_unzip", zipName); err != nil {
			log.Printf("error creating export zip: %v", err)
			resp := map[string]string{"status": "error", "message": fmt.Sprintf("Failed to create export: %v", err)}
			jsonMsg, _ := json.Marshal(resp)
			conn.WriteMessage(websocket.TextMessage, jsonMsg)
			return
		}

		if err := os.Rename(zipName, exportName); err != nil {
			log.Printf("error renaming export zip: %v", err)
			resp := map[string]string{"status": "error", "message": fmt.Sprintf("Failed to finalize export: %v", err)}
			jsonMsg, _ := json.Marshal(resp)
			conn.WriteMessage(websocket.TextMessage, jsonMsg)
			return
		}

		resp := map[string]string{"status": "success", "message": fmt.Sprintf("Exported pack to %s", exportName)}
		jsonMsg, _ := json.Marshal(resp)
		conn.WriteMessage(websocket.TextMessage, jsonMsg)
	})
}

func createZipFromFolder(srcDir, zipPath string) error {
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

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		f, err := w.Create(relPath)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		if _, err := io.Copy(f, file); err != nil {
			return err
		}

		return nil
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func listenSaveImage(conn *websocket.Conn) {
	defer func() {
		log.Println("Closing save image connection...")
		conn.Close()
	}()

	for {
		var msg SaveImageMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Unexpected connection closure: %v", err)
			} else {
				log.Printf("read error: %v", err)
			}
			return
		}
		fmt.Println(msg.Done)
		if msg.Done {
			log.Println("Batch processing done — notifying client.")
			conn.WriteMessage(websocket.TextMessage, []byte("done"))
			continue
		}

		log.Printf("Received save image request: ImageName=%s, Hue=%d", msg.ImageName, msg.Hue)

		imageData, err := base64.StdEncoding.DecodeString(msg.ImageData)
		if err != nil {
			log.Printf("Error decoding base64 image: %v", err)
			continue
		}

		var savePath string
		if msg.RelPath != "" {
			savePath = filepath.Join("temp_unzip", filepath.FromSlash(msg.RelPath))
		} else {
			var foundPath string
			filepath.Walk("temp_unzip", func(path string, info fs.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() {
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
			log.Printf("Error saving image to %s: %v", savePath, err)
			errorMsg := map[string]string{"status": "error", "message": fmt.Sprintf("Failed to save image: %v", err)}
			jsonMsg, _ := json.Marshal(errorMsg)
			conn.WriteMessage(websocket.TextMessage, jsonMsg)
		} else {
			log.Printf("Successfully saved image to %s", savePath)
			successMsg := map[string]string{"status": "success", "message": "Image saved successfully"}
			jsonMsg, _ := json.Marshal(successMsg)
			conn.WriteMessage(websocket.TextMessage, jsonMsg)
		}
	}
}

func RegisterSaveImage() {
	RegisterWebsocketHandler("/ws/SaveImage", func(conn *websocket.Conn) {
		listenSaveImage(conn)
	})
}

func RegisterFolderProgress() {
	RegisterWebsocketHandler("/ws/FolderProgress", func(conn *websocket.Conn) {
		folderProgressConnections[conn] = struct{}{}
		log.Println("New connection established for FolderProgress")

		defer func() {
			delete(folderProgressConnections, conn)
			log.Println("Connection removed from folderProgressConnections")
			conn.Close()
		}()

		handler(conn)
	})
}

func sendFolderProgress(title, message, icon string) {
	sendFolderProgressDetailed(title, message, icon, "", 0, 0)
}

func sendFolderProgressDetailed(title, message, icon, fileName string, total, completed int) {
	msg := map[string]string{
		"title":     title,
		"message":   message,
		"icon":      icon,
		"total":     fmt.Sprintf("%d", total),
		"completed": fmt.Sprintf("%d", completed),
		"fileName":  fileName,
	}
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		log.Printf("JSON marshal error: %v", err)
		return
	}
	for conn := range folderProgressConnections {
		err = conn.WriteMessage(websocket.TextMessage, jsonMsg)
		if err != nil {
			log.Printf("write error: %v", err)
		}
	}
}

func sendWSJSON(sc *safeConn, data map[string]string) {
	jsonMsg, err := json.Marshal(data)
	if err != nil {
		log.Printf("JSON marshal error: %v", err)
		return
	}
	sc.write.Lock()
	defer sc.write.Unlock()
	sc.conn.WriteMessage(websocket.TextMessage, jsonMsg)
}

func sendConnProgress(sc *safeConn, title, message, icon string) {
	sendConnProgressDetailed(sc, title, message, icon, "", 0, 0)
}

func sendConnProgressDetailed(sc *safeConn, title, message, icon, fileName string, total, completed int) {
	sendWSJSON(sc, map[string]string{
		"title":     title,
		"message":   message,
		"icon":      icon,
		"total":     fmt.Sprintf("%d", total),
		"completed": fmt.Sprintf("%d", completed),
		"fileName":  fileName,
	})
}

func RegisterPortFolder() {
	RegisterWebsocketHandler("/ws/PortFolder", func(conn *websocket.Conn) {
		defer conn.Close()

		sc := &safeConn{conn: conn}

		log.Println("[PortFolder] New connection")

		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("[PortFolder] read error: %v", err)
			return
		}

		url := strings.TrimSpace(msg.Text)
		log.Printf("[PortFolder] Received URL: %s", url)
		if url == "" {
			sendConnProgress(sc, "Error", "No URL provided.", "error")
			return
		}

		client := &http.Client{
			Timeout: 60 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		}

		isFolder := isMediaFireURL(url) && strings.Contains(url, "/folder/")
		log.Printf("[PortFolder] isMediaFire=%v isFolder=%v", isMediaFireURL(url), isFolder)

		type fileEntry struct {
			Name string
			Data []byte
		}
		var files []fileEntry

		if isFolder {
			sendConnProgress(sc, "Starting", "Reading folder page...", "info")

			fileLinks, err := extractMediaFireFolderLinks(client, url)
			if err != nil {
				log.Printf("[PortFolder] folder scrape error: %v", err)
				sendConnProgress(sc, "Error", fmt.Sprintf("Failed to read folder: %v", err), "error")
				return
			}

			log.Printf("[PortFolder] Found %d files in folder", len(fileLinks))
			if len(fileLinks) == 0 {
				sendConnProgress(sc, "Error", "No files found in the MediaFire folder.", "error")
				return
			}

			sendConnProgressDetailed(sc, "Found", fmt.Sprintf("Found %d file(s). Downloading...", len(fileLinks)), "info", "", len(fileLinks), 0)

			for i, fl := range fileLinks {
				log.Printf("[PortFolder] Downloading [%d/%d] %s from %s", i+1, len(fileLinks), fl.Name, fl.URL)
				sendConnProgressDetailed(sc, "Downloading", fmt.Sprintf("[%d/%d] %s", i+1, len(fileLinks), fl.Name), "info", fl.Name, len(fileLinks), i)

				data, _, err := downloadDirect(client, fl.URL)
				if err != nil {
					log.Printf("[PortFolder] failed to download %s: %v", fl.Name, err)
					sendConnProgress(sc, "Warning", fmt.Sprintf("Skipped %s: %v", fl.Name, err), "warning")
					continue
				}
				log.Printf("[PortFolder] Downloaded %s (%d bytes)", fl.Name, len(data))
				files = append(files, fileEntry{Name: fl.Name, Data: data})
			}
		} else {
			sendConnProgress(sc, "Starting", "Downloading file...", "info")

			data, filename, err := downloadFromURL(url)
			if err != nil {
				log.Printf("[PortFolder] download error: %v", err)
				sendConnProgress(sc, "Error", fmt.Sprintf("Failed to download: %v", err), "error")
				return
			}

			files = append(files, fileEntry{Name: filename, Data: data})
		}

		if len(files) == 0 {
			sendConnProgress(sc, "Error", "No files were downloaded.", "error")
			return
		}

		type fileCandidate struct {
			Name    string
			ZipPath string
			IsZip   bool
		}
		var candidates []fileCandidate

		tempDir := filepath.Join(".", "folder_port_temp")
		os.MkdirAll(tempDir, os.ModePerm)
		defer os.RemoveAll(tempDir)

		for _, f := range files {
			lower := strings.ToLower(f.Name)
			if strings.HasSuffix(lower, ".zip") {
				zipPath := filepath.Join(tempDir, f.Name)
				if err := os.WriteFile(zipPath, f.Data, 0644); err != nil {
					log.Printf("[PortFolder] failed to write %s: %v", f.Name, err)
					continue
				}

				extractDir := filepath.Join(tempDir, strings.TrimSuffix(f.Name, filepath.Ext(f.Name)))
				if err := unzip(zipPath, extractDir); err != nil {
					candidates = append(candidates, fileCandidate{Name: f.Name, ZipPath: zipPath, IsZip: true})
					continue
				}

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
					candidates = append(candidates, fileCandidate{Name: f.Name, ZipPath: zipPath, IsZip: true})
				}
			}
		}

		if len(candidates) == 0 {
			sendConnProgress(sc, "Error", "No .zip pack files found.", "error")
			return
		}

		log.Printf("[PortFolder] %d pack(s) to port", len(candidates))
		sendConnProgressDetailed(sc, "Porting", fmt.Sprintf("Porting %d pack(s)...", len(candidates)), "info", "", len(candidates), 0)

		javaDir := filepath.Join(".", "Java")
		bedrockDir := filepath.Join(".", "Bedrock")
		os.MkdirAll(javaDir, os.ModePerm)
		os.MkdirAll(bedrockDir, os.ModePerm)

		type portResult struct {
			FileName string
			Output   string
			Error    error
		}

		results := make([]portResult, len(candidates))
		var wg sync.WaitGroup
		var completedCount int32
		sem := make(chan struct{}, 4)

		for i, c := range candidates {
			wg.Add(1)
			go func(idx int, c fileCandidate) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				baseName := c.Name
				sendConnProgressDetailed(sc, "Porting", baseName, "info", baseName, len(candidates), int(atomic.LoadInt32(&completedCount)))

				data, err := os.ReadFile(c.ZipPath)
				if err != nil {
					results[idx] = portResult{FileName: baseName, Error: err}
					atomic.AddInt32(&completedCount, 1)
					sendConnProgressDetailed(sc, "Progress", baseName, "fail", baseName, len(candidates), int(atomic.LoadInt32(&completedCount)))
					return
				}

				out, err := port.Port(data, baseName, port.PortOptions{ShowCredits: false})
				if err != nil {
					results[idx] = portResult{FileName: baseName, Error: err}
					atomic.AddInt32(&completedCount, 1)
					sendConnProgressDetailed(sc, "Progress", baseName, "fail", baseName, len(candidates), int(atomic.LoadInt32(&completedCount)))
					return
				}

				reportOut, reportErr := reportFolder(out, baseName, bedrockDir)
				if reportErr != nil {
					results[idx] = portResult{FileName: baseName, Error: reportErr}
					atomic.AddInt32(&completedCount, 1)
					sendConnProgressDetailed(sc, "Progress", baseName, "fail", baseName, len(candidates), int(atomic.LoadInt32(&completedCount)))
					return
				}

				results[idx] = portResult{FileName: baseName, Output: reportOut}
				atomic.AddInt32(&completedCount, 1)
				sendConnProgressDetailed(sc, "Progress", baseName, "done", baseName, len(candidates), int(atomic.LoadInt32(&completedCount)))
			}(i, c)
		}

		wg.Wait()

		for _, c := range candidates {
			javaDst := filepath.Join(javaDir, c.Name)
			if err := copyFile(c.ZipPath, javaDst); err != nil {
				log.Printf("[PortFolder] Failed to copy %s to Java/: %v", c.Name, err)
			}
		}

		successCount := 0
		failCount := 0
		for _, r := range results {
			if r.Error != nil {
				failCount++
				log.Printf("[PortFolder] Failed to port %s: %v", r.FileName, r.Error)
			} else {
				successCount++
				log.Printf("[PortFolder] Ported: %s -> %s", r.FileName, r.Output)
			}
		}

		if failCount > 0 && successCount > 0 {
			sendConnProgress(sc, "Partial", fmt.Sprintf("Ported %d pack(s), %d failed. Check Java/ and Bedrock/ folders.", successCount, failCount), "warning")
		} else if failCount > 0 {
			sendConnProgress(sc, "Error", fmt.Sprintf("All %d pack(s) failed to port.", failCount), "error")
		} else {
			sendConnProgress(sc, "Done", fmt.Sprintf("Ported %d pack(s) to Bedrock/. Originals in Java/.", successCount), "success")
		}
	})
}

func reportFolder(zipBytes []byte, FileName string, outDir string) (string, error) {
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
		data, err := ioutil.ReadAll(rc)
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

	outFile := filepath.Join(outDir, strings.TrimSuffix(FileName, filepath.Ext(FileName))+".mcpack")
	if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create output directory: %v", err)
	}
	if err := os.WriteFile(outFile, buf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("failed to write output file: %v", err)
	}

	fmt.Println("Porting finished:", outFile)
	return outFile, nil
}

func downloadFromURL(url string) ([]byte, string, error) {
	client := &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	if isMediaFireURL(url) {
		directURL, err := extractMediaFireDownloadURL(client, url)
		if err != nil {
			return nil, "", fmt.Errorf("failed to extract MediaFire download URL: %v", err)
		}
		url = directURL
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response body: %v", err)
	}

	filename := extractFilename(resp, url)

	return data, filename, nil
}

func isMediaFireURL(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "mediafire.com")
}

func isMediaFireFolderURL(url string) bool {
	return isMediaFireURL(url) && strings.Contains(url, "/folder/")
}

type mediaFireFileEntry struct {
	Name string
	URL  string
}

func extractMediaFireFolderLinks(client *http.Client, folderURL string) ([]mediaFireFileEntry, error) {
	folderKey := ""
	parts := strings.Split(folderURL, "/")
	for i, p := range parts {
		if p == "folder" && i+1 < len(parts) {
			folderKey = parts[i+1]
			break
		}
	}
	if folderKey == "" {
		return nil, fmt.Errorf("could not extract folder key from URL")
	}

	type mfFileLink struct {
		NormalDownload string `json:"normal_download"`
	}
	type mfFile struct {
		Quickkey string      `json:"quickkey"`
		Filename string      `json:"filename"`
		Links    mfFileLink  `json:"links"`
	}
	type mfFolderContent struct {
		Files []mfFile `json:"files"`
	}
	type mfResponse struct {
		FolderContent mfFolderContent `json:"folder_content"`
	}
	type mfAPIResponse struct {
		Response mfResponse `json:"response"`
	}

	var allFiles []mediaFireFileEntry
	chunk := 1
	for {
		apiURL := fmt.Sprintf("https://www.mediafire.com/api/1.5/folder/get_content.php?folder_key=%s&content_type=files&response_format=json&chunk=%d&first_item=1&last_item=100", folderKey, chunk)

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch MediaFire API: %v", err)
		}

		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read API response: %v", err)
		}

		var apiResp mfAPIResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			return nil, fmt.Errorf("failed to parse API response: %v", err)
		}

		files := apiResp.Response.FolderContent.Files
		if len(files) == 0 {
			break
		}

		for _, f := range files {
			if f.Links.NormalDownload == "" {
				continue
			}
			name := f.Filename
			name = strings.ReplaceAll(name, "+", " ")
			allFiles = append(allFiles, mediaFireFileEntry{
				Name: name,
				URL:  f.Links.NormalDownload,
			})
		}

		if len(files) < 100 {
			break
		}
		chunk++
	}

	return allFiles, nil
}

func downloadDirect(client *http.Client, url string) ([]byte, string, error) {
	dlURL := url
	if isMediaFireURL(url) && !isMediaFireFolderURL(url) {
		extracted, err := extractMediaFireDownloadURL(client, url)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get MediaFire download link: %v", err)
		}
		dlURL = extracted
	}

	req, err := http.NewRequest("GET", dlURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("status %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/html") {
		return nil, "", fmt.Errorf("got HTML instead of file (likely a bad download link)")
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	return data, extractFilename(resp, dlURL), nil
}

func extractMediaFireDownloadURL(client *http.Client, pageURL string) (string, error) {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch MediaFire page: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read MediaFire page: %v", err)
	}

	html := string(body)

	patterns := []string{
		`id="downloadButton"[^>]*href="([^"]+)"`,
		`class="input popsok"[^>]*href="([^"]+)"`,
		`href="(https?://download\d*[^"]*\.zip[^"]*)"`,
		`href="(https?://[^"]*\.mediafire\.com/[^"]*)"`,
		`var\s+LimiServer\s*=\s*"([^"]+)"`,
		`class="download_link[^"]*"[^>]*href="([^"]+)"`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 {
			dlURL := strings.Trim(matches[1], `"`)
			if strings.HasPrefix(dlURL, "//") {
				dlURL = "https:" + dlURL
			}
			return dlURL, nil
		}
	}

	dlRe := regexp.MustCompile(`href="(https?://[^"]*)"`)
	allLinks := dlRe.FindAllStringSubmatch(html, -1)
	for _, match := range allLinks {
		if len(match) > 1 {
			link := match[1]
			if strings.Contains(link, "download") || strings.Contains(link, ".zip") {
				return link, nil
			}
		}
	}

	return "", fmt.Errorf("could not find download link on MediaFire page")
}

func extractFilename(resp *http.Response, url string) string {
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		re := regexp.MustCompile(`filename[*]?="?([^";\n]+)"?`)
		matches := re.FindStringSubmatch(cd)
		if len(matches) > 1 {
			name := strings.TrimSpace(matches[1])
			name = strings.Trim(name, `"`)
			if name != "" {
				return name
			}
		}
	}

	parts := strings.Split(url, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" && strings.Contains(parts[i], ".") {
			return parts[i]
		}
	}

	return "download.zip"
}

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

	"github.com/gorilla/websocket"
	"github.com/swim-services/swim_porter/port"
	"github.com/swim-services/swim_porter/porterror"
)

var upgrader = websocket.Upgrader{}
var finishedPackConnections = make(map[*websocket.Conn]struct{})
var zz = make(map[*websocket.Conn]struct{})

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

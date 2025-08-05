package modules

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/swim-services/swim_porter/port"
	"github.com/swim-services/swim_porter/porterror"
)

var upgrader = websocket.Upgrader{}
var finishedPackConnections = make(map[*websocket.Conn]struct{})

type Message struct {
	Text  string `json:"text"`
	Bytes []byte `json:"bytes"`
}

func init() {
	PopupMenus()
	RegisterMessageHandler()
	FinishedPack()
}

func RegisterWebsocketHandler(path string, handler func(*websocket.Conn)) {
	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade error: %v", err)
			return
		}
		log.Println("New connection established")
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

func SendMessage(conn *websocket.Conn, message string) {
	msg := map[string]string{"message": message}
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
	output := "."
	skyboxOverride := ""
	out, err := port.Port(Bytes, FileName, port.PortOptions{ShowCredits: false, SkyboxOverride: skyboxOverride})
	if err != nil {
		log.Println(err.Error())
		var portError *porterror.PortError
		if errors.As(err, &portError) {
			fmt.Println(portError.StackTrace())
		}
		return
	}
	outFile := filepath.Join(output, strings.TrimSuffix(FileName, filepath.Ext(FileName))) + ".mcpack"
	if err := os.WriteFile(outFile, out, 0644); err != nil {
		log.Fatalln(err)
	}
	fmt.Print("Porting finished: " + outFile + "\n")
	for conn := range finishedPackConnections {
		SendMessage(conn, outFile)
	}
}

func PopupMenus() {
	RegisterWebsocketHandler("/ws/PopupMenu", func(conn *websocket.Conn) {
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

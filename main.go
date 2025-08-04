// go: generate goversioninfo -icon = icon.ico
package main

import (
	_ "Cura/modules"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	webview "github.com/webview/webview_go"
)

var Main string

func main() {
	data, err := os.ReadFile("./html/Main.html")
	if err != nil {
		fmt.Printf("Error reading Main.html: %v\n", err)
		os.Exit(1)
	}
	Main = string(data)

	OpenPort := findOpenPort()
	go func() {
		fmt.Println("Starting server on port " + strconv.Itoa(OpenPort))
		http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
			http.ServeContent(writer, request, "index.html", time.Now(), strings.NewReader(Main))
		})
		if err := http.ListenAndServe("127.0.0.1:"+strconv.Itoa(OpenPort), nil); err != nil {
			fmt.Printf("Error starting server: %s", err)
			os.Exit(1)
		}
	}()

	w := webview.New(true)

	w.SetTitle("Cura ᨀ github.com/kyarottoOwO")
	w.SetSize(1200, 800, webview.HintFixed)

	defer w.Destroy()

	w.Dispatch(func() {
		w.Navigate("http://127.0.0.1:" + strconv.Itoa(OpenPort) + "/Cura.HTML")
	})

	w.Run()
}

func findOpenPort() int {
	for i := 1932; i < 65535; i++ {
		conn, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(i))
		if err != nil {
			continue
		}
		_ = conn.Close()
		return i
	}
	return -1
}

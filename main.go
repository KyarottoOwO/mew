package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	_ "srm/modules"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "embed"

	webview "github.com/webview/webview_go"
)

//go:embed html/Main.HTML
var Main string

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	defer os.RemoveAll("./temp_unzip")

	go func() {
		<-c
		fmt.Println("Received interrupt, cleaning up...")
		os.RemoveAll("./temp_unzip")
		os.Exit(0)
	}()

	OpenPort := findOpenPort()
	go func() {
		fmt.Println("Starting server on port " + strconv.Itoa(OpenPort))
		http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
			http.ServeContent(writer, request, "index.html", time.Now(), strings.NewReader(Main))
		})
		if err := http.ListenAndServe("127.0.0.1:"+strconv.Itoa(OpenPort), nil); err != nil {
			fmt.Printf("Error starting server: %s\n", err)
			os.Exit(1)
		}
	}()

	w := webview.New(true) //debug t/f
	w.SetTitle("SRM")
	w.SetSize(1200, 800, webview.HintFixed)
	defer w.Destroy()

	w.Dispatch(func() {
		w.Navigate("http://127.0.0.1:" + strconv.Itoa(OpenPort) + "/SRM.HTML")
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

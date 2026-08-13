package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	debug := false
	for _, arg := range os.Args[1:] {
		if arg == "--debug" {
			debug = true
			break
		}
	}

	if debug {
		allocConsole()
		log.SetOutput(os.Stderr)
		log.SetFlags(log.Ltime | log.Lshortfile)
		log.Println("Debug mode enabled")
	}

	app := NewApp(debug)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("Received interrupt, cleaning up...")
		os.RemoveAll("./temp_unzip")
		os.RemoveAll("./folder_port_temp")
		os.Exit(0)
	}()
	defer os.RemoveAll("./temp_unzip")
	defer os.RemoveAll("./folder_port_temp")
	defer os.RemoveAll("./anim_port_temp")
	defer os.RemoveAll("./anim_merge_temp")

	err := wails.Run(&options.App{
		Title:     "MEW",
		Width:     1200,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 2, G: 2, B: 2, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

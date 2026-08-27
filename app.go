package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

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
	a.GetSettings()
	if a.debug {
		log.Println("[startup] App context initialized")
	}
	a.startDiscordRPC()
}

func (a *App) logDebug(msg string) {
	if a.debug {
		log.Println("[DEBUG]", msg)
	}
	debugLog("[DEBUG] " + msg)
}

func (a *App) IsDebug() bool {
	return a.debug
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

package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"gopkg.in/natefinch/npipe.v2"
)

const (
	discordClientID    = "1513804565366177882"
	discordLargeImage  = "mew_logo"
	discordGitHubURL   = "https://github.com/KyarottoOwO/mew"
	discordInviteURL   = "https://discord.gg/nv9GrqTVM3"
	discordDialTimeout = 2 * time.Second
)

const (
	discordOpHandshake = 0
	discordOpFrame     = 1
	discordOpClose     = 2
	discordOpPing      = 3
	discordOpPong      = 4
)

const discordMaxFrame = 1 << 20

type discordHandshake struct {
	V        string `json:"v"`
	ClientID string `json:"client_id"`
}

type discordFrame struct {
	Cmd   string               `json:"cmd"`
	Args  discordActivityArgs  `json:"args"`
	Nonce string               `json:"nonce"`
}

type discordActivityArgs struct {
	Pid      int              `json:"pid"`
	Activity discordActivity  `json:"activity"`
}

type discordActivity struct {
	Details    string             `json:"details,omitempty"`
	State      string             `json:"state,omitempty"`
	Timestamps *discordTimestamps `json:"timestamps,omitempty"`
	Assets     discordAssets      `json:"assets,omitempty"`
	Buttons    []discordButton    `json:"buttons,omitempty"`
}

type discordTimestamps struct {
	Start int64 `json:"start"`
}

type discordAssets struct {
	LargeImage string `json:"large_image,omitempty"`
	LargeText  string `json:"large_text,omitempty"`
}

type discordButton struct {
	Label string `json:"label"`
	Url   string `json:"url"`
}

func (a *App) startDiscordRPC() {
	if discordClientID == "" {
		return
	}
	a.discordStart = time.Now()
	a.discordStop = make(chan struct{})
	go a.discordLoop()
}

func (a *App) shutdown(ctx context.Context) {
	a.stopDiscordRPC()
}

func (a *App) stopDiscordRPC() {
	if a.discordStop != nil {
		close(a.discordStop)
		a.discordStop = nil
	}
	a.discordClose()
}

func (a *App) discordLoop() {
	for {
		select {
		case <-a.discordStop:
			return
		default:
		}

		if !a.getBoolSetting("discordRPC") {
			a.discordClose()
			time.Sleep(5 * time.Second)
			continue
		}

		a.discordReconcile()
		a.refreshDiscordActivity()
		time.Sleep(5 * time.Second)
	}
}

func (a *App) discordReconcile() {
	want := discordClientPipes()
	wantSet := make(map[string]bool, len(want))
	for _, name := range want {
		wantSet[name] = true
	}

	a.discordIpcMu.Lock()
	have := make(map[string]bool, len(a.discordConns))
	for name := range a.discordConns {
		have[name] = true
	}
	a.discordIpcMu.Unlock()

	for _, name := range want {
		if have[name] {
			continue
		}
		conn, err := npipe.DialTimeout(name, discordDialTimeout)
		if err != nil {
			continue
		}
		a.discordIpcMu.Lock()
		a.discordConns[name] = conn
		a.discordIpcMu.Unlock()
		go a.discordReadLoop(name, conn)
		if payload, err := json.Marshal(discordHandshake{V: "1", ClientID: discordClientID}); err == nil {
			a.discordSend(conn, discordOpHandshake, payload)
		}
	}

	a.discordIpcMu.Lock()
	for name, conn := range a.discordConns {
		if !wantSet[name] {
			delete(a.discordConns, name)
			conn.Close()
		}
	}
	a.discordIpcMu.Unlock()
}

func discordClientPipes() []string {
	var pipes []string
	for i := 0; i < 40; i++ {
		name := fmt.Sprintf(`\\.\pipe\discord-ipc-%d`, i)
		ptr, err := windows.UTF16PtrFromString(name)
		if err != nil {
			continue
		}
		h, err := windows.CreateFile(
			ptr,
			windows.GENERIC_READ,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
			nil,
			windows.OPEN_EXISTING,
			windows.FILE_FLAG_OVERLAPPED,
			0,
		)
		if err != nil {
			continue
		}
		var pid uint32
		pe := windows.GetNamedPipeServerProcessId(h, &pid)
		windows.CloseHandle(h)
		if pe != nil || pid == 0 {
			continue
		}
		p, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
		if err != nil {
			continue
		}
		var buf [windows.MAX_PATH]uint16
		var size uint32 = uint32(len(buf))
		ie := windows.QueryFullProcessImageName(p, 0, &buf[0], &size)
		windows.CloseHandle(p)
		if ie != nil {
			continue
		}
		base := filepath.Base(windows.UTF16ToString(buf[:size]))
		if strings.HasPrefix(strings.ToLower(base), "discord") {
			pipes = append(pipes, name)
		}
	}
	return pipes
}

func (a *App) discordReadLoop(name string, conn net.Conn) {
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, err := conn.Read(tmp)
		if err != nil {
			a.discordRemoveConn(name, conn)
			return
		}
		buf = append(buf, tmp[:n]...)
		for len(buf) >= 8 {
			opcode := int32(binary.LittleEndian.Uint32(buf[0:4]))
			length := int32(binary.LittleEndian.Uint32(buf[4:8]))
			if length < 0 || length > discordMaxFrame {
				a.discordRemoveConn(name, conn)
				return
			}
			if int(length)+8 > len(buf) {
				break
			}
			buf = buf[8+int(length):]
			if opcode == discordOpClose {
				a.discordRemoveConn(name, conn)
				return
			}
		}
	}
}

func (a *App) discordRemoveConn(name string, conn net.Conn) {
	a.discordIpcMu.Lock()
	if a.discordConns[name] == conn {
		delete(a.discordConns, name)
	}
	a.discordIpcMu.Unlock()
	conn.Close()
}

func (a *App) discordSend(conn net.Conn, opcode int32, payload []byte) error {
	a.discordIpcMu.Lock()
	defer a.discordIpcMu.Unlock()
	if conn == nil {
		return fmt.Errorf("discord: not connected")
	}
	buf := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint32(buf[0:4], uint32(opcode))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(payload)))
	copy(buf[8:], payload)
	if _, err := conn.Write(buf); err != nil {
		log.Printf("Discord RPC send error: %v", err)
		return err
	}
	return nil
}

func (a *App) discordClose() {
	a.discordIpcMu.Lock()
	conns := make([]net.Conn, 0, len(a.discordConns))
	for _, c := range a.discordConns {
		conns = append(conns, c)
	}
	a.discordConns = map[string]net.Conn{}
	a.discordIpcMu.Unlock()
	for _, c := range conns {
		c.Close()
	}
}

func (a *App) SetDiscordActivity(details, state string) {
	a.discordMu.Lock()
	if a.discordDetails == details && a.discordState == state {
		a.discordMu.Unlock()
		return
	}
	a.discordDetails = details
	a.discordState = state
	a.discordMu.Unlock()
	a.refreshDiscordActivity()
}

func (a *App) refreshDiscordActivity() error {
	a.discordIpcMu.Lock()
	conns := make([]net.Conn, 0, len(a.discordConns))
	for _, c := range a.discordConns {
		conns = append(conns, c)
	}
	a.discordIpcMu.Unlock()
	if len(conns) == 0 {
		return nil
	}

	a.discordMu.Lock()
	details, state := a.discordDetails, a.discordState
	a.discordMu.Unlock()

	frame := discordFrame{
		Cmd: "SET_ACTIVITY",
		Args: discordActivityArgs{
			Pid: os.Getpid(),
			Activity: discordActivity{
				Details: details,
				State:   state,
				Timestamps: &discordTimestamps{
					Start: a.discordStart.Unix(),
				},
				Assets: discordAssets{
					LargeImage: discordLargeImage,
					LargeText:  "MEW",
				},
				Buttons: []discordButton{
					{Label: "View on GitHub", Url: discordGitHubURL},
					{Label: "Join our Discord", Url: discordInviteURL},
				},
			},
		},
		Nonce: discordNonce(),
	}
	payload, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	for _, c := range conns {
		a.discordSend(c, discordOpFrame, payload)
	}
	return nil
}

func discordNonce() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}

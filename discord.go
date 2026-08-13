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
	"time"

	"gopkg.in/natefinch/npipe.v2"
)

const (
	discordClientID     = "1513804565366177882"
	discordLargeImage   = "mew_logo"
	discordGitHubURL    = "https://github.com/KyarottoOwO/mew"
	discordInviteURL    = "https://discord.gg/AA8MSTDjB"
	discordDialTimeout  = 2 * time.Second
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
	Cmd   string                `json:"cmd"`
	Args  discordActivityArgs   `json:"args"`
	Nonce string                `json:"nonce"`
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

		if !a.discordConnected.Load() {
			err := a.discordConnect()
			if err == nil {
				a.discordConnected.Store(true)
				a.refreshDiscordActivity()
			} else {
				log.Printf("Discord RPC connect failed: %v", err)
			}
			time.Sleep(5 * time.Second)
			continue
		}

		if a.discordDead.Load() {
			a.discordClose()
			time.Sleep(5 * time.Second)
			continue
		}
		time.Sleep(5 * time.Second)
	}
}

// discordConnect dials the Discord IPC named pipe, sends the handshake and
// starts a reader goroutine that drains all incoming frames. The reader also
// detects CLOSE frames and read errors, flagging the connection as dead.
func (a *App) discordConnect() error {
	var conn net.Conn
	var err error
	for i := 0; i < 10; i++ {
		addr := fmt.Sprintf(`\\.\pipe\discord-ipc-%d`, i)
		conn, err = npipe.DialTimeout(addr, discordDialTimeout)
		if err == nil {
			break
		}
	}
	if err != nil {
		return err
	}

	a.discordIpcMu.Lock()
	a.discordConn = conn
	a.discordIpcMu.Unlock()
	a.discordDead.Store(false)

	go a.discordReadLoop(conn)

	payload, err := json.Marshal(discordHandshake{V: "1", ClientID: discordClientID})
	if err != nil {
		a.discordClose()
		return err
	}
	if err := a.discordSend(discordOpHandshake, payload); err != nil {
		a.discordClose()
		return err
	}
	return nil
}

// discordReadLoop reads frames continuously from the IPC socket so no leftover
// bytes ever accumulate in the pipe, and flags the connection as dead when
// Discord closes it (opcode 2) or the pipe errors.
func (a *App) discordReadLoop(conn net.Conn) {
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, err := conn.Read(tmp)
		if err != nil {
			a.discordDead.Store(true)
			return
		}
		buf = append(buf, tmp[:n]...)
		for len(buf) >= 8 {
			opcode := int32(binary.LittleEndian.Uint32(buf[0:4]))
			length := int32(binary.LittleEndian.Uint32(buf[4:8]))
			if length < 0 || length > discordMaxFrame {
				a.discordDead.Store(true)
				return
			}
			if int(length)+8 > len(buf) {
				break
			}
			buf = buf[8+int(length):]
			if opcode == discordOpClose {
				a.discordDead.Store(true)
				return
			}
		}
	}
}

// discordSend writes a single framed payload (opcode + length + JSON) to the
// IPC socket. All socket writes are serialized by discordIpcMu.
func (a *App) discordSend(opcode int32, payload []byte) error {
	a.discordIpcMu.Lock()
	defer a.discordIpcMu.Unlock()
	conn := a.discordConn
	if conn == nil {
		return fmt.Errorf("discord: not connected")
	}
	buf := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint32(buf[0:4], uint32(opcode))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(payload)))
	copy(buf[8:], payload)
	_, err := conn.Write(buf)
	if err != nil {
		log.Printf("Discord RPC send error: %v", err)
	}
	return err
}

// discordClose tears down the connection and clears any presence. It is
// idempotent and safe to call from any goroutine.
func (a *App) discordClose() {
	a.discordIpcMu.Lock()
	conn := a.discordConn
	a.discordConn = nil
	a.discordIpcMu.Unlock()
	if conn != nil {
		conn.Close()
	}
	a.discordDead.Store(false)
	a.discordConnected.Store(false)
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
	if !a.discordConnected.Load() {
		return nil
	}
	if a.discordDead.Load() {
		return fmt.Errorf("discord: connection lost")
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
	if err := a.discordSend(discordOpFrame, payload); err != nil {
		a.discordDead.Store(true)
		log.Printf("Discord RPC send error: %v", err)
		return err
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

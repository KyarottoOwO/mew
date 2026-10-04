package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Uploaded-pack recolor sessions live outside mew\temp so that the temp wipe on
// shutdown (and ClearCache) can never destroy in-progress work.
const (
	sessionMaxAge   = 7 * 24 * time.Hour
	sessionMaxCount = 20
)

func mewDataDir() string {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		local = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
	}
	return filepath.Join(local, "mew")
}

func (a *App) getSessionsDir() string {
	return filepath.Join(mewDataDir(), "sessions")
}

type SessionMeta struct {
	Id        string   `json:"id"`
	PackName  string   `json:"packName"`
	Folders   []string `json:"folders"`
	CreatedAt int64    `json:"createdAt"`
	UpdatedAt int64    `json:"updatedAt"`
}

func sanitizeSessionId(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || strings.Contains(id, "..") || strings.ContainsAny(id, `\/:`) {
		return "", fmt.Errorf("invalid session id")
	}
	return id, nil
}

func (a *App) sessionDir(id string) (string, error) {
	safe, err := sanitizeSessionId(id)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(a.getSessionsDir(), safe)
	st, statErr := os.Stat(dir)
	if statErr != nil || !st.IsDir() {
		return "", fmt.Errorf("session not found: %s", safe)
	}
	return dir, nil
}

func (a *App) sessionMetaPath(id string) string {
	return filepath.Join(a.getSessionsDir(), id+".json")
}

func (a *App) writeSessionMeta(meta SessionMeta) {
	if err := os.MkdirAll(a.getSessionsDir(), 0755); err != nil {
		return
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(a.sessionMetaPath(meta.Id), data, 0644)
}

func (a *App) readSessionMeta(id string) (SessionMeta, error) {
	data, err := os.ReadFile(a.sessionMetaPath(id))
	if err != nil {
		return SessionMeta{Id: id}, err
	}
	var meta SessionMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return SessionMeta{Id: id}, err
	}
	if meta.Id == "" {
		meta.Id = id
	}
	return meta, nil
}

// setSessionMeta records what the session holds.
func (a *App) setSessionMeta(id, packName string, folders []string) {
	safe, err := sanitizeSessionId(id)
	if err != nil {
		return
	}
	meta, err := a.readSessionMeta(safe)
	if err != nil {
		meta = SessionMeta{Id: safe, CreatedAt: time.Now().UnixMilli()}
	}
	meta.PackName = packName
	meta.Folders = folders
	meta.UpdatedAt = time.Now().UnixMilli()
	a.writeSessionMeta(meta)
}

// touchSession bumps a session's updated time so pruneStaleSessions does not
// drop a session the user is actively editing.
func (a *App) touchSession(id string) {
	safe, err := sanitizeSessionId(id)
	if err != nil {
		return
	}
	meta, err := a.readSessionMeta(safe)
	if err != nil {
		return
	}
	meta.UpdatedAt = time.Now().UnixMilli()
	a.writeSessionMeta(meta)
}

func (a *App) createSessionDir() (string, error) {
	id := uuid.NewString()
	dir := filepath.Join(a.getSessionsDir(), id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	now := time.Now().UnixMilli()
	a.writeSessionMeta(SessionMeta{Id: id, CreatedAt: now, UpdatedAt: now})
	return id, nil
}

func (a *App) DeleteSession(id string) error {
	safe, err := sanitizeSessionId(id)
	if err != nil {
		return err
	}
	os.Remove(a.sessionMetaPath(safe))
	return os.RemoveAll(filepath.Join(a.getSessionsDir(), safe))
}

// ListSessions returns every session that still has a working directory on
// disk, newest first.
func (a *App) ListSessions() []SessionMeta {
	root := a.getSessionsDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		return []SessionMeta{}
	}

	var out []SessionMeta
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		meta, err := a.readSessionMeta(e.Name())
		if err != nil {
			meta = SessionMeta{Id: e.Name()}
			if info, statErr := e.Info(); statErr == nil {
				meta.UpdatedAt = info.ModTime().UnixMilli()
				meta.CreatedAt = meta.UpdatedAt
			}
		}
		out = append(out, meta)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out
}

// pruneStaleSessions drops sessions that have not been touched recently and
// caps the total, so autosaved work survives restarts without growing forever.
func (a *App) pruneStaleSessions() {
	root := a.getSessionsDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}

	now := time.Now()
	kept := 0
	var orphans []string

	for _, e := range entries {
		path := filepath.Join(root, e.Name())
		if !e.IsDir() {
			if strings.HasSuffix(e.Name(), ".json") {
				orphans = append(orphans, path)
			}
			continue
		}

		meta, metaErr := a.readSessionMeta(e.Name())
		updated := time.UnixMilli(meta.UpdatedAt)
		if metaErr != nil {
			if info, statErr := e.Info(); statErr == nil {
				updated = info.ModTime()
			}
		}
		if now.Sub(updated) > sessionMaxAge {
			a.logDebug(fmt.Sprintf("pruneStaleSessions: removing stale session %s", e.Name()))
			os.RemoveAll(path)
			os.Remove(a.sessionMetaPath(e.Name()))
			continue
		}
		kept++
	}

	// metadata files whose working directory is gone
	for _, orphan := range orphans {
		id := strings.TrimSuffix(filepath.Base(orphan), ".json")
		if _, err := a.sessionDir(id); err != nil {
			os.Remove(orphan)
		}
	}

	if kept > sessionMaxCount {
		sessions := a.ListSessions()
		for _, meta := range sessions[sessionMaxCount:] {
			a.DeleteSession(meta.Id)
		}
	}
}

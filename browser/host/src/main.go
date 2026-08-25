package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxNativeMessage = 1024 * 1024

type BrowserTab struct {
	TabID      int    `json:"tab_id"`
	WindowID   int    `json:"window_id"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	Domain     string `json:"domain"`
	FavIconURL string `json:"fav_icon_url,omitempty"`
	Active     bool   `json:"active"`
	Audible    bool   `json:"audible"`
	Pinned     bool   `json:"pinned"`
	Discarded  bool   `json:"discarded"`
}

type Snapshot struct {
	Type       string       `json:"type"`
	Version    int          `json:"version"`
	Browser    string       `json:"browser"`
	CapturedAt string       `json:"captured_at"`
	Tabs       []BrowserTab `json:"tabs"`
}

type StoredBrowser struct {
	CapturedAt string       `json:"captured_at"`
	Tabs       []BrowserTab `json:"tabs"`
}

type State struct {
	Version   int                      `json:"version"`
	UpdatedAt string                   `json:"updated_at"`
	Browsers  map[string]StoredBrowser `json:"browsers"`
}

func main() {
	var snapshot Snapshot
	if err := readNativeMessage(os.Stdin, &snapshot); err != nil {
		_ = writeNativeMessage(os.Stdout, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if err := saveSnapshot(snapshot); err != nil {
		_ = writeNativeMessage(os.Stdout, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	_ = writeNativeMessage(os.Stdout, map[string]any{"ok": true, "tabs": len(snapshot.Tabs)})
}

func readNativeMessage(r io.Reader, out any) error {
	var size uint32
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return fmt.Errorf("mensagem não recebida: %w", err)
	}
	if size == 0 || size > maxNativeMessage {
		return fmt.Errorf("tamanho de mensagem inválido: %d", size)
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	return json.Unmarshal(buf, out)
}

func writeNativeMessage(w io.Writer, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(raw) > maxNativeMessage {
		return errors.New("resposta muito grande")
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(len(raw))); err != nil {
		return err
	}
	_, err = w.Write(raw)
	return err
}

func saveSnapshot(snapshot Snapshot) error {
	if snapshot.Type != "tabs.snapshot" {
		return fmt.Errorf("tipo de mensagem não permitido: %s", snapshot.Type)
	}
	browser := strings.ToLower(strings.TrimSpace(snapshot.Browser))
	if browser != "opera" && browser != "chrome" && browser != "edge" {
		browser = "chrome"
	}
	if len(snapshot.Tabs) > 500 {
		snapshot.Tabs = snapshot.Tabs[:500]
	}
	for i := range snapshot.Tabs {
		snapshot.Tabs[i].Title = truncate(strings.TrimSpace(snapshot.Tabs[i].Title), 500)
		snapshot.Tabs[i].URL = truncate(strings.TrimSpace(snapshot.Tabs[i].URL), 2048)
		snapshot.Tabs[i].Domain = truncate(strings.TrimSpace(snapshot.Tabs[i].Domain), 255)
		snapshot.Tabs[i].FavIconURL = truncate(strings.TrimSpace(snapshot.Tabs[i].FavIconURL), 4096)
	}
	captured := strings.TrimSpace(snapshot.CapturedAt)
	if captured == "" {
		captured = time.Now().UTC().Format(time.RFC3339)
	}
	path, err := statePath()
	if err != nil {
		return err
	}
	state := State{Version: 1, Browsers: map[string]StoredBrowser{}}
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &state)
		if state.Browsers == nil {
			state.Browsers = map[string]StoredBrowser{}
		}
	}
	state.Version = 1
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	state.Browsers[browser] = StoredBrowser{CapturedAt: captured, Tabs: snapshot.Tabs}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func statePath() (string, error) {
	local := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if local == "" {
		return "", errors.New("LOCALAPPDATA não disponível")
	}
	return filepath.Join(local, "CoreTuner", "Browser", "browser-tabs.json"), nil
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

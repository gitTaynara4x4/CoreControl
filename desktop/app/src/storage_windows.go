//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (a *App) loadHistory() {
	b, err := os.ReadFile(filepath.Join(dataDir(), "history.json"))
	if err == nil {
		json.Unmarshal(b, &a.history)
	}
}
func dataDir() string { return configuredDataDir() }

func sessionPath() string { return filepath.Join(dataDir(), "session.json") }
func (a *App) saveSession() {
	os.MkdirAll(dataDir(), 0755)
	a.mu.RLock()
	s := Session{ServerURL: a.serverURL, AccessToken: a.token, User: a.user, Company: a.company, SavedAt: time.Now()}
	a.mu.RUnlock()
	b, _ := json.MarshalIndent(s, "", "  ")
	_ = os.WriteFile(sessionPath(), b, 0600)
}
func (a *App) loadSession() {
	b, err := os.ReadFile(sessionPath())
	if err != nil {
		return
	}
	var s Session
	if json.Unmarshal(b, &s) != nil || s.AccessToken == "" {
		return
	}
	a.serverURL = s.ServerURL
	a.token = s.AccessToken
	a.user = s.User
	a.company = s.Company
	a.centralOK = false
	setText(a.controls[idServer], a.serverURL)
	a.hideAuth()
	go a.refreshCentralStatus()
}
func loadServerURL() string {
	b, err := os.ReadFile(filepath.Join(dataDir(), "server-url.txt"))
	if err == nil && strings.TrimSpace(string(b)) != "" {
		return strings.TrimRight(strings.TrimSpace(string(b)), "/")
	}
	if v := strings.TrimSpace(os.Getenv("CORETUNER_SERVER_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultServerURL
}
func saveServerURL(v string) {
	os.MkdirAll(dataDir(), 0755)
	_ = os.WriteFile(filepath.Join(dataDir(), "server-url.txt"), []byte(v), 0600)
}
func companyName(c *Company) string {
	if c == nil || strings.TrimSpace(c.Name) == "" {
		return "Empresa"
	}
	return c.Name
}

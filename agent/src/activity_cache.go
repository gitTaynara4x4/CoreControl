package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const activityCacheFreshFor = 20 * time.Second

type activityCacheEnvelope struct {
	UpdatedAt string                 `json:"updated_at"`
	Snapshot  activitySnapshotResult `json:"snapshot"`
}

var (
	activityCacheMu   sync.RWMutex
	activityCacheFile string
)

func setActivityCachePath(path string) {
	activityCacheMu.Lock()
	activityCacheFile = strings.TrimSpace(path)
	activityCacheMu.Unlock()
}

func getActivityCachePath() string {
	activityCacheMu.RLock()
	defer activityCacheMu.RUnlock()
	return activityCacheFile
}

func collectForegroundActivityForAgent() foregroundActivity {
	if snapshot, ok := readFreshActivitySnapshot(); ok {
		return snapshot.Foreground
	}
	return collectForegroundActivity()
}

func collectActivitySnapshotForAgent() activitySnapshotResult {
	if snapshot, ok := readFreshActivitySnapshot(); ok {
		return snapshot
	}
	return collectActivitySnapshot()
}

func readFreshActivitySnapshot() (activitySnapshotResult, bool) {
	path := getActivityCachePath()
	if path == "" {
		return activitySnapshotResult{}, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return activitySnapshotResult{}, false
	}
	var envelope activityCacheEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return activitySnapshotResult{}, false
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(envelope.UpdatedAt))
	if err != nil || time.Since(updatedAt) > activityCacheFreshFor || time.Since(updatedAt) < -time.Minute {
		return activitySnapshotResult{}, false
	}
	return envelope.Snapshot, true
}

func runSessionActivityHelper(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("caminho do cache de atividade não informado")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}

	// A sessão interativa é a única capaz de enxergar janelas, abas e ícones do
	// usuário. Ela não fala diretamente com o servidor: publica um snapshot local
	// e executa, quando solicitado, apenas comandos que precisam do contexto do usuário.
	go runSessionCommandWorker(path)
	for {
		snapshot := collectActivitySnapshot()
		envelope := activityCacheEnvelope{
			UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
			Snapshot:  snapshot,
		}
		// Uma falha transitória de arquivo não deve matar o helper. O próximo ciclo
		// tenta publicar novamente e o serviço continua enviando telemetria básica.
		_ = writeActivityCache(path, envelope)
		time.Sleep(4 * time.Second)
	}
}

func writeActivityCache(path string, envelope activityCacheEnvelope) error {
	raw, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, raw, 0600); err != nil {
		return err
	}
	if err := os.Rename(temp, path); err == nil {
		return nil
	}
	// No Windows, Rename não substitui um destino existente. Fazemos a troca
	// curta para evitar que o serviço leia JSON parcialmente escrito.
	_ = os.Remove(path)
	return os.Rename(temp, path)
}

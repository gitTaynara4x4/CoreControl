//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type sessionCommandRequest struct {
	Nonce     string         `json:"nonce"`
	CreatedAt string         `json:"created_at"`
	ExpiresAt string         `json:"expires_at"`
	Command   pendingCommand `json:"command"`
}

type sessionCommandResult struct {
	Nonce       string                 `json:"nonce"`
	CompletedAt string                 `json:"completed_at"`
	OK          bool                   `json:"ok"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

func executeAgentCommandViaSession(command pendingCommand) (map[string]interface{}, error, bool) {
	if !isAgentServiceExecution() || !sessionCommandNeedsInteractiveUser(command.Type) {
		return nil, nil, false
	}

	// Activity já é publicada continuamente pelo helper; não há motivo para criar
	// uma ida e volta de comando apenas para ler o snapshot atual.
	if command.Type == "activity.snapshot" {
		if snapshot, ok := readFreshActivitySnapshot(); ok {
			result, err := mapFromStruct(snapshot)
			return result, err, true
		}
		return nil, nil, false
	}

	if _, ok := readFreshActivitySnapshot(); !ok {
		// Sem sessão interativa disponível, deixa o executor local tentar. Isso
		// mantém comandos de máquina funcionais mesmo na tela de login.
		return nil, nil, false
	}

	// A partir do momento em que o pedido foi entregue ao helper, nunca o executa
	// novamente no serviço. Isso evita duplicar uma instalação/otimização quando a
	// resposta demora ou a sessão desaparece no meio do comando.
	result, err := dispatchCommandToSessionHelper(command)
	return result, err, true
}

func sessionCommandNeedsInteractiveUser(commandType string) bool {
	switch strings.TrimSpace(commandType) {
	case "activity.snapshot",
		"updates.scan",
		"updates.install",
		"optimization.diagnose",
		"optimization.cleanup_temp",
		"optimization.apply":
		return true
	default:
		return false
	}
}

func sessionCommandTimeout(commandType string) time.Duration {
	switch strings.TrimSpace(commandType) {
	case "updates.install":
		return 30 * time.Minute
	case "updates.scan":
		return 5 * time.Minute
	case "optimization.apply", "optimization.cleanup_temp", "optimization.diagnose":
		return 10 * time.Minute
	default:
		return 30 * time.Second
	}
}

func dispatchCommandToSessionHelper(command pendingCommand) (map[string]interface{}, error) {
	cachePath := getActivityCachePath()
	if strings.TrimSpace(cachePath) == "" {
		return nil, errors.New("helper de sessão não configurado")
	}
	directory := filepath.Dir(cachePath)
	requestPath := filepath.Join(directory, "session-command.json")
	resultPath := filepath.Join(directory, "session-result.json")
	timeout := sessionCommandTimeout(command.Type)
	now := time.Now().UTC()
	nonce := fmt.Sprintf("%d-%d", command.ID, now.UnixNano())
	request := sessionCommandRequest{
		Nonce:     nonce,
		CreatedAt: now.Format(time.RFC3339Nano),
		ExpiresAt: now.Add(timeout).Format(time.RFC3339Nano),
		Command:   command,
	}
	_ = os.Remove(resultPath)
	if err := writeSessionJSONAtomic(requestPath, request); err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(requestPath) }()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(resultPath)
		if err == nil {
			var response sessionCommandResult
			if json.Unmarshal(raw, &response) == nil && response.Nonce == nonce {
				_ = os.Remove(resultPath)
				if !response.OK {
					if strings.TrimSpace(response.Error) == "" {
						response.Error = "o helper da sessão não conseguiu executar o comando"
					}
					return response.Result, errors.New(response.Error)
				}
				return response.Result, nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return nil, fmt.Errorf("o helper da sessão não respondeu ao comando %s dentro do prazo", command.Type)
}

func runSessionCommandWorker(activityCachePath string) {
	directory := filepath.Dir(activityCachePath)
	requestPath := filepath.Join(directory, "session-command.json")
	resultPath := filepath.Join(directory, "session-result.json")
	for {
		raw, err := os.ReadFile(requestPath)
		if err != nil {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		var request sessionCommandRequest
		if json.Unmarshal(raw, &request) != nil || strings.TrimSpace(request.Nonce) == "" {
			_ = os.Remove(requestPath)
			time.Sleep(300 * time.Millisecond)
			continue
		}
		if expires, err := time.Parse(time.RFC3339Nano, request.ExpiresAt); err != nil || time.Now().After(expires) {
			_ = os.Remove(requestPath)
			continue
		}

		// Remove antes de executar para garantir que um loop rápido nunca dispare o
		// mesmo comando duas vezes.
		_ = os.Remove(requestPath)
		result, execErr := executeAgentCommandLocal(request.Command)
		response := sessionCommandResult{
			Nonce:       request.Nonce,
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
			OK:          execErr == nil,
			Result:      result,
		}
		if execErr != nil {
			response.Error = execErr.Error()
		}
		_ = writeSessionJSONAtomic(resultPath, response)
	}
}

func writeSessionJSONAtomic(path string, value interface{}) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, raw, 0600); err != nil {
		return err
	}
	_ = os.Remove(path)
	return os.Rename(temp, path)
}

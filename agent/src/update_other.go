//go:build !windows

package main

import (
	"errors"
	"fmt"
)

func executeAgentCommand(command pendingCommand) (map[string]interface{}, error) {
	if command.Type == "power.shutdown" {
		return nil, errors.New("desligamento real ainda não é suportado neste sistema operacional")
	}
	if command.Type == "power.wake_peer" {
		return executeWakePeerCommand(command)
	}
	return nil, fmt.Errorf("o comando %q só está disponível no Windows", command.Type)
}

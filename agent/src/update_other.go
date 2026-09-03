//go:build !windows

package main

import "fmt"

func executeAgentCommand(command pendingCommand) (map[string]interface{}, error) {
	if command.Type == "power.wake_peer" {
		return executeWakePeerCommand(command)
	}
	return nil, fmt.Errorf("o comando %q só está disponível no Windows", command.Type)
}

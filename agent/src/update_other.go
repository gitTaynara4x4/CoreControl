//go:build !windows

package main

import "fmt"

func executeAgentCommand(command pendingCommand) (map[string]interface{}, error) {
	return nil, fmt.Errorf("o comando %q só está disponível no Windows", command.Type)
}

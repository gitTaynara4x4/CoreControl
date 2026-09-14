package main

import "sync/atomic"

var agentServiceExecution atomic.Bool

func setAgentServiceExecution(value bool) {
	agentServiceExecution.Store(value)
}

func isAgentServiceExecution() bool {
	return agentServiceExecution.Load()
}

func executeAgentCommand(command pendingCommand) (map[string]interface{}, error) {
	if result, err, handled := executeAgentCommandViaSession(command); handled {
		return result, err
	}
	return executeAgentCommandLocal(command)
}

//go:build !windows

package main

func executeAgentCommandViaSession(command pendingCommand) (map[string]interface{}, error, bool) {
	_ = command
	return nil, nil, false
}

func runSessionCommandWorker(activityCachePath string) {
	_ = activityCachePath
}

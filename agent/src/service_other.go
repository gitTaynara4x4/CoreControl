//go:build !windows

package main

import "errors"

func runWindowsAgentService(run func(stop <-chan struct{})) error {
	_ = run
	return errors.New("serviço do Windows indisponível neste sistema")
}

func installWindowsAgentService(configPath, activityCachePath string) error {
	_, _ = configPath, activityCachePath
	return errors.New("instalação de serviço do Windows indisponível neste sistema")
}

func uninstallWindowsAgentService() error {
	return nil
}

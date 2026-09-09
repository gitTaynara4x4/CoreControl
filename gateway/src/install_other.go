//go:build !windows

package main

import (
	"fmt"
	"os"
)

func installGateway(token, serverURL string) error {
	cfg := Config{ServerURL: serverURL, EnrollmentToken: token}
	return saveConfig(gatewayConfigPath(), cfg)
}

func uninstallGateway() error {
	return os.RemoveAll(gatewayConfigPath())
}

func showMessage(title, message string, isError bool) {
	fmt.Printf("%s: %s\n", title, message)
}

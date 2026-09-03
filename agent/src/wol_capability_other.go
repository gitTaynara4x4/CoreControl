//go:build !windows

package main

func collectWOLCapability(primaryMAC string) wolCapability {
	return wolCapability{
		Checked: false,
		Reason:  "Diagnóstico Wake-on-LAN automático disponível apenas no Windows.",
	}
}

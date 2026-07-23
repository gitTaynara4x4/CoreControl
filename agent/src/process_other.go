//go:build !windows

package main

import "fmt"

func acquireSingleInstance(name string) (func(), error) { return func() {}, nil }
func collectWindowsSnapshotNative() (MachineSnapshot, error) {
	return MachineSnapshot{}, fmt.Errorf("coleta nativa do Windows indisponível")
}

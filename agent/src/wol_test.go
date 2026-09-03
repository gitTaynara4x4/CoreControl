package main

import "testing"

func TestSendWakeOnLANRejectsInvalidMAC(t *testing.T) {
	if _, err := sendWakeOnLAN("not-a-mac"); err == nil {
		t.Fatal("expected invalid MAC to be rejected")
	}
}

func TestLocalBroadcastAddressesAlwaysHasLimitedBroadcast(t *testing.T) {
	values := localBroadcastAddresses()
	for _, value := range values {
		if value == "255.255.255.255" {
			return
		}
	}
	t.Fatal("255.255.255.255 broadcast address missing")
}

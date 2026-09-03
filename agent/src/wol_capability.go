package main

type wolCapability struct {
	Checked              bool   `json:"checked"`
	CheckedAt            string `json:"checked_at,omitempty"`
	AdapterName          string `json:"adapter_name,omitempty"`
	InterfaceDescription string `json:"interface_description,omitempty"`
	MACAddress           string `json:"mac_address,omitempty"`
	LinkType             string `json:"link_type,omitempty"`
	MagicPacketSupported bool   `json:"magic_packet_supported"`
	MagicPacketEnabled   bool   `json:"magic_packet_enabled"`
	WakeProgrammable     bool   `json:"wake_programmable"`
	WakeArmed            bool   `json:"wake_armed"`
	S5DriverHint         bool   `json:"s5_driver_hint"`
	IntelAMTDetected     bool   `json:"intel_amt_detected"`
	AutoConfigured       bool   `json:"auto_configured"`
	WindowsPrepared      bool   `json:"windows_prepared"`
	FirmwareNeedsCheck   bool   `json:"firmware_needs_check"`
	Reason               string `json:"reason,omitempty"`
	Error                string `json:"error,omitempty"`
}

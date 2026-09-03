package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"
)

type wakePeerPayload struct {
	MACAddress string `json:"mac_address"`
}

func executeWakePeerCommand(command pendingCommand) (map[string]interface{}, error) {
	var payload wakePeerPayload
	if len(command.Payload) > 0 {
		if err := json.Unmarshal(command.Payload, &payload); err != nil {
			return nil, fmt.Errorf("payload de Wake-on-LAN inválido: %w", err)
		}
	}
	mac := strings.TrimSpace(payload.MACAddress)
	if mac == "" {
		return nil, errors.New("endereço MAC não informado")
	}
	result, err := sendWakeOnLAN(mac)
	if err != nil {
		return result, err
	}
	return result, nil
}

func sendWakeOnLAN(macText string) (map[string]interface{}, error) {
	mac, err := net.ParseMAC(strings.TrimSpace(macText))
	if err != nil || len(mac) != 6 {
		return nil, errors.New("endereço MAC inválido para Wake-on-LAN")
	}

	packet := make([]byte, 6+16*len(mac))
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}
	for i := 0; i < 16; i++ {
		copy(packet[6+i*len(mac):], mac)
	}

	broadcasts := localBroadcastAddresses()
	if len(broadcasts) == 0 {
		broadcasts = []string{"255.255.255.255"}
	}

	ports := []int{9, 7}
	sent := 0
	var lastErr error
	for round := 0; round < 2; round++ {
		for _, host := range broadcasts {
			for _, port := range ports {
				address := &net.UDPAddr{IP: net.ParseIP(host), Port: port}
				conn, dialErr := net.DialUDP("udp4", nil, address)
				if dialErr != nil {
					lastErr = dialErr
					continue
				}
				_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
				_, writeErr := conn.Write(packet)
				_ = conn.Close()
				if writeErr != nil {
					lastErr = writeErr
					continue
				}
				sent++
			}
		}
		if round == 0 {
			time.Sleep(75 * time.Millisecond)
		}
	}

	result := map[string]interface{}{
		"mac_address":  strings.ToLower(mac.String()),
		"broadcasts":   broadcasts,
		"packets_sent": sent,
	}
	if sent == 0 {
		if lastErr == nil {
			lastErr = errors.New("nenhum pacote Wake-on-LAN foi enviado")
		}
		return result, fmt.Errorf("falha ao enviar Wake-on-LAN: %w", lastErr)
	}
	return result, nil
}

func localBroadcastAddresses() []string {
	seen := map[string]struct{}{"255.255.255.255": {}}
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				ipNet, ok := addr.(*net.IPNet)
				if !ok {
					continue
				}
				ip := ipNet.IP.To4()
				if ip == nil {
					continue
				}
				mask := ipNet.Mask
				ones, bits := mask.Size()
				if ones < 0 || bits != 32 || ones >= 32 {
					continue
				}
				broadcast := make(net.IP, net.IPv4len)
				for i := 0; i < net.IPv4len; i++ {
					broadcast[i] = ip[i] | ^mask[i]
				}
				seen[broadcast.String()] = struct{}{}
			}
		}
	}
	values := make([]string, 0, len(seen))
	for value := range seen {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

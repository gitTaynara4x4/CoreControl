//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type wolRouteProbePayload struct {
	MACAddress   string `json:"mac_address"`
	NetworkCIDR  string `json:"network_cidr"`
	ProbeToken   string `json:"probe_token"`
	ExternalPort int    `json:"external_port"`
}

type wolRouteConfirmPayload struct {
	ProbeToken   string `json:"probe_token"`
	Method       string `json:"method"`
	ExternalIP   string `json:"external_ip"`
	ExternalPort int    `json:"external_port"`
	InternalPort int    `json:"internal_port"`
	BroadcastIP  string `json:"broadcast_ip"`
	ControlURL   string `json:"control_url,omitempty"`
	ServiceType  string `json:"service_type,omitempty"`
	ProbeSent    bool   `json:"probe_sent"`
	ProbeError   string `json:"probe_error,omitempty"`
}

var wolRouteReceipts = struct {
	sync.Mutex
	values map[string]time.Time
}{values: map[string]time.Time{}}

type upnpService struct {
	ServiceType string `xml:"serviceType"`
	ControlURL  string `xml:"controlURL"`
}

type upnpDevice struct {
	Services []upnpService `xml:"serviceList>service"`
	Devices  []upnpDevice  `xml:"deviceList>device"`
}

type upnpRoot struct {
	Device upnpDevice `xml:"device"`
}

func executeWOLRouteProbeCommand(command pendingCommand) (map[string]interface{}, error) {
	var payload wolRouteProbePayload
	if len(command.Payload) > 0 {
		if err := json.Unmarshal(command.Payload, &payload); err != nil {
			return nil, fmt.Errorf("payload do teste de rota Wake-on-LAN inválido: %w", err)
		}
	}
	token := strings.TrimSpace(payload.ProbeToken)
	if len(token) < 12 || len(token) > 160 {
		return nil, errors.New("token do teste de rota inválido")
	}
	mac, err := net.ParseMAC(strings.TrimSpace(payload.MACAddress))
	if err != nil || len(mac) != 6 {
		return nil, errors.New("endereço MAC inválido para o teste de rota")
	}
	broadcastIP, err := broadcastFromCIDR(strings.TrimSpace(payload.NetworkCIDR))
	if err != nil {
		return nil, fmt.Errorf("não foi possível determinar o broadcast da rede: %w", err)
	}

	basePort := payload.ExternalPort
	if basePort < 40000 || basePort > 59990 {
		basePort = 40009
	}
	internalPort := basePort
	firewallRule := fmt.Sprintf("CoreControl Wake Route %d", internalPort)
	firewallAdded := addTemporaryUDPFirewallRule(firewallRule, internalPort)

	listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: internalPort})
	if err != nil {
		return nil, fmt.Errorf("não foi possível abrir a porta local do teste (%d/UDP): %w", internalPort, err)
	}
	go listenForRouteProbe(listener, token, 45*time.Second, firewallRule, firewallAdded)

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	controlURL, serviceType, location, err := discoverUPnPWANService(ctx)
	if err != nil {
		_ = listener.Close()
		return map[string]interface{}{
			"method":              "upnp_broadcast",
			"broadcast_ip":        broadcastIP,
			"internal_port":       internalPort,
			"probe_token":         token,
			"upnp_detected":       false,
			"firewall_rule_added": firewallAdded,
		}, fmt.Errorf("UPnP não disponível para criar a rota automática: %w", err)
	}

	externalIP, _ := upnpGetExternalIPAddress(ctx, controlURL, serviceType)
	actualPort := 0
	var mappingErr error
	for offset := 0; offset < 8; offset++ {
		candidate := basePort + offset
		if candidate > 59999 {
			candidate = 40000 + offset
		}
		mappingErr = upnpAddPortMapping(ctx, controlURL, serviceType, candidate, internalPort, broadcastIP)
		if mappingErr == nil {
			actualPort = candidate
			break
		}
	}
	if actualPort == 0 {
		_ = listener.Close()
		return map[string]interface{}{
			"method":        "upnp_broadcast",
			"broadcast_ip":  broadcastIP,
			"internal_port": internalPort,
			"probe_token":   token,
			"upnp_detected": true,
			"external_ip":   externalIP,
			"control_url":   controlURL,
			"service_type":  serviceType,
			"location":      location,
		}, fmt.Errorf("o roteador respondeu ao UPnP, mas não aceitou uma rota para o broadcast da LAN: %w", mappingErr)
	}

	return map[string]interface{}{
		"method":              "upnp_broadcast",
		"mac_address":         strings.ToLower(mac.String()),
		"broadcast_ip":        broadcastIP,
		"external_ip":         strings.TrimSpace(externalIP),
		"external_port":       actualPort,
		"internal_port":       internalPort,
		"probe_token":         token,
		"upnp_detected":       true,
		"mapping_created":     true,
		"firewall_rule_added": firewallAdded,
		"control_url":         controlURL,
		"service_type":        serviceType,
		"location":            location,
	}, nil
}

func executeWOLRouteProbeConfirmCommand(command pendingCommand) (map[string]interface{}, error) {
	var payload wolRouteConfirmPayload
	if len(command.Payload) > 0 {
		if err := json.Unmarshal(command.Payload, &payload); err != nil {
			return nil, fmt.Errorf("payload de confirmação de rota inválido: %w", err)
		}
	}
	token := strings.TrimSpace(payload.ProbeToken)
	if token == "" {
		return nil, errors.New("token de confirmação não informado")
	}
	if !payload.ProbeSent {
		message := strings.TrimSpace(payload.ProbeError)
		if message == "" {
			message = "a VPS não conseguiu enviar o pacote externo de validação"
		}
		return map[string]interface{}{
			"verified":      false,
			"method":        firstNonEmptyRoute(payload.Method, "upnp_broadcast"),
			"external_ip":   payload.ExternalIP,
			"external_port": payload.ExternalPort,
			"internal_port": payload.InternalPort,
			"broadcast_ip":  payload.BroadcastIP,
			"probe_token":   token,
		}, errors.New(message)
	}

	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		wolRouteReceipts.Lock()
		receivedAt, ok := wolRouteReceipts.values[token]
		if ok {
			delete(wolRouteReceipts.values, token)
		}
		wolRouteReceipts.Unlock()
		if ok {
			return map[string]interface{}{
				"verified":      true,
				"method":        firstNonEmptyRoute(payload.Method, "upnp_broadcast"),
				"external_ip":   payload.ExternalIP,
				"external_port": payload.ExternalPort,
				"internal_port": payload.InternalPort,
				"broadcast_ip":  payload.BroadcastIP,
				"probe_token":   token,
				"received_at":   receivedAt.UTC().Format(time.RFC3339),
			}, nil
		}
		time.Sleep(250 * time.Millisecond)
	}

	return map[string]interface{}{
		"verified":      false,
		"method":        firstNonEmptyRoute(payload.Method, "upnp_broadcast"),
		"external_ip":   payload.ExternalIP,
		"external_port": payload.ExternalPort,
		"internal_port": payload.InternalPort,
		"broadcast_ip":  payload.BroadcastIP,
		"probe_token":   token,
	}, errors.New("a VPS enviou o pacote de teste, mas ele não chegou ao PC pela rota UPnP")
}

func firstNonEmptyRoute(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func listenForRouteProbe(listener *net.UDPConn, token string, lifetime time.Duration, firewallRule string, firewallAdded bool) {
	defer listener.Close()
	if firewallAdded {
		defer removeTemporaryUDPFirewallRule(firewallRule)
	}
	_ = listener.SetReadDeadline(time.Now().Add(lifetime))
	buffer := make([]byte, 2048)
	for {
		count, _, err := listener.ReadFromUDP(buffer)
		if err != nil {
			return
		}
		if strings.TrimSpace(string(buffer[:count])) != token {
			continue
		}
		now := time.Now().UTC()
		wolRouteReceipts.Lock()
		// Limpa recibos antigos para que o mapa continue pequeno.
		for key, created := range wolRouteReceipts.values {
			if now.Sub(created) > 2*time.Minute {
				delete(wolRouteReceipts.values, key)
			}
		}
		wolRouteReceipts.values[token] = now
		wolRouteReceipts.Unlock()
		return
	}
}

func addTemporaryUDPFirewallRule(name string, port int) bool {
	if port < 40000 || port > 59999 {
		return false
	}
	command := exec.Command("netsh", "advfirewall", "firewall", "add", "rule", "name="+name, "dir=in", "action=allow", "protocol=UDP", "localport="+strconv.Itoa(port), "profile=any")
	return command.Run() == nil
}

func removeTemporaryUDPFirewallRule(name string) {
	if strings.TrimSpace(name) == "" {
		return
	}
	command := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", "name="+name)
	_ = command.Run()
}

func broadcastFromCIDR(value string) (string, error) {
	ip, network, err := net.ParseCIDR(value)
	if err != nil || ip.To4() == nil || network == nil {
		return "", errors.New("CIDR IPv4 inválido")
	}
	ip4 := ip.To4()
	mask := network.Mask
	ones, bits := mask.Size()
	if ones < 0 || bits != 32 || ones >= 31 {
		return "", errors.New("sub-rede não possui broadcast utilizável")
	}
	broadcast := make(net.IP, net.IPv4len)
	for index := 0; index < net.IPv4len; index++ {
		broadcast[index] = ip4[index] | ^mask[index]
	}
	return broadcast.String(), nil
}

func discoverUPnPWANService(ctx context.Context) (string, string, string, error) {
	locations, err := ssdpLocations(ctx)
	if err != nil && len(locations) == 0 {
		return "", "", "", err
	}
	client := &http.Client{Timeout: 4 * time.Second}
	var lastErr error
	for _, location := range locations {
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
		if requestErr != nil {
			lastErr = requestErr
			continue
		}
		response, requestErr := client.Do(request)
		if requestErr != nil {
			lastErr = requestErr
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 2<<20))
		response.Body.Close()
		if readErr != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
			if readErr != nil {
				lastErr = readErr
			} else {
				lastErr = fmt.Errorf("descrição UPnP respondeu HTTP %d", response.StatusCode)
			}
			continue
		}
		var root upnpRoot
		if unmarshalErr := xml.Unmarshal(body, &root); unmarshalErr != nil {
			lastErr = unmarshalErr
			continue
		}
		service, ok := findWANService(root.Device)
		if !ok {
			lastErr = errors.New("serviço WANIPConnection/WANPPPConnection não encontrado")
			continue
		}
		base, parseErr := url.Parse(location)
		if parseErr != nil {
			lastErr = parseErr
			continue
		}
		control, parseErr := url.Parse(strings.TrimSpace(service.ControlURL))
		if parseErr != nil {
			lastErr = parseErr
			continue
		}
		return base.ResolveReference(control).String(), strings.TrimSpace(service.ServiceType), location, nil
	}
	if lastErr == nil {
		lastErr = errors.New("nenhum gateway UPnP compatível respondeu")
	}
	return "", "", "", lastErr
}

func findWANService(device upnpDevice) (upnpService, bool) {
	for _, service := range device.Services {
		kind := strings.ToLower(service.ServiceType)
		if strings.Contains(kind, "wanipconnection") || strings.Contains(kind, "wanpppconnection") {
			return service, true
		}
	}
	for _, child := range device.Devices {
		if service, ok := findWANService(child); ok {
			return service, true
		}
	}
	return upnpService{}, false
}

func ssdpLocations(ctx context.Context) ([]string, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	deadline := time.Now().Add(3500 * time.Millisecond)
	_ = conn.SetDeadline(deadline)
	destination := &net.UDPAddr{IP: net.ParseIP("239.255.255.250"), Port: 1900}
	searches := []string{
		"urn:schemas-upnp-org:device:InternetGatewayDevice:1",
		"urn:schemas-upnp-org:service:WANIPConnection:1",
		"ssdp:all",
	}
	for _, searchTarget := range searches {
		payload := "M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\nST: " + searchTarget + "\r\n\r\n"
		_, _ = conn.WriteToUDP([]byte(payload), destination)
	}

	seen := map[string]struct{}{}
	locations := []string{}
	buffer := make([]byte, 65535)
	for {
		select {
		case <-ctx.Done():
			if len(locations) > 0 {
				return locations, nil
			}
			return locations, ctx.Err()
		default:
		}
		count, _, readErr := conn.ReadFromUDP(buffer)
		if readErr != nil {
			if len(locations) > 0 {
				return locations, nil
			}
			return locations, readErr
		}
		headers := parseSSDPHeaders(string(buffer[:count]))
		location := strings.TrimSpace(headers["location"])
		if location == "" {
			continue
		}
		if _, exists := seen[location]; exists {
			continue
		}
		seen[location] = struct{}{}
		locations = append(locations, location)
		if len(locations) >= 8 {
			return locations, nil
		}
	}
}

func parseSSDPHeaders(raw string) map[string]string {
	headers := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		if index := strings.Index(line, ":"); index > 0 {
			key := strings.ToLower(strings.TrimSpace(line[:index]))
			value := strings.TrimSpace(line[index+1:])
			headers[key] = value
		}
	}
	return headers
}

func upnpGetExternalIPAddress(ctx context.Context, controlURL string, serviceType string) (string, error) {
	body := `<u:GetExternalIPAddress xmlns:u="` + xmlEscape(serviceType) + `"></u:GetExternalIPAddress>`
	response, err := upnpSOAP(ctx, controlURL, serviceType, "GetExternalIPAddress", body)
	if err != nil {
		return "", err
	}
	decoder := xml.NewDecoder(bytes.NewReader(response))
	for {
		token, decodeErr := decoder.Token()
		if decodeErr != nil {
			break
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "NewExternalIPAddress" {
			continue
		}
		var value string
		if decodeErr = decoder.DecodeElement(&value, &start); decodeErr == nil {
			return strings.TrimSpace(value), nil
		}
	}
	return "", errors.New("UPnP não devolveu o IP externo")
}

func upnpAddPortMapping(ctx context.Context, controlURL string, serviceType string, externalPort int, internalPort int, internalClient string) error {
	body := `<u:AddPortMapping xmlns:u="` + xmlEscape(serviceType) + `">` +
		`<NewRemoteHost></NewRemoteHost>` +
		`<NewExternalPort>` + strconv.Itoa(externalPort) + `</NewExternalPort>` +
		`<NewProtocol>UDP</NewProtocol>` +
		`<NewInternalPort>` + strconv.Itoa(internalPort) + `</NewInternalPort>` +
		`<NewInternalClient>` + xmlEscape(internalClient) + `</NewInternalClient>` +
		`<NewEnabled>1</NewEnabled>` +
		`<NewPortMappingDescription>CoreControl Wake-on-LAN</NewPortMappingDescription>` +
		`<NewLeaseDuration>0</NewLeaseDuration>` +
		`</u:AddPortMapping>`
	_, err := upnpSOAP(ctx, controlURL, serviceType, "AddPortMapping", body)
	return err
}

func upnpSOAP(ctx context.Context, controlURL string, serviceType string, action string, actionBody string) ([]byte, error) {
	envelope := `<?xml version="1.0" encoding="utf-8"?>` +
		`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">` +
		`<s:Body>` + actionBody + `</s:Body></s:Envelope>`
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, controlURL, strings.NewReader(envelope))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	request.Header.Set("SOAPAction", `"`+serviceType+`#`+action+`"`)
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if readErr != nil {
		return nil, readErr
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		text := strings.TrimSpace(string(body))
		if len(text) > 600 {
			text = text[:600]
		}
		return nil, fmt.Errorf("UPnP %s respondeu HTTP %d: %s", action, response.StatusCode, text)
	}
	return body, nil
}

func xmlEscape(value string) string {
	var buffer bytes.Buffer
	_ = xml.EscapeText(&buffer, []byte(value))
	return buffer.String()
}

//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"unsafe"
)

func (a *App) server() (string, error) {
	raw := strings.TrimRight(strings.TrimSpace(getText(a.controls[idServer])), "/")
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", errors.New("Informe um endereço válido do CoreControl")
	}
	host := strings.ToLower(strings.Split(u.Host, ":")[0])
	local := u.Scheme == "http" && (host == "127.0.0.1" || host == "localhost" || host == "::1")
	if u.Scheme != "https" && !local {
		return "", errors.New("HTTPS é obrigatório fora do teste local")
	}
	a.serverURL = raw
	saveServerURL(raw)
	return raw, nil
}
func (a *App) openPasswordRecovery() {
	server, err := a.server()
	if err != nil {
		message("CoreControl", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	target := server + "/?forgot=1"
	procShellExecute.Call(uintptr(a.hwnd), uintptr(unsafe.Pointer(utf16("open"))), uintptr(unsafe.Pointer(utf16(target))), 0, 0, SW_SHOWNORMAL)
}

func (a *App) login() {
	server, err := a.server()
	if err != nil {
		message("CoreControl", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	email := strings.TrimSpace(getText(a.controls[idEmail]))
	pass := getText(a.controls[idPassword])
	if email == "" || pass == "" {
		message("Preencha seus dados", "Informe seu e-mail e sua senha.", MB_OK|MB_ICONERROR)
		return
	}
	a.setBusy(true, "Entrando na empresa...")
	var resp AuthResponse
	err = a.request("POST", server+"/api/auth/login", map[string]string{"email": email, "password": pass}, "", &resp)
	a.setBusy(false, "")
	if err != nil {
		message("Não foi possível entrar", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	a.applyAuth(resp)
	a.addHistory("Login realizado", fmt.Sprintf("Empresa: %s", companyName(resp.Company)))
}
func (a *App) register() {
	p := map[string]string{
		"company_name":          strings.TrimSpace(getText(a.controls[idCompany])),
		"responsible_name":      strings.TrimSpace(getText(a.controls[idResponsible])),
		"email":                 strings.TrimSpace(getText(a.controls[idRegEmail])),
		"password":              getText(a.controls[idRegPassword]),
		"password_confirmation": getText(a.controls[idRegConfirm]),
	}
	if p["company_name"] == "" || p["responsible_name"] == "" || p["email"] == "" || p["password"] == "" || p["password_confirmation"] == "" {
		message("Preencha todos os campos", "Informe os dados da empresa, responsável, e-mail e senha.", MB_OK|MB_ICONERROR)
		return
	}
	if !strings.Contains(p["email"], "@") || !strings.Contains(strings.TrimPrefix(p["email"], "@"), ".") {
		message("E-mail inválido", "Informe um endereço de e-mail válido.", MB_OK|MB_ICONERROR)
		return
	}
	if len([]rune(p["password"])) < 10 {
		message("Senha muito curta", "Use uma senha com pelo menos 10 caracteres.", MB_OK|MB_ICONERROR)
		return
	}
	if p["password"] != p["password_confirmation"] {
		message("As senhas não coincidem", "Digite a mesma senha nos campos Senha e Confirmar senha.", MB_OK|MB_ICONERROR)
		return
	}
	server, err := a.server()
	if err != nil {
		message("CoreControl", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	a.setBusy(true, "Criando empresa...")
	var resp AuthResponse
	err = a.request("POST", server+"/api/auth/register-company", p, "", &resp)
	a.setBusy(false, "")
	if err != nil {
		message("Cadastro não concluído", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	a.applyAuth(resp)
	a.addHistory("Empresa criada", companyName(resp.Company))
	message("CoreControl", "Empresa criada com sucesso.", MB_OK|MB_ICONINFORMATION)
}
func (a *App) applyAuth(resp AuthResponse) {
	a.mu.Lock()
	a.token = resp.AccessToken
	a.user = resp.User
	a.company = resp.Company
	a.centralOK = true
	a.statusText = "Conectado ao CoreControl"
	a.mu.Unlock()
	a.hideAuth()
	a.saveSession()
	go a.refreshCentralStatus()
	a.invalidate()
}
func (a *App) logout() {
	a.mu.Lock()
	a.token = ""
	a.centralOK = false
	a.company = nil
	a.page = 0
	a.statusText = "Sessão encerrada"
	a.mu.Unlock()
	os.Remove(sessionPath())
	setText(a.controls[idPassword], "")
	a.showLogin("login")
}

func (a *App) setBusy(v bool, text string) {
	a.mu.Lock()
	a.busy = v
	if text != "" {
		a.statusText = text
	}
	a.mu.Unlock()
	enable(a.controls[idLogin], !v)
	enable(a.controls[idRegister], !v)
	a.invalidate()
}
func (a *App) request(method, endpoint string, payload any, token string, out any) error {
	var body io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return errors.New("Não foi possível conectar ao CoreControl. Verifique se o servidor está disponível e tente novamente.")
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New(friendlyAPIError(b, resp.StatusCode))
	}
	if out != nil && len(b) > 0 {
		if err = json.Unmarshal(b, out); err != nil {
			return fmt.Errorf("resposta inválida do servidor")
		}
	}
	return nil
}
func friendlyAPIError(body []byte, statusCode int) string {
	var payload map[string]any
	if json.Unmarshal(body, &payload) == nil {
		detail := payload["detail"]
		switch value := detail.(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return value
			}
		case []any:
			for _, raw := range value {
				item, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				field := ""
				if loc, ok := item["loc"].([]any); ok && len(loc) > 0 {
					if name, ok := loc[len(loc)-1].(string); ok {
						field = name
					}
				}
				typeName, _ := item["type"].(string)
				label := map[string]string{
					"password":              "A senha",
					"password_confirmation": "A confirmação da senha",
					"email":                 "O e-mail",
					"company_name":          "O nome da empresa",
					"responsible_name":      "O nome do responsável",
				}[field]
				if label == "" {
					label = "O campo informado"
				}
				if typeName == "string_too_short" {
					minimum := 0
					if ctx, ok := item["ctx"].(map[string]any); ok {
						if n, ok := ctx["min_length"].(float64); ok {
							minimum = int(n)
						}
					}
					if minimum > 0 {
						return fmt.Sprintf("%s deve ter pelo menos %d caracteres.", label, minimum)
					}
					return label + " é muito curto."
				}
				if typeName == "missing" {
					return label + " é obrigatório."
				}
			}
		}
	}
	switch statusCode {
	case http.StatusUnauthorized:
		return "E-mail ou senha inválidos."
	case http.StatusConflict:
		return "Já existe um cadastro com esses dados."
	case http.StatusUnprocessableEntity:
		return "Verifique os dados informados e tente novamente."
	default:
		return "Não foi possível concluir a solicitação. Tente novamente."
	}
}

func (a *App) refreshCentralStatus() {
	a.mu.RLock()
	token, server := a.token, a.serverURL
	a.mu.RUnlock()
	if token == "" {
		return
	}
	var me MeResponse
	if err := a.request("GET", server+"/api/auth/me", nil, token, &me); err != nil {
		a.mu.Lock()
		a.centralOK = false
		a.statusText = "Falha ao verificar a sessão da Central: " + err.Error()
		a.mu.Unlock()
		a.invalidate()
		return
	}
	a.mu.Lock()
	a.centralOK = true
	a.user = AuthUser{ID: me.ID, Name: me.Name, Email: me.Email, Role: me.Role, CompanyID: me.CompanyID}
	a.company = me.Company
	a.statusText = "Conectado ao CoreControl"
	a.mu.Unlock()
	a.saveSession()
	a.invalidate()
}

func (a *App) refreshLocal(full bool) {
	a.mu.RLock()
	base := a.sys
	currentProcesses := append([]ProcessInfo(nil), a.processes...)
	page := a.page
	a.mu.RUnlock()
	var sys SystemInfo
	if full || base.Hostname == "" {
		sys = collectSystemFull(a.client, a.serverURL)
	} else {
		sys = collectSystemDynamic(base)
	}
	procs := currentProcesses
	if full || page == 4 || len(procs) == 0 {
		procs = collectProcesses()
	}
	a.mu.Lock()
	a.sys = sys
	a.processes = procs
	if full && base.Hostname == "" {
		a.statusText = "Diagnóstico local concluído"
	}
	a.mu.Unlock()
	a.invalidate()
}

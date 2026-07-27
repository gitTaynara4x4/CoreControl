# Design do instalador CoreTuner

A tela nativa do `CoreTunerSetup.exe` foi separada para facilitar futuras alterações visuais sem tocar na lógica de instalação.

## Onde alterar

- `src/main.go`
  - Função `buildUI`: textos, posições, larguras e alturas dos campos.
  - Função `createFonts`: tamanhos e pesos das fontes.
  - Não altere os IDs dos controles (`idLoginEmail`, `idInstall`, etc.).

- `src/theme_windows.go`
  - Cores do fundo, cartão, linha azul e botões.
  - Botões primários: `idLoginButton`, `idRegisterButton`, `idInstall`.
  - Botões secundários e recuperação de senha também são desenhados aqui.

- `../assets/coretuner-logo.png`
  - Arquivo visual da logo completa para edição.

- `../assets/coretuner-logo.bmp`
  - Arquivo incorporado no instalador durante a compilação.

## Recompilar

Na pasta `desktop`, execute:

```powershell
.\Build_Windows.ps1
```

O executável atualizado será criado em:

```text
app/downloads/CoreTunerSetup.exe
```

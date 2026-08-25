# Assinatura e Microsoft Defender

A correção 0.4.2 remove o formato de autoextração e ações agressivas da instalação anterior. Ela não tenta burlar ou desativar o antivírus.

Para produção:

1. Obtenha um certificado de assinatura de código de uma autoridade confiável, ou use uma solução oficial de assinatura compatível com Windows.
2. Assine e aplique carimbo de tempo nos três arquivos: `CoreControlSetup.exe`, `CoreControl.exe` e `CoreControlAgent.exe`.
3. Verifique a assinatura com `signtool verify /pa /v arquivo.exe`.
4. Publique sempre versões assinadas pela mesma identidade.
5. Caso o Defender detecte uma versão limpa, envie o arquivo como desenvolvedor no portal Microsoft Security Intelligence e aguarde o resultado final.

Não oriente clientes a desativar o Defender ou criar exclusões permanentes.

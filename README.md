# CoreControl — favicons nas abas do navegador (v9)

Esta correção melhora a lista de abas abertas no painel do CoreControl.

O que foi alterado:
- cada aba do navegador agora tenta mostrar o favicon real do site;
- exemplos: Gmail, YouTube, Google Maps, Instagram, Google Imagens, Segware Cloud;
- se o site não tiver favicon acessível, o sistema faz fallback automático para o ícone do navegador;
- localhost, páginas internas e domínios sem favicon continuam com o ícone do Chrome/Edge/Opera.

Observações:
- esta mudança é somente no painel/Central;
- não exige reinstalar o Agent no PC da Luiza, desde que ela já esteja na versão 0.8.6;
- depois de substituir os arquivos na Central/VPS, reinicie a Central e faça Ctrl+F5 no navegador.

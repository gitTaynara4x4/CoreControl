# CoreControl — favicons das abas corrigidos (v9.1)

Correções:
- aceita o campo real `fav_icon_url` produzido pelo Browser Bridge;
- quando a aba veio pelo Windows sem URL/domínio, reconhece serviços comuns pelo título
  (Gmail, Google Maps, Google Imagens, Google Agenda, Contatos, YouTube, Instagram,
  Segware Cloud, WhatsApp, LinkedIn, GitHub etc.) e mostra o favicon correspondente;
- aba do CoreControl usa a própria marca local;
- se o favicon falhar, volta automaticamente para o ícone real do navegador;
- cache-bust do `devices.js` atualizado no `index.html`.

Esta correção é somente painel/Central. Não exige reinstalar o Agent da Luiza.
Depois de substituir os arquivos na Central/VPS, reinicie a aplicação e recarregue a página.

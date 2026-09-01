# CoreControl v10.4 — correção do 404 em Reinstalar / atualizar

Este patch deve ser aplicado **por cima da v10.3**.

Corrige a regressão em que `app/api.py` da v10.3 removeu o endpoint:

`POST /api/devices/{device_id}/reinstall-token`

Mantém ao mesmo tempo:
- reinstalação direcionada do mesmo computador, sem duplicar o dispositivo;
- vínculo do token à máquina original;
- preservação de nome/setor/local na reinstalação direcionada;
- correção v10.3 do Mesh Agent por empresa;
- download autenticado de `/api/agent/remote-agent`;
- consulta de `/api/agent/remote-status`;
- limpeza do vínculo remoto antigo para o Setup substituir pelo agente da empresa correta.

Arquivos alterados:
- `app/api.py`
- `app/models.py`
- `app/db.py`

Depois de aplicar, faça Force Rebuild do serviço CoreControl.
Não é necessário apagar o computador da Luiza nem clicar em Adicionar computador.
Após o rebuild, abra o dispositivo existente e clique em **Reinstalar / atualizar CoreControl**.

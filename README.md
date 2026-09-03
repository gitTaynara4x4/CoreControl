# CoreControl v10.10.1 — nome amigável do computador

Correção da Visão Geral e do fluxo de reinstalação para separar o nome escolhido pela empresa do hostname técnico do Windows.

## Alterações
- A Visão Geral mostra `device.name` como identificação principal.
- O hostname aparece apenas como `Nome técnico: ...` em texto secundário.
- A área `Em foco agora` segue a mesma regra.
- Reinstalar/atualizar o CoreControl não substitui mais um nome amigável já cadastrado pelo hostname enviado pelo Setup.
- Re-enrollment do Agent também preserva o nome amigável.
- Setor/local existentes deixam de ser apagados quando a reinstalação não envia novos valores.

## Observação
Se uma reinstalação anterior já substituiu o nome amigável pelo hostname, renomeie o computador uma única vez em `Editar computador`. A partir desta versão, novas reinstalações preservam esse nome.

(function () {
  'use strict';
  const CT = window.CoreTuner;

  const sections = {
    general: ['Geral', 'Preferências da plataforma e da organização.'],
    permissions: ['Usuários e permissões', 'Regras de acesso por perfil administrativo.'],
    alerts: ['Alertas', 'Limites, severidades e notificações.'],
    security: ['Segurança', 'Sessão, autenticação e ações críticas.'],
    integrations: ['Integrações', 'E-mail, acesso remoto, API e serviços externos.'],
    appearance: ['Aparência', 'Tema e identidade visual do painel.'],
    agent: ['Agente CoreControl', 'Versão, heartbeat, telemetria e implantação.'],
    audit: ['Auditoria', 'Retenção e eventos administrativos registrados.'],
  };

  function general() {
    return `<div class="settings-grid"><div class="card module-section"><div class="card-header"><div><h2>Plataforma</h2><p>Preferências gerais.</p></div></div><div class="settings-rows"><div><span>Nome exibido</span><strong>CoreControl</strong></div><div><span>Idioma</span><strong>Português (Brasil)</strong></div><div><span>Fuso horário</span><strong>Configuração do servidor</strong></div><div><span>Computador offline</span><strong>Regra atual do monitoramento</strong></div></div></div><div class="card module-section"><div class="card-header"><div><h2>Suporte</h2><p>Dados apresentados aos clientes.</p></div></div><div class="module-empty compact"><strong>Configuração preparada</strong><span>E-mail, telefone e identificação do suporte entram aqui.</span></div></div></div>`;
  }

  function permissions() {
    const rows = [
      ['Ver todas as empresas', '✓', '✓', 'Própria', 'Própria'],
      ['Editar empresa', '✓', '✓', 'Própria', '—'],
      ['Excluir empresa definitivamente', '✓', '—', '—', '—'],
      ['Acesso remoto', '✓', '✓', 'Conforme permissão', '✓'],
      ['Scripts', '✓', '✓', 'Conforme permissão', '✓'],
    ];
    return `<div class="card module-section"><div class="card-header"><div><h2>Matriz de permissões</h2><p>Base visual dos perfis do CoreControl.</p></div></div><div class="table-wrap"><table><thead><tr><th>Permissão</th><th>Administrador Global</th><th>Plataforma</th><th>Empresa</th><th>Técnico</th></tr></thead><tbody>${rows.map((r) => `<tr>${r.map((c, i) => `<td>${i === 0 ? `<strong>${c}</strong>` : c}</td>`).join('')}</tr>`).join('')}</tbody></table></div></div>`;
  }

  function alerts() {
    const items = [['CPU alta', '90%', 'Aviso'], ['Memória alta', '90%', 'Aviso'], ['Disco crítico', '10% livre', 'Crítico'], ['Computador offline', 'Regra atual', 'Crítico'], ['Temperatura', 'Quando disponível', 'Aviso']];
    return `<div class="card module-section"><div class="card-header"><div><h2>Regras de alerta</h2><p>Estrutura para limites e severidades.</p></div><button class="btn primary" data-setting-action="Nova regra">Nova regra</button></div><div class="settings-rule-list">${items.map(([name, limit, severity]) => `<div><span><strong>${name}</strong><small>${limit}</small></span><span class="pill ${severity === 'Crítico' ? 'critical' : 'warning'}">${severity}</span><button class="btn small" data-setting-action="Editar ${name}">Editar</button></div>`).join('')}</div></div>`;
  }

  function security() {
    return `<div class="module-feature-grid"><article class="module-feature"><div><strong>Sessões</strong><p>Tempo máximo de sessão e sessões ativas.</p></div><span>Configurar</span></article><article class="module-feature"><div><strong>Política de senha</strong><p>Tamanho mínimo, complexidade e expiração.</p></div><span>Configurar</span></article><article class="module-feature"><div><strong>Ações críticas</strong><p>Nova autenticação para exclusões e operações em massa.</p></div><span>Recomendado</span></article><article class="module-feature"><div><strong>Tentativas de login</strong><p>Bloqueio e acompanhamento de falhas de autenticação.</p></div><span>Configurar</span></article></div>`;
  }

  function integrations() {
    return `<div class="module-feature-grid"><article class="module-feature"><div><strong>E-mail</strong><p>SMTP, remetente e teste de envio.</p></div><span>Configurar</span></article><article class="module-feature"><div><strong>Acesso remoto</strong><p>MeshCentral, status da integração e conexão.</p></div><span>Ativo no projeto</span></article><article class="module-feature"><div><strong>API</strong><p>Tokens, webhooks e integrações externas.</p></div><span>Configurar</span></article><article class="module-feature"><div><strong>Mensageria</strong><p>Espaço para WhatsApp, Teams e Slack.</p></div><span>Futuro</span></article></div>`;
  }

  function appearance() {
    const current = document.documentElement.dataset.theme || 'light';
    return `<div class="card module-section"><div class="card-header"><div><h2>Aparência</h2><p>A engrenagem do sidebar continua como atalho rápido.</p></div></div><div class="appearance-choice-grid"><button type="button" class="appearance-choice ${current === 'light' ? 'active' : ''}" data-settings-theme="light"><span class="appearance-preview light"><i></i><b></b></span><strong>Claro</strong><small>Interface clara</small></button><button type="button" class="appearance-choice ${current === 'dark' ? 'active' : ''}" data-settings-theme="dark"><span class="appearance-preview dark"><i></i><b></b></span><strong>Escuro</strong><small>Interface escura</small></button></div><div class="settings-note"><strong>Identidade CoreControl</strong><span>Logo e favicon atuais continuam sendo usados em todos os temas.</span></div></div>`;
  }

  function agent() {
    return `<div class="settings-grid"><div class="card module-section"><div class="card-header"><div><h2>Agente</h2><p>Políticas de comunicação do CoreControlAgent.</p></div></div><div class="settings-rows"><div><span>Versão mínima</span><strong>Gerenciada pelo servidor</strong></div><div><span>Heartbeat</span><strong>Configuração atual</strong></div><div><span>Telemetria</span><strong>Ativa</strong></div><div><span>Acesso remoto</span><strong>Conforme empresa</strong></div></div></div><div class="card module-section"><div class="card-header"><div><h2>Implantação</h2><p>Instalação de novos computadores.</p></div></div><div class="module-action-stack"><button class="btn primary" data-go="companies">Gerenciar por empresa</button><button class="btn" data-setting-action="Baixar instalador">Baixar instalador</button><button class="btn" data-setting-action="Gerar token">Gerar token</button></div></div></div>`;
  }

  function audit() {
    return `<div class="card module-section"><div class="card-header"><div><h2>Política de auditoria</h2><p>Defina retenção e categorias de eventos administrativos.</p></div><button class="btn" data-go="reports">Abrir relatórios</button></div><div class="settings-rule-list"><div><span><strong>Alterações administrativas</strong><small>Empresas, usuários e computadores</small></span><span class="pill resolved">Registrar</span></div><div><span><strong>Operações críticas</strong><small>Exclusão, scripts e acesso remoto</small></span><span class="pill resolved">Registrar</span></div><div><span><strong>Retenção</strong><small>Política configurável</small></span><span class="pill">Definir</span></div></div></div>`;
  }

  const renderers = { general, permissions, alerts, security, integrations, appearance, agent, audit };

  CT.registerPage('settings', async function renderSettings() {
    await CT.mountPage('settings');
    const page = CT.$('.page-settings');
    const view = CT.$('#settingsView');
    const render = (tab) => {
      CT.$$('[data-module-tab]', page).forEach((button) => button.classList.toggle('active', button.dataset.moduleTab === tab));
      const [title, description] = sections[tab] || sections.general;
      view.innerHTML = `<div class="module-context"><div><span>CONFIGURAÇÕES</span><strong>${title}</strong><p>${description}</p></div></div>${(renderers[tab] || general)()}`;
    };
    page.addEventListener('click', (event) => {
      const tab = event.target.closest('[data-module-tab]');
      if (tab) return render(tab.dataset.moduleTab);
      const theme = event.target.closest('[data-settings-theme]');
      if (theme) {
        window.CoreControlTheme?.apply(theme.dataset.settingsTheme, true);
        return render('appearance');
      }
      const go = event.target.closest('[data-go]');
      if (go) return CT.navigate(go.dataset.go);
      const action = event.target.closest('[data-setting-action]');
      if (action) CT.toast(`${action.dataset.settingAction}: estrutura pronta para implementação.`);
    });
    render('general');
  });
})();

(function () {
  'use strict';
  const CT = window.CoreTuner;

  const sections = {
    general: ['Geral', 'Preferências da plataforma, suporte e comportamento padrão.'],
    permissions: ['Permissões', 'Controle visual dos acessos por perfil administrativo.'],
    alerts: ['Alertas', 'Limites, severidades e comportamento das notificações.'],
    security: ['Segurança', 'Sessões, autenticação e proteção de ações críticas.'],
    integrations: ['Integrações', 'Serviços externos, acesso remoto, API e mensageria.'],
    appearance: ['Aparência', 'Tema e identidade visual do painel.'],
    agent: ['Agente', 'Comunicação, telemetria e implantação do CoreControl Agent.'],
    audit: ['Auditoria', 'Registro, retenção e rastreabilidade das ações administrativas.'],
  };

  function icon(name) {
    const icons = {
      settings: '<path d="M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7Z"/><path d="M19.4 15a1.7 1.7 0 0 0 .34 1.88l.06.06-2.12 2.12-.06-.06a1.7 1.7 0 0 0-1.88-.34 1.7 1.7 0 0 0-1.03 1.56V20.3h-3v-.08a1.7 1.7 0 0 0-1.03-1.56 1.7 1.7 0 0 0-1.88.34l-.06.06-2.12-2.12.06-.06A1.7 1.7 0 0 0 7 15a1.7 1.7 0 0 0-1.56-1.03H5.3v-3h.14A1.7 1.7 0 0 0 7 9.94a1.7 1.7 0 0 0-.34-1.88L6.6 8l2.12-2.12.06.06a1.7 1.7 0 0 0 1.88.34 1.7 1.7 0 0 0 1.03-1.56V4.6h3v.12a1.7 1.7 0 0 0 1.03 1.56 1.7 1.7 0 0 0 1.88-.34l.06-.06L19.8 8l-.06.06a1.7 1.7 0 0 0-.34 1.88 1.7 1.7 0 0 0 1.56 1.03h.14v3h-.14A1.7 1.7 0 0 0 19.4 15Z"/>',
      shield: '<path d="M12 3 5 6v5c0 4.8 2.9 8 7 10 4.1-2 7-5.2 7-10V6l-7-3Z"/><path d="m9 12 2 2 4-4"/>',
      bell: '<path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 7h18s-3 0-3-7"/><path d="M10 19a2 2 0 0 0 4 0"/>',
      link: '<path d="M10 13a5 5 0 0 0 7.1.1l2-2a5 5 0 0 0-7.1-7.1l-1.1 1.1"/><path d="M14 11a5 5 0 0 0-7.1-.1l-2 2A5 5 0 0 0 12 20l1.1-1.1"/>',
      palette: '<path d="M12 3a9 9 0 0 0 0 18h1.5a1.5 1.5 0 0 0 0-3H12a2 2 0 0 1 0-4h2.5A6.5 6.5 0 0 0 21 7.5C21 5 17 3 12 3Z"/><circle cx="7.5" cy="10.5" r="1"/><circle cx="9.5" cy="6.5" r="1"/><circle cx="14.5" cy="6.5" r="1"/><circle cx="17" cy="10" r="1"/>',
      agent: '<rect x="4" y="5" width="16" height="12" rx="2"/><path d="M8 21h8M12 17v4"/>',
      audit: '<path d="M9 5H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-3"/><path d="M16 3h5v5M21 3l-9 9"/><path d="M8 14h4M8 17h7"/>',
      users: '<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75"/>',
      check: '<path d="m5 12 4 4L19 6"/>',
      clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
      support: '<path d="M4 13a8 8 0 0 1 16 0"/><path d="M4 13v4a2 2 0 0 0 2 2h1v-7H6a2 2 0 0 0-2 1ZM20 13v4a2 2 0 0 1-2 2h-1v-7h1a2 2 0 0 1 2 1Z"/><path d="M17 19c0 1.1-.9 2-2 2h-3"/>',
      key: '<circle cx="8" cy="15" r="4"/><path d="m11 12 8-8M15 8l2 2M17 6l2 2"/>',
    };
    return `<svg viewBox="0 0 24 24" aria-hidden="true">${icons[name] || icons.settings}</svg>`;
  }

  function header(tab, meta = '') {
    const [title, description] = sections[tab] || sections.general;
    return `<div class="settings-status-card card">
      <div class="settings-status-main">
        <span class="settings-status-icon">${icon(tab === 'general' ? 'settings' : tab === 'permissions' ? 'users' : tab === 'alerts' ? 'bell' : tab === 'security' ? 'shield' : tab === 'integrations' ? 'link' : tab === 'appearance' ? 'palette' : tab === 'agent' ? 'agent' : 'audit')}</span>
        <div><span class="settings-eyebrow">CONFIGURAÇÕES</span><h2>${title}</h2><p>${description}</p></div>
      </div>
      ${meta ? `<div class="settings-status-meta">${meta}</div>` : ''}
    </div>`;
  }

  function valueRow(label, value, helper = '') {
    return `<div class="settings-value-row"><div><strong>${label}</strong>${helper ? `<small>${helper}</small>` : ''}</div><span>${value}</span></div>`;
  }

  function general() {
    return `${header('general', '<span><b>CoreControl</b> plataforma</span><span><b>pt-BR</b> idioma</span>')}
      <div class="settings-grid">
        <article class="card settings-panel">
          <div class="settings-panel-header"><div><h3>Plataforma</h3><p>Preferências gerais usadas em toda a organização.</p></div><span class="settings-panel-icon">${icon('settings')}</span></div>
          <div class="settings-value-list">
            ${valueRow('Nome exibido', 'CoreControl', 'Nome apresentado no painel e telas administrativas.')}
            ${valueRow('Idioma', 'Português (Brasil)', 'Idioma padrão da interface.')}
            ${valueRow('Fuso horário', 'Configuração do servidor', 'Aplicado a eventos, auditoria e agendamentos.')}
            ${valueRow('Computador offline', 'Regra do monitoramento', 'O status é calculado pelo heartbeat autenticado do Agent.')}
          </div>
        </article>
        <article class="card settings-panel">
          <div class="settings-panel-header"><div><h3>Suporte ao cliente</h3><p>Identificação que poderá ser apresentada dentro do CoreControl.</p></div><span class="settings-panel-icon neutral">${icon('support')}</span></div>
          <div class="settings-prepared-state"><span>${icon('check')}</span><div><strong>Estrutura preparada</strong><p>E-mail, telefone e identificação do suporte podem ser conectados ao backend quando você definir esses dados.</p></div></div>
          <button class="btn" type="button" data-setting-action="Configurar suporte">Configurar suporte</button>
        </article>
      </div>`;
  }

  function permissions() {
    const rows = [
      ['Ver todas as empresas', 'Permitido', 'Permitido', 'Própria empresa', 'Própria empresa'],
      ['Editar empresa', 'Permitido', 'Permitido', 'Própria empresa', 'Sem acesso'],
      ['Excluir empresa', 'Permitido', 'Sem acesso', 'Sem acesso', 'Sem acesso'],
      ['Acesso remoto', 'Permitido', 'Permitido', 'Conforme permissão', 'Permitido'],
      ['Executar scripts', 'Permitido', 'Permitido', 'Conforme permissão', 'Permitido'],
    ];
    return `${header('permissions', '<span><b>4</b> perfis</span><span><b>5</b> regras exibidas</span>')}
      <article class="card settings-table-card">
        <div class="settings-panel-header table-head"><div><h3>Matriz de permissões</h3><p>Visão consolidada dos principais acessos administrativos.</p></div><button class="btn primary" type="button" data-go="users">Gerenciar usuários</button></div>
        <div class="table-wrap settings-table-wrap"><table class="settings-table"><thead><tr><th>Permissão</th><th>Admin. Global</th><th>Plataforma</th><th>Empresa</th><th>Técnico</th></tr></thead><tbody>${rows.map((row) => `<tr>${row.map((cell, index) => `<td>${index === 0 ? `<strong>${cell}</strong>` : `<span class="settings-access ${cell.includes('Sem') ? 'denied' : ''}">${cell}</span>`}</td>`).join('')}</tr>`).join('')}</tbody></table></div>
      </article>`;
  }

  function alerts() {
    const items = [
      ['CPU alta', '90%', 'Aviso'],
      ['Memória alta', '90%', 'Aviso'],
      ['Disco crítico', '10% livre', 'Crítico'],
      ['Computador offline', 'Regra do monitoramento', 'Crítico'],
      ['Temperatura', 'Quando disponível', 'Aviso'],
    ];
    return `${header('alerts', '<span><b>5</b> regras</span><span><b>2</b> críticas</span>')}
      <article class="card settings-panel settings-panel-full">
        <div class="settings-panel-header"><div><h3>Regras de alerta</h3><p>Limites de referência e severidades usadas pela plataforma.</p></div><button class="btn primary" type="button" data-setting-action="Nova regra">Nova regra</button></div>
        <div class="settings-rule-list">${items.map(([name, limit, severity]) => `<div class="settings-rule-row"><div class="settings-rule-name"><span class="settings-rule-dot ${severity === 'Crítico' ? 'critical' : 'warning'}"></span><div><strong>${name}</strong><small>${limit}</small></div></div><span class="pill ${severity === 'Crítico' ? 'critical' : 'warning'}">${severity}</span><button class="btn small" type="button" data-setting-action="Editar ${name}">Editar</button></div>`).join('')}</div>
      </article>`;
  }

  function featureCard(iconName, title, text, status, action = 'Configurar') {
    return `<article class="settings-feature-card card"><span class="settings-feature-icon">${icon(iconName)}</span><div class="settings-feature-copy"><strong>${title}</strong><p>${text}</p></div><div class="settings-feature-foot"><span>${status}</span><button class="btn small" type="button" data-setting-action="${action} ${title}">${action}</button></div></article>`;
  }

  function security() {
    return `${header('security', '<span><b>Proteção</b> ativa</span>')}
      <div class="settings-feature-grid">
        ${featureCard('clock', 'Sessões', 'Tempo máximo de sessão e acompanhamento das sessões ativas.', 'Política do servidor')}
        ${featureCard('key', 'Política de senha', 'Tamanho mínimo, complexidade e regras de expiração.', 'Configuração pendente')}
        ${featureCard('shield', 'Ações críticas', 'Nova autenticação para exclusões e operações sensíveis.', 'Recomendado')}
        ${featureCard('audit', 'Tentativas de login', 'Bloqueio e acompanhamento de falhas de autenticação.', 'Auditoria disponível')}
      </div>`;
  }

  function integrations() {
    return `${header('integrations', '<span><b>1</b> integração ativa</span>')}
      <div class="settings-feature-grid">
        ${featureCard('support', 'E-mail', 'SMTP, remetente padrão e teste de envio.', 'Não configurado')}
        ${featureCard('link', 'Acesso remoto', 'MeshCentral, disponibilidade e conexão remota.', 'Ativo no projeto', 'Ver')}
        ${featureCard('key', 'API', 'Tokens, webhooks e integrações externas.', 'Estrutura preparada')}
        ${featureCard('bell', 'Mensageria', 'Canal futuro para WhatsApp, Teams ou Slack.', 'Planejado')}
      </div>`;
  }

  function appearance() {
    const current = document.documentElement.dataset.theme || 'light';
    return `${header('appearance', `<span><b>${current === 'dark' ? 'Escuro' : 'Claro'}</b> tema atual</span>`)}
      <article class="card settings-panel settings-panel-full">
        <div class="settings-panel-header"><div><h3>Tema da interface</h3><p>A alteração é aplicada imediatamente e salva no navegador.</p></div><span class="settings-panel-icon">${icon('palette')}</span></div>
        <div class="settings-theme-grid">
          <button type="button" class="settings-theme-card ${current === 'light' ? 'active' : ''}" data-settings-theme="light"><span class="settings-theme-preview light"><i></i><b></b><em></em></span><span><strong>Claro</strong><small>Interface clara e neutra</small></span><i class="settings-theme-check">${icon('check')}</i></button>
          <button type="button" class="settings-theme-card ${current === 'dark' ? 'active' : ''}" data-settings-theme="dark"><span class="settings-theme-preview dark"><i></i><b></b><em></em></span><span><strong>Escuro</strong><small>Visual escuro no estilo CoreControl</small></span><i class="settings-theme-check">${icon('check')}</i></button>
        </div>
        <div class="settings-note"><span>${icon('check')}</span><div><strong>Identidade preservada</strong><p>Logo, favicon e estrutura do CoreControl permanecem iguais nos dois temas.</p></div></div>
      </article>`;
  }

  function agent() {
    return `${header('agent', '<span><b>Telemetria</b> ativa</span>')}
      <div class="settings-grid">
        <article class="card settings-panel">
          <div class="settings-panel-header"><div><h3>CoreControl Agent</h3><p>Parâmetros gerais de comunicação dos computadores.</p></div><span class="settings-panel-icon">${icon('agent')}</span></div>
          <div class="settings-value-list">
            ${valueRow('Versão mínima', 'Gerenciada pelo servidor')}
            ${valueRow('Heartbeat', 'Configuração atual', 'Usado para confirmar online/offline e desligamento.')}
            ${valueRow('Telemetria', 'Ativa')}
            ${valueRow('Acesso remoto', 'Conforme empresa')}
          </div>
        </article>
        <article class="card settings-panel">
          <div class="settings-panel-header"><div><h3>Implantação</h3><p>Instalação e manutenção de computadores gerenciados.</p></div><span class="settings-panel-icon neutral">${icon('settings')}</span></div>
          <div class="settings-action-stack"><button class="btn primary" type="button" data-go="companies">Gerenciar por empresa</button><button class="btn" type="button" data-setting-action="Baixar instalador">Baixar instalador</button><button class="btn" type="button" data-setting-action="Gerar token">Gerar token</button></div>
        </article>
      </div>`;
  }

  function audit() {
    return `${header('audit', '<span><b>Rastreabilidade</b> habilitada</span>')}
      <article class="card settings-panel settings-panel-full">
        <div class="settings-panel-header"><div><h3>Política de auditoria</h3><p>Eventos administrativos que devem permanecer rastreáveis.</p></div><button class="btn" type="button" data-go="reports">Abrir relatórios</button></div>
        <div class="settings-rule-list">
          <div class="settings-rule-row"><div class="settings-rule-name"><span class="settings-rule-dot good"></span><div><strong>Alterações administrativas</strong><small>Empresas, usuários e computadores</small></div></div><span class="pill resolved">Registrar</span></div>
          <div class="settings-rule-row"><div class="settings-rule-name"><span class="settings-rule-dot good"></span><div><strong>Operações críticas</strong><small>Exclusão, scripts e acesso remoto</small></div></div><span class="pill resolved">Registrar</span></div>
          <div class="settings-rule-row"><div class="settings-rule-name"><span class="settings-rule-dot"></span><div><strong>Retenção</strong><small>Prazo de armazenamento dos eventos</small></div></div><span class="pill">Definir</span><button class="btn small" type="button" data-setting-action="Definir retenção">Configurar</button></div>
        </div>
      </article>`;
  }

  const renderers = { general, permissions, alerts, security, integrations, appearance, agent, audit };

  function tabFromUrl() {
    const url = new URL(window.location.href);
    const tab = url.searchParams.get('tab');
    return Object.prototype.hasOwnProperty.call(sections, tab) ? tab : 'general';
  }

  function writeTab(tab, mode = 'replace') {
    const url = new URL(window.location.href);
    url.searchParams.set('page', 'settings');
    if (tab === 'general') url.searchParams.delete('tab');
    else url.searchParams.set('tab', tab);
    const state = { ...(window.history.state || {}), corecontrol: true, page: 'settings', tab };
    if (mode === 'push') window.history.pushState(state, '', url);
    else window.history.replaceState(state, '', url);
  }

  CT.registerPage('settings', async function renderSettings() {
    await CT.mountPage('settings');
    const page = CT.$('.page-settings');
    const view = CT.$('#settingsView');

    const render = (tab, historyMode = 'replace') => {
      if (!Object.prototype.hasOwnProperty.call(sections, tab)) tab = 'general';
      CT.$$('[data-settings-tab]', page).forEach((button) => {
        const active = button.dataset.settingsTab === tab;
        button.classList.toggle('active', active);
        button.setAttribute('aria-selected', active ? 'true' : 'false');
      });
      view.innerHTML = (renderers[tab] || general)();
      writeTab(tab, historyMode);
    };

    page.addEventListener('click', (event) => {
      const tab = event.target.closest('[data-settings-tab]');
      if (tab) return render(tab.dataset.settingsTab, 'push');

      const theme = event.target.closest('[data-settings-theme]');
      if (theme) {
        window.CoreControlTheme?.apply(theme.dataset.settingsTheme, true);
        return render('appearance', 'replace');
      }

      const go = event.target.closest('[data-go]');
      if (go) return CT.navigate(go.dataset.go);

      const action = event.target.closest('[data-setting-action]');
      if (action) CT.toast(`${action.dataset.settingAction}: esta configuração ainda precisa ser conectada ao backend.`);
    });

    render(tabFromUrl(), 'replace');
  });
})();

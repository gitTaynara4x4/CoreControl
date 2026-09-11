// CoreControl v10.56 - estilos exclusivos da Visão Geral.
// Carrega um CSS separado para não alterar sidebar nem estilos globais.
(function ensureCoreControlOverviewV1056Styles(){
  const id = 'cc-overview-v1056-styles';
  if (document.getElementById(id)) return;
  const link = document.createElement('link');
  link.id = id;
  link.rel = 'stylesheet';
  link.href = '/static/overview-v10.56.css?v=20260911-2';
  document.head.appendChild(link);
})();

(function () {
  'use strict';

  const CT = window.CoreTuner;

  const ICONS = {
    monitor: '<svg viewBox="0 0 24 24"><rect x="3.5" y="4.5" width="17" height="12" rx="2"/><path d="M8 20h8M12 16.5V20"/></svg>',
    pulse: '<svg viewBox="0 0 24 24"><path d="M3 12h4l2-5 4 10 2-5h6"/></svg>',
    alert: '<svg viewBox="0 0 24 24"><path d="M12 4 3.8 18h16.4L12 4Z"/><path d="M12 9v4M12 16h.01"/></svg>',
    spark: '<svg viewBox="0 0 24 24"><path d="M12 3 9.8 9.8 3 12l6.8 2.2L12 21l2.2-6.8L21 12l-6.8-2.2L12 3Z"/></svg>',
    shield: '<svg viewBox="0 0 24 24"><path d="M12 3.5 19 6v5.4c0 4.3-2.7 7.3-7 9.1-4.3-1.8-7-4.8-7-9.1V6l7-2.5Z"/><path d="m8.8 12 2 2 4.4-4.4"/></svg>',
    clock: '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="8"/><path d="M12 7.5V12l3 2"/></svg>',
    remote: '<svg viewBox="0 0 24 24"><rect x="3.5" y="4.5" width="17" height="13" rx="2"/><path d="M8 21h8M12 17.5V21M14 8h3v3M17 8l-5.25 5.25"/></svg>',
    gauge: '<svg viewBox="0 0 24 24"><path d="M4 17a8 8 0 1 1 16 0"/><path d="m12 13 4-4"/><path d="M7 17h10"/></svg>',
    app: '<svg viewBox="0 0 24 24"><rect x="4" y="4" width="16" height="16" rx="3"/><path d="M8 8h8M8 12h5M8 16h3"/></svg>',
    update: '<svg viewBox="0 0 24 24"><path d="M20 11a8 8 0 1 0-2.35 5.65"/><path d="M20 4v7h-7"/></svg>',
    report: '<svg viewBox="0 0 24 24"><path d="M5 20V10h4v10M10 20V4h4v16M15 20v-7h4v7M3 20h18"/></svg>',
    disk: '<svg viewBox="0 0 24 24"><ellipse cx="12" cy="6" rx="7" ry="3"/><path d="M5 6v6c0 1.7 3.1 3 7 3s7-1.3 7-3V6M5 12v6c0 1.7 3.1 3 7 3s7-1.3 7-3v-6"/></svg>',
    temperature: '<svg viewBox="0 0 24 24"><path d="M10 5a2 2 0 1 1 4 0v8.2a4 4 0 1 1-4 0V5Z"/><path d="M12 8v7"/></svg>',
    memory: '<svg viewBox="0 0 24 24"><rect x="5" y="7" width="14" height="10" rx="2"/><path d="M8 4v3M12 4v3M16 4v3M8 17v3M12 17v3M16 17v3M2 10h3M2 14h3M19 10h3M19 14h3"/></svg>',
    chevron: '<svg viewBox="0 0 24 24"><path d="m9 6 6 6-6 6"/></svg>',
    check: '<svg viewBox="0 0 24 24"><path d="m5 12 4 4L19 6"/></svg>',
  };

  function icon(name) {
    return ICONS[name] || ICONS.monitor;
  }

  function cleanProfile(profile) {
    const value = String(profile || '').trim();
    return !value || value.toLowerCase() === 'nenhum' ? null : value;
  }

  function friendlyApp(name) {
    const value = String(name || '').trim();
    if (!value) return 'Sem janela em foco';
    const normalized = value.toLowerCase();
    const exact = {
      simnext: 'SIM Next',
      chrome: 'Google Chrome',
      msedge: 'Microsoft Edge',
      firefox: 'Mozilla Firefox',
      opera: 'Opera',
      opera_gx: 'Opera GX',
      whatsapp: 'WhatsApp',
      spotify: 'Spotify',
      anydesk: 'AnyDesk',
      excel: 'Microsoft Excel',
      winword: 'Microsoft Word',
      outlook: 'Microsoft Outlook',
      teams: 'Microsoft Teams',
      explorer: 'Explorador de Arquivos',
    };
    if (exact[normalized]) return exact[normalized];
    if (normalized.includes('valorant')) return 'VALORANT';
    if (normalized.includes('corecontrol')) return 'CoreControl';
    return value.replace(/[-_]+/g, ' ').replace(/\bwin64\b/gi, '').replace(/\bshipping\b/gi, '').replace(/\s+/g, ' ').trim();
  }

  function ago(value) {
    if (!value) return 'sem atualização';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return 'sem atualização';
    const seconds = Math.max(0, Math.round((Date.now() - date.getTime()) / 1000));
    if (seconds < 60) return `há ${seconds}s`;
    if (seconds < 3600) return `há ${Math.floor(seconds / 60)} min`;
    if (seconds < 86400) return `há ${Math.floor(seconds / 3600)} h`;
    return `há ${Math.floor(seconds / 86400)} d`;
  }

  function duration(seconds) {
    const value = Number(seconds || 0);
    if (!Number.isFinite(value) || value <= 0) return '—';
    const days = Math.floor(value / 86400);
    const hours = Math.floor((value % 86400) / 3600);
    const minutes = Math.floor((value % 3600) / 60);
    if (days) return `${days}d ${hours}h`;
    if (hours) return `${hours}h ${minutes}min`;
    return `${Math.max(1, minutes)}min`;
  }

  function tempInfo(telemetry) {
    if (!telemetry) return { value: '—', label: 'Temperatura' };
    if (telemetry.temperature_c != null) {
      return { value: `${CT.fmtNum(telemetry.temperature_c, 0)} °C`, label: telemetry.temperature_source === 'gpu' ? 'GPU' : 'Temperatura' };
    }
    if (telemetry.gpu_temperature_c != null) {
      return { value: `${CT.fmtNum(telemetry.gpu_temperature_c, 0)} °C`, label: 'GPU' };
    }
    return { value: '—', label: 'Temperatura' };
  }

  function healthLabel(score) {
    if (score >= 90) return 'Excelente';
    if (score >= 80) return 'Muito boa';
    if (score >= 70) return 'Boa';
    if (score >= 55) return 'Atenção';
    return 'Crítica';
  }

  function metricValue(value, suffix = '%', digits = 0) {
    return value == null ? '—' : `${CT.fmtNum(value, digits)}${suffix}`;
  }

  function deviceIssues(device) {
    const issues = [];
    const telemetry = device.telemetry || {};
    if (!device.online) {
      issues.push({ level: 'critical', title: 'Sem comunicação', text: `Último contato ${ago(device.last_seen)}` });
      return issues;
    }
    if (device.alerts_open > 0) issues.push({ level: 'critical', title: `${device.alerts_open} alerta${device.alerts_open === 1 ? '' : 's'} ativo${device.alerts_open === 1 ? '' : 's'}`, text: 'Há evento técnico aguardando avaliação.' });
    if (device.health_score < 70) issues.push({ level: 'warning', title: `Saúde ${device.health_score}/100`, text: 'O CoreControl detectou sinais que merecem análise.' });
    if (telemetry.memory_percent >= 90) issues.push({ level: 'warning', title: `Memória em ${CT.fmtNum(telemetry.memory_percent, 0)}%`, text: 'Uso de RAM elevado neste momento.' });
    if (telemetry.disk_percent >= 90) issues.push({ level: 'warning', title: `Disco em ${CT.fmtNum(telemetry.disk_percent, 0)}%`, text: 'Pouco espaço disponível no armazenamento.' });
    const temperature = telemetry.temperature_c ?? telemetry.gpu_temperature_c;
    if (temperature != null && temperature >= 80) issues.push({ level: 'warning', title: `Temperatura ${CT.fmtNum(temperature, 0)} °C`, text: 'Temperatura elevada detectada.' });
    if (telemetry.defender_active === false) issues.push({ level: 'critical', title: 'Proteção do Windows desativada', text: 'Microsoft Defender não está ativo.' });
    if (telemetry.firewall_active === false) issues.push({ level: 'critical', title: 'Firewall desativado', text: 'Firewall do Windows não está ativo.' });
    return issues;
  }

  function eventCopy(event) {
    const action = String(event?.action || '');
    const details = event?.details;
    let title = 'Atividade registrada';
    let tone = 'neutral';
    let subtitle = event?.device_name || 'CoreControl';

    if (action === 'remote.session.request') {
      title = 'Acesso remoto iniciado';
      tone = 'blue';
    } else if (action === 'optimization.apply.success') {
      const profile = details?.result?.active_profile_name;
      title = profile ? `${profile} aplicado com sucesso` : 'Otimização aplicada com sucesso';
      tone = 'green';
    } else if (action === 'optimization.apply.failed') {
      title = 'Falha ao aplicar otimização';
      tone = 'red';
    } else if (action === 'optimization.diagnose.success') {
      title = 'Diagnóstico inteligente concluído';
      tone = 'green';
    } else if (action === 'optimization.diagnose.failed') {
      title = 'Falha no diagnóstico inteligente';
      tone = 'red';
    } else if (action === 'optimization.cleanup_temp.success') {
      title = 'Limpeza segura concluída';
      tone = 'green';
    } else if (action === 'optimization.cleanup_temp.failed') {
      title = 'Falha na limpeza segura';
      tone = 'red';
    } else if (action === 'updates.install.success') {
      title = 'Atualizações instaladas';
      tone = 'green';
    } else if (action === 'updates.install.failed') {
      title = 'Falha ao instalar atualizações';
      tone = 'red';
    } else if (action === 'alert.acknowledge') {
      title = 'Alerta reconhecido';
      tone = 'amber';
    } else if (action === 'agent.enroll') {
      title = 'Computador vinculado ao CoreControl';
      tone = 'blue';
    } else if (action === 'device.update') {
      title = 'Cadastro do computador atualizado';
      tone = 'blue';
    } else if (action === 'power.economy.entered') {
      title = 'Modo econômico ativado';
      tone = 'blue';
    } else if (action === 'power.economy.exited') {
      title = 'Computador ativado';
      tone = 'green';
    } else if (action === 'power.shutdown.sent') {
      title = 'Desligamento completo enviado';
      tone = 'amber';
    } else if (action === 'power.wake.sent') {
      title = 'Comando legado para ligar enviado';
      tone = 'green';
    } else if (action === 'power.off.sent') {
      title = 'Comando legado para desligar enviado';
      tone = 'amber';
    }

    if (event?.actor_name) subtitle += ` · ${event.actor_name}`;
    return { title, subtitle, tone };
  }

  function renderGlobal(summary, companies, devices, alerts) {
    const attention = devices.filter((device) => !device.online || device.health_score < 70 || device.alerts_open > 0).slice(0, 7);
    CT.$('#pageTitle').textContent = 'Visão geral';
    CT.$('#pageSubtitle').textContent = 'Acompanhe empresas, computadores e alertas da plataforma.';
    CT.$('#content').innerHTML = `
      <section class="page page-overview">
        <div id="overviewStats" class="grid stats-grid"></div>
        <div class="grid split overview-panels">
          <div class="card"><div class="card-header"><div><h2>Computadores que exigem atenção</h2><p>Offline, nota baixa ou alerta ativo.</p></div><button class="btn small" data-go="devices">Ver todos</button></div><div id="overviewAttention"></div></div>
          <div class="card"><div class="card-header"><div><h2>Alertas recentes</h2><p>Eventos técnicos ativos.</p></div><button class="btn small" data-go="alerts">Ver alertas</button></div><div id="overviewAlerts"></div></div>
        </div>
        <div class="card overview-companies-card"><div class="card-header"><div><h2>Empresas</h2><p>Resumo por cliente.</p></div><button id="newCompanyBtn" class="btn primary">Cadastrar empresa</button></div><div id="overviewCompanies" class="grid company-cards"></div></div>
      </section>`;

    CT.$('#overviewStats').innerHTML = [
      CT.stat('Empresas', summary.companies, 'Clientes cadastrados'),
      CT.stat('Computadores', summary.devices, 'Agentes vinculados'),
      CT.stat('Online', summary.online, 'Comunicando agora', 'var(--green)'),
      CT.stat('Offline', summary.offline, 'Sem comunicação', 'var(--red)'),
      CT.stat('Alertas ativos', summary.alerts_open, 'Precisam de avaliação', 'var(--amber)'),
    ].join('');
    CT.$('#overviewAttention').innerHTML = attention.length ? CT.deviceTable(attention) : '<div class="empty"><strong>Tudo certo por aqui</strong><span>Nenhum computador exige atenção agora.</span></div>';
    CT.$('#overviewAlerts').innerHTML = alerts.length ? alerts.slice(0, 6).map(CT.alertRow).join('') : '<div class="empty"><strong>Sem alertas ativos</strong><span>Não há eventos técnicos pendentes neste momento.</span></div>';
    CT.$('#overviewCompanies').innerHTML = companies.length ? companies.map(CT.companyCard).join('') : '<div class="empty"><strong>Nenhuma empresa cadastrada</strong><span>Cadastre a primeira empresa para começar.</span></div>';
    CT.bindCommonActions();
  }

  function renderCompany(summary, devices, alerts) {
    const operations = summary.operations || {};
    const companyName = operations.company_name || CT.state.user?.company?.name || devices[0]?.company_name || 'Sua empresa';
    const onlineDevices = devices.filter((device) => device.online);
    const issueRows = devices.flatMap((device) => deviceIssues(device).map((issue) => ({ ...issue, device })));
    const attentionDevices = new Set(issueRows.map((row) => row.device.id)).size;
    const updatesPending = Number(operations.updates?.pending || 0);
    const last24 = operations.last_24h || {};
    const remoteAvailableCount = devices.filter((device) => Boolean(device.remote?.available)).length;
    const telemetryCount = devices.filter((device) => Boolean(device.telemetry) && device.online).length;
    const allGood = summary.offline === 0 && attentionDevices === 0 && Number(summary.alerts_open || 0) === 0;
    const refreshedAt = new Date().toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' });

    CT.$('#pageTitle').textContent = 'Visão geral';
    CT.$('#pageSubtitle').textContent = 'Acompanhe o estado dos computadores e veja primeiro o que precisa de ação.';

    const computerRows = devices.length ? devices.map((device) => {
      const t = device.telemetry || {};
      const powerOn = CT.devicePowerIsOn(device);
      const economyActive = CT.deviceEconomyModeActive(device);
      const remoteReady = Boolean(device.remote?.available) && !economyActive;
      const healthAvailable = CT.healthAvailable(device);
      const score = healthAvailable ? Math.max(0, Math.min(100, Number(device.health_score || 0))) : null;
      const temperature = tempInfo(t);
      const issues = deviceIssues(device);
      const statusKey = !device.online ? 'offline' : issues.length ? 'attention' : 'online';
      const statusText = economyActive ? 'Modo econômico' : powerOn ? 'Ligado' : 'Desligado';
      const searchable = [device.name, device.hostname, device.sector, statusText, device.online ? 'conectado online' : 'sem comunicação offline'].filter(Boolean).join(' ').toLowerCase();

      return `
        <article class="cc-overview-device-row" data-overview-device data-overview-status="${device.online ? 'online' : 'offline'}${issues.length ? ' attention' : ''}" data-overview-search="${CT.esc(searchable)}">
          <div class="cc-overview-device-name">
            <span class="cc-overview-device-icon">${icon('monitor')}</span>
            <span class="cc-overview-device-copy">
              <strong>${CT.esc(device.name || 'Computador sem nome')}</strong>
              <small>${CT.esc(device.hostname || 'Nome técnico não informado')}${device.sector ? ` · ${CT.esc(device.sector)}` : ''}</small>
            </span>
          </div>

          <div class="cc-overview-state-stack">
            <span class="cc-overview-state ${powerOn ? 'good' : 'neutral'}"><i></i>${CT.esc(statusText)}</span>
            <span class="cc-overview-state ${remoteReady ? 'good' : 'neutral'}"><i></i>${remoteReady ? 'Acesso remoto disponível' : 'Acesso remoto indisponível'}</span>
            <span class="cc-overview-state ${device.online ? 'good' : 'bad'}"><i></i>${device.online ? 'Agent conectado' : 'Agent sem comunicação'}</span>
          </div>

          <div class="cc-overview-health ${score == null ? 'unavailable' : (score >= 80 ? 'good' : score >= 60 ? 'warn' : 'bad')}" style="--health-score:${score == null ? 0 : score}">
            <span class="cc-overview-health-ring"><b>${score == null ? '—' : score}</b></span>
            <small>Saúde</small>
          </div>

          <div class="cc-overview-metric"><strong>${metricValue(t.cpu_percent)}</strong><small>CPU</small></div>
          <div class="cc-overview-metric"><strong>${metricValue(t.memory_percent)}</strong><small>RAM</small></div>
          <div class="cc-overview-metric"><strong>${metricValue(t.disk_percent)}</strong><small>Disco</small></div>
          <div class="cc-overview-metric"><strong>${CT.esc(temperature.value)}</strong><small>Temp.</small></div>

          <div class="cc-overview-last-seen">
            <strong>${device.online ? 'Agora' : CT.esc(ago(device.last_seen))}</strong>
            <small>Agent ${CT.esc(device.agent_version || '—')}</small>
          </div>

          <div class="cc-overview-row-actions">
            <button class="btn small primary" data-ops="remote" data-device="${device.id}" ${remoteReady ? '' : 'disabled'}>${icon('remote')}<span>Acessar</span></button>
            <button class="cc-overview-more" data-ops="device" data-device="${device.id}" title="Abrir detalhes" aria-label="Abrir detalhes de ${CT.esc(device.name || 'computador')}">•••</button>
          </div>
        </article>`;
    }).join('') : '<div class="cc-overview-empty"><strong>Nenhum computador cadastrado</strong><span>Adicione o primeiro computador para começar o monitoramento.</span></div>';

    const attentionItems = [];
    issueRows.slice(0, 3).forEach((row) => {
      attentionItems.push(`
        <button class="cc-overview-attention-item ${row.level}" data-ops="device" data-device="${row.device.id}">
          <span class="cc-overview-alert-icon">${icon('alert')}</span>
          <span><strong>${CT.esc(row.title)}</strong><small>${CT.esc(row.device.name || 'Computador')} · ${CT.esc(row.text)}</small></span>
          ${icon('chevron')}
        </button>`);
    });
    if (updatesPending > 0) {
      attentionItems.push(`
        <button class="cc-overview-attention-item warning" data-ops="updates">
          <span class="cc-overview-alert-icon">${icon('update')}</span>
          <span><strong>${updatesPending} atualização${updatesPending === 1 ? '' : 'ões'} disponíve${updatesPending === 1 ? 'l' : 'is'}</strong><small>Revisar e instalar as atualizações pendentes.</small></span>
          ${icon('chevron')}
        </button>`);
    }
    const attentionHtml = attentionItems.length ? attentionItems.join('') : `
      <div class="cc-overview-attention-ok">
        <span>${icon('check')}</span>
        <div><strong>Nenhuma ação necessária</strong><small>Os computadores estão operando normalmente.</small></div>
      </div>`;

    const statusTitle = devices.length === 0 ? 'Aguardando computadores' : allGood ? 'Operação normal' : 'Operação com atenção';
    const statusSubtitle = devices.length === 0 ? 'Cadastre um computador para iniciar.' : allGood ? 'Nenhum problema crítico detectado.' : 'Existem itens que merecem revisão.';

    CT.$('#content').innerHTML = `
      <section class="page cc-overview-v2">
        <header class="cc-overview-context">
          <div>
            <span class="cc-overview-eyebrow">EMPRESA</span>
            <h2>${CT.esc(companyName)}</h2>
            <p><span class="cc-overview-live-dot ${allGood ? 'good' : 'attention'}"></span>${CT.esc(statusTitle)} <em>·</em> ${CT.esc(statusSubtitle)} <em>·</em> Atualizado às ${CT.esc(refreshedAt)}</p>
          </div>
          <div class="cc-overview-context-actions">
            <button class="btn" data-ops="remote-page">${icon('remote')}<span>Acessar computador</span></button>
            <button class="btn primary" data-ops="reports">${icon('report')}<span>Relatórios</span></button>
          </div>
        </header>

        <div class="cc-overview-kpis">
          <button class="cc-overview-kpi" data-ops="devices">
            <span class="cc-overview-kpi-icon blue">${icon('monitor')}</span>
            <span><small>Computadores</small><strong>${Number(summary.devices || 0)}</strong><p>${Number(summary.devices || 0) === 1 ? '1 cadastrado na empresa' : `${Number(summary.devices || 0)} cadastrados na empresa`}</p></span>
            ${icon('chevron')}
          </button>
          <button class="cc-overview-kpi ${Number(summary.online || 0) ? 'tone-green' : ''}" data-overview-filter-shortcut="online">
            <span class="cc-overview-kpi-icon green">${icon('pulse')}</span>
            <span><small>Online</small><strong>${Number(summary.online || 0)}</strong><p>${Number(summary.online || 0)} com Agent conectado</p></span>
            ${icon('chevron')}
          </button>
          <button class="cc-overview-kpi ${attentionDevices ? 'tone-red' : ''}" data-overview-filter-shortcut="attention">
            <span class="cc-overview-kpi-icon red">${icon('alert')}</span>
            <span><small>Precisam de atenção</small><strong>${attentionDevices}</strong><p>${attentionDevices ? `${attentionDevices} computador${attentionDevices === 1 ? '' : 'es'} com problema` : 'Nenhum problema ativo'}</p></span>
            ${icon('chevron')}
          </button>
          <button class="cc-overview-kpi tone-blue" data-ops="updates">
            <span class="cc-overview-kpi-icon blue">${icon('update')}</span>
            <span><small>Atualizações</small><strong>${updatesPending}</strong><p>${updatesPending ? 'disponíveis para instalação' : 'Tudo atualizado'}</p></span>
            ${icon('chevron')}
          </button>
        </div>

        <div class="cc-overview-main-grid">
          <section class="card cc-overview-computers">
            <div class="cc-overview-section-head">
              <div><h2>Computadores</h2><p>Estado real de cada máquina e comunicação com o CoreControl.</p></div>
              <div class="cc-overview-tools">
                <label class="cc-overview-search">${icon('monitor')}<input id="overviewDeviceSearch" type="search" placeholder="Buscar computador..." autocomplete="off"></label>
                <select id="overviewStatusFilter" class="cc-overview-filter" aria-label="Filtrar computadores por status">
                  <option value="all">Todos os status</option>
                  <option value="online">Online</option>
                  <option value="attention">Com atenção</option>
                  <option value="offline">Sem comunicação</option>
                </select>
              </div>
            </div>

            <div class="cc-overview-table-head" aria-hidden="true">
              <span>Computador</span><span>Estado</span><span>Saúde</span><span>CPU</span><span>RAM</span><span>Disco</span><span>Temp.</span><span>Último contato</span><span>Ações</span>
            </div>
            <div id="overviewDeviceRows" class="cc-overview-device-rows">${computerRows}</div>
            <div id="overviewNoResults" class="cc-overview-empty hidden"><strong>Nenhum computador encontrado</strong><span>Altere a busca ou o filtro para ver outros resultados.</span></div>
          </section>

          <aside class="cc-overview-side">
            <section class="card cc-overview-attention-card">
              <div class="cc-overview-section-head compact"><div><h2>Atenção</h2><p>Somente o que exige alguma ação.</p></div><button class="btn small" data-ops="alerts">Ver todos</button></div>
              <div class="cc-overview-attention-list">${attentionHtml}</div>
            </section>

            <section class="card cc-overview-system-card">
              <div class="cc-overview-section-head compact"><div><h2>Status do sistema</h2><p>Conectividade desta empresa agora.</p></div></div>
              <div class="cc-overview-system-summary ${allGood ? 'good' : 'attention'}">
                <span>${icon('shield')}</span><div><strong>${allGood ? 'CoreControl operacional' : 'CoreControl operacional com alertas'}</strong><small>${allGood ? 'Monitoramento funcionando normalmente.' : 'O painel está disponível; há itens para revisar.'}</small></div>
              </div>
              <div class="cc-overview-system-list">
                <div><span>Agent conectado</span><strong class="${Number(summary.online || 0) === Number(summary.devices || 0) && Number(summary.devices || 0) ? 'good' : 'warn'}">${Number(summary.online || 0)}/${Number(summary.devices || 0)}</strong></div>
                <div><span>Acesso remoto</span><strong class="${remoteAvailableCount ? 'good' : 'muted'}">${remoteAvailableCount} disponível${remoteAvailableCount === 1 ? '' : 'is'}</strong></div>
                <div><span>Telemetria atual</span><strong class="${telemetryCount ? 'good' : 'muted'}">${telemetryCount}/${Number(summary.devices || 0)}</strong></div>
                <div><span>Alertas ativos</span><strong class="${Number(summary.alerts_open || 0) ? 'bad' : 'good'}">${Number(summary.alerts_open || 0)}</strong></div>
              </div>
            </section>
          </aside>
        </div>

        <section class="card cc-overview-activity">
          <div class="cc-overview-section-head compact"><div><h2>Atividade recente</h2><p>Ações realizadas nas últimas 24 horas.</p></div><button class="btn small" data-ops="reports">Ver histórico completo</button></div>
          <div class="cc-overview-activity-grid">
            <button data-ops="remote-page"><span>${icon('remote')}</span><div><strong>${Number(last24.remote_sessions || 0)}</strong><small>Acessos remotos</small></div>${icon('chevron')}</button>
            <button data-ops="devices"><span>${icon('spark')}</span><div><strong>${Number(last24.optimizations || 0)}</strong><small>Otimizações</small></div>${icon('chevron')}</button>
            <button data-ops="devices"><span>${icon('gauge')}</span><div><strong>${Number(last24.diagnostics || 0)}</strong><small>Diagnósticos</small></div>${icon('chevron')}</button>
            <button data-ops="devices"><span>${icon('disk')}</span><div><strong>${Number(last24.cleanups || 0)}</strong><small>Limpezas seguras</small></div>${icon('chevron')}</button>
          </div>
        </section>
      </section>`;

    const searchInput = CT.$('#overviewDeviceSearch');
    const statusFilter = CT.$('#overviewStatusFilter');
    const noResults = CT.$('#overviewNoResults');

    function applyDeviceFilters() {
      const query = String(searchInput?.value || '').trim().toLowerCase();
      const status = String(statusFilter?.value || 'all');
      let visible = 0;
      CT.$$('[data-overview-device]').forEach((row) => {
        const matchesSearch = !query || String(row.dataset.overviewSearch || '').includes(query);
        const rowStatuses = String(row.dataset.overviewStatus || '').split(/\s+/).filter(Boolean);
        const matchesStatus = status === 'all' || rowStatuses.includes(status);
        const show = matchesSearch && matchesStatus;
        row.classList.toggle('hidden', !show);
        if (show) visible += 1;
      });
      noResults?.classList.toggle('hidden', visible !== 0 || devices.length === 0);
    }

    searchInput?.addEventListener('input', applyDeviceFilters);
    statusFilter?.addEventListener('change', applyDeviceFilters);
    CT.$$('[data-overview-filter-shortcut]').forEach((button) => {
      button.addEventListener('click', () => {
        if (!statusFilter) return;
        statusFilter.value = button.dataset.overviewFilterShortcut || 'all';
        applyDeviceFilters();
        CT.$('.cc-overview-computers')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
      });
    });

    CT.$$('[data-ops]').forEach((button) => {
      button.addEventListener('click', async () => {
        if (button.disabled) return;
        const action = button.dataset.ops;
        const deviceId = Number(button.dataset.device || 0);
        if (action === 'device' && deviceId) return CT.navigate('device', deviceId);
        if (action === 'remote' && deviceId) return CT.openRemoteSession(deviceId);
        if (action === 'devices') return CT.navigate('devices');
        if (action === 'remote-page') return CT.navigate('remote');
        if (action === 'alerts') return CT.navigate('alerts');
        if (action === 'updates') return CT.navigate('updates');
        if (action === 'reports') return CT.navigate('reports');
      });
    });
  }
  CT.registerPage('overview', async function renderOverview() {
    const [summary, companies, devices, alerts] = await Promise.all([
      CT.api('/dashboard/summary'),
      CT.api('/companies'),
      CT.api('/devices'),
      CT.api('/alerts?status_filter=active'),
    ]);

    CT.setAlertBadge(summary.alerts_open);
    if (CT.isGlobalAdmin()) {
      renderGlobal(summary, companies, devices, alerts);
      return;
    }
    renderCompany(summary, devices, alerts);
  });
})();

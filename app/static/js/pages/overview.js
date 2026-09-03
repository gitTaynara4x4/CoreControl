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
    power: '<svg viewBox="0 0 24 24"><path d="M12 3v8"/><path d="M7.05 5.75a8 8 0 1 0 9.9 0"/></svg>',
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
    } else if (action === 'power.wake.request') {
      title = 'Comando para ligar computador enviado';
      tone = 'blue';
    } else if (action === 'power.off.request') {
      title = 'Desligamento remoto solicitado';
      tone = 'amber';
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
    }

    if (event?.actor_name) subtitle += ` · ${event.actor_name}`;
    return { title, subtitle, tone };
  }

  function powerIsOn(device) {
    if (device?.power && typeof device.power.powered_on === 'boolean') return device.power.powered_on;
    return Boolean(device?.online || device?.remote?.mesh_connected);
  }

  function powerAvailable(device) {
    if (device?.power && typeof device.power.available === 'boolean') return device.power.available;
    return Boolean(device?.remote?.mesh_node_id);
  }

  async function waitPowerState(deviceId, expectedOn, timeoutMs = 90000) {
    const started = Date.now();
    while (Date.now() - started < timeoutMs) {
      await new Promise((resolve) => setTimeout(resolve, 4000));
      try {
        const latest = await CT.api(`/devices/${deviceId}`);
        if (powerIsOn(latest) === expectedOn) return latest;
      } catch (_) {}
    }
    return null;
  }

  async function runPowerAction(button, device, action) {
    if (!powerAvailable(device)) {
      CT.toast('O controle de energia exige o módulo remoto configurado neste computador.', true);
      return;
    }
    if (action === 'off') {
      const confirmed = window.confirm(
        `Desligar “${device.name || device.hostname || 'este computador'}”?\n\n` +
        'O Windows será desligado e programas abertos podem perder alterações não salvas.'
      );
      if (!confirmed) return;
    }

    const original = button.innerHTML;
    button.disabled = true;
    button.textContent = action === 'wake' ? 'Ligando…' : 'Desligando…';
    try {
      const result = await CT.api(`/devices/${device.id}/power/${action}`, { method: 'POST' });
      CT.toast(result.message || 'Comando de energia enviado.');
      const reached = await waitPowerState(device.id, action === 'wake');
      if (reached) {
        CT.toast(action === 'wake' ? 'Computador ligado e comunicando.' : 'Computador desligado.');
      } else if (action === 'wake') {
        CT.toast(result.warning || 'O sinal foi enviado, mas o computador ainda não respondeu. Verifique o Wake-on-LAN da máquina.', true);
      } else {
        CT.toast('O comando foi enviado, mas o computador ainda aparece online.', true);
      }
      return CT.navigate('overview');
    } catch (error) {
      CT.toast(error.message, true);
      button.disabled = false;
      button.innerHTML = original;
    }
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
    const onlineHealth = onlineDevices.map((device) => Number(device.health_score || 0));
    const avgHealth = onlineHealth.length ? Math.round(onlineHealth.reduce((sum, value) => sum + value, 0) / onlineHealth.length) : 0;
    const optimized = devices.filter((device) => cleanProfile(device.profile)).length;
    const issueRows = devices.flatMap((device) => deviceIssues(device).map((issue) => ({ ...issue, device })));
    const attentionDevices = new Set(issueRows.map((row) => row.device.id)).size;
    const focusDevices = onlineDevices.filter((device) => device.telemetry?.activity?.process_name);
    const securitySamples = onlineDevices.filter((device) => device.telemetry);
    const securityProblems = securitySamples.filter((device) => device.telemetry.defender_active === false || device.telemetry.firewall_active === false).length;
    const updatesPending = Number(operations.updates?.pending || 0);
    const rebootRequired = Number(operations.updates?.reboot_required || 0);
    const allGood = summary.offline === 0 && attentionDevices === 0 && summary.alerts_open === 0;
    const statusTitle = devices.length === 0 ? 'Nenhum computador monitorado ainda' : allGood ? 'Tudo funcionando normalmente' : 'Há pontos que precisam da sua atenção';
    const statusText = devices.length === 0 ? 'Adicione o primeiro computador para começar a acompanhar a operação.' : allGood ? 'Todos os computadores estão comunicando e sem alertas críticos.' : `${attentionDevices || summary.offline} computador${(attentionDevices || summary.offline) === 1 ? '' : 'es'} merece${(attentionDevices || summary.offline) === 1 ? '' : 'm'} uma análise.`;

    CT.$('#pageTitle').textContent = 'Visão geral';
    CT.$('#pageSubtitle').textContent = `${companyName} · central de operação em tempo quase real.`;

    const computerCards = devices.length ? devices.map((device) => {
      const t = device.telemetry || {};
      const activity = t.activity || {};
      const currentApp = device.online ? friendlyApp(activity.process_name) : 'Sem comunicação';
      const currentWindow = device.online ? (activity.window_title || 'Nenhuma janela em foco identificada') : `Último contato ${ago(device.last_seen)}`;
      const temperature = tempInfo(t);
      const profile = cleanProfile(device.profile);
      const remoteReady = Boolean(device.remote?.available);
      const stateTone = device.online ? (device.health_score >= 80 ? 'good' : 'warn') : 'bad';
      return `
        <article class="ops-device-card" data-device-card="${device.id}">
          <div class="ops-device-head">
            <div class="ops-device-ident">
              <span class="ops-device-icon">${icon('monitor')}</span>
              <div><div class="ops-device-title-row"><h3>${CT.esc(device.name || 'Computador sem nome')}</h3><span class="ops-live ${device.online ? 'online' : 'offline'}"><i></i>${device.online ? 'Online' : 'Offline'}</span></div><p>Nome técnico: ${CT.esc(device.hostname || 'não informado')}${device.sector ? ` · ${CT.esc(device.sector)}` : ''}</p></div>
            </div>
            <div class="ops-health-badge ${stateTone}"><strong>${device.health_score}</strong><span>Saúde</span></div>
          </div>
          <div class="ops-device-focus"><span class="ops-focus-label">Em foco agora</span><strong>${CT.esc(currentApp)}</strong><small title="${CT.esc(currentWindow)}">${CT.esc(currentWindow)}</small></div>
          <div class="ops-device-metrics">
            <div><span>CPU</span><strong>${metricValue(t.cpu_percent)}</strong></div>
            <div><span>RAM</span><strong>${metricValue(t.memory_percent)}</strong></div>
            <div><span>Disco</span><strong>${metricValue(t.disk_percent)}</strong></div>
            <div><span>GPU</span><strong>${metricValue(t.gpu_usage_percent)}</strong></div>
            <div><span>${CT.esc(temperature.label)}</span><strong>${CT.esc(temperature.value)}</strong></div>
          </div>
          <div class="ops-device-foot">
            <div class="ops-device-meta"><span>${profile ? `Perfil: <b>${CT.esc(profile)}</b>` : 'Sem perfil de otimização ativo'}</span><span>Agente ${CT.esc(device.agent_version || '—')} · ${device.online ? `atualizado ${ago(device.last_seen)}` : `último contato ${ago(device.last_seen)}`}</span></div>
            <div class="ops-device-actions">
              <button class="btn small" data-ops="device" data-device="${device.id}">Ver atividade</button>
              <button class="btn small" data-ops="remote" data-device="${device.id}" ${remoteReady ? '' : 'disabled'}>Acessar</button>
              <button class="btn small primary" data-ops="optimize" data-device="${device.id}" ${device.online ? '' : 'disabled'}>Otimizar</button>
              <button class="btn small ${powerIsOn(device) ? 'danger' : 'primary'}" data-ops="power" data-power-action="${powerIsOn(device) ? 'off' : 'wake'}" data-device="${device.id}" ${powerAvailable(device) ? '' : 'disabled'}>${icon('power')} ${powerIsOn(device) ? 'Desligar' : 'Ligar computador'}</button>
            </div>
          </div>
        </article>`;
    }).join('') : '<div class="ops-empty-compact"><strong>Nenhum computador cadastrado</strong><span>Adicione um computador para começar a acompanhar a operação.</span></div>';

    const attentionHtml = issueRows.length ? issueRows.slice(0, 6).map((row) => `
      <button class="ops-attention-row ${row.level}" data-ops="device" data-device="${row.device.id}">
        <span class="ops-attention-dot"></span><span><strong>${CT.esc(row.device.name)}</strong><b>${CT.esc(row.title)}</b><small>${CT.esc(row.text)}</small></span>${icon('chevron')}
      </button>`).join('') : `
      <div class="ops-ok-state"><span>${icon('check')}</span><strong>Tudo certo por aqui</strong><p>Nenhum computador exige atenção agora.</p></div>`;

    const focusHtml = devices.length ? devices.map((device) => {
      const activity = device.telemetry?.activity || {};
      const app = device.online ? friendlyApp(activity.process_name) : 'Offline';
      const windowTitle = device.online ? (activity.window_title || 'Sem janela identificada') : `Último contato ${ago(device.last_seen)}`;
      return `<button class="ops-activity-row" data-ops="device" data-device="${device.id}"><span class="ops-activity-status ${device.online ? 'online' : 'offline'}"></span><span class="ops-activity-device"><strong>${CT.esc(device.name || 'Computador sem nome')}</strong><small>${device.hostname ? `Nome técnico: ${CT.esc(device.hostname)}` : 'Nome técnico não informado'}</small></span><span class="ops-activity-app"><strong>${CT.esc(app)}</strong><small title="${CT.esc(windowTitle)}">${CT.esc(windowTitle)}</small></span><span class="ops-activity-health ${CT.healthClass(device.health_score)}">${device.health_score}/100</span>${icon('chevron')}</button>`;
    }).join('') : '<div class="ops-empty-compact"><span>Sem atividade para exibir.</span></div>';

    const last24 = operations.last_24h || {};
    const recentHtml = (operations.recent_events || []).length ? operations.recent_events.slice(0, 8).map((event) => {
      const copy = eventCopy(event);
      return `<button class="ops-event-row" ${event.device_id ? `data-ops="device" data-device="${event.device_id}"` : ''}><span class="ops-event-mark ${copy.tone}"></span><span><strong>${CT.esc(copy.title)}</strong><small>${CT.esc(copy.subtitle)}</small></span><time>${CT.esc(ago(event.created_at))}</time></button>`;
    }).join('') : '<div class="ops-empty-compact"><strong>Nenhum acontecimento recente</strong><span>As principais ações administrativas aparecerão aqui.</span></div>';

    const maxTemp = onlineDevices.reduce((max, device) => {
      const t = device.telemetry || {};
      const value = Number(t.temperature_c ?? t.gpu_temperature_c);
      return Number.isFinite(value) ? Math.max(max, value) : max;
    }, 0);
    const minDiskFree = onlineDevices.reduce((min, device) => {
      const value = Number(device.telemetry?.disk_free_gb);
      return Number.isFinite(value) ? Math.min(min, value) : min;
    }, Number.POSITIVE_INFINITY);

    CT.$('#content').innerHTML = `
      <section class="page ops-overview">
        <div class="ops-hero">
          <div class="ops-hero-copy"><span class="ops-kicker">CENTRAL DE OPERAÇÃO</span><h2>${CT.esc(companyName)}</h2><div class="ops-hero-status ${allGood ? 'good' : 'attention'}"><span></span><strong>${CT.esc(statusTitle)}</strong><small>${CT.esc(statusText)}</small></div></div>
          <div class="ops-hero-actions"><button class="btn" data-ops="devices">Computadores</button><button class="btn" data-ops="remote-page">Acesso remoto</button><button class="btn" data-ops="alerts">Alertas</button><button class="btn primary" data-ops="reports">Relatórios</button></div>
        </div>

        <div class="ops-kpis">
          <div class="ops-kpi"><span class="ops-kpi-icon">${icon('monitor')}</span><div><small>Computadores online</small><strong>${summary.online}<em>/${summary.devices}</em></strong><p>${summary.offline ? `${summary.offline} sem comunicação` : 'Todos comunicando'}</p></div></div>
          <div class="ops-kpi"><span class="ops-kpi-icon">${icon('pulse')}</span><div><small>Saúde média</small><strong>${avgHealth}<em>/100</em></strong><p>${onlineDevices.length ? healthLabel(avgHealth) : 'Sem leitura'}</p></div></div>
          <div class="ops-kpi"><span class="ops-kpi-icon">${icon('alert')}</span><div><small>Precisam de atenção</small><strong>${attentionDevices}</strong><p>${summary.alerts_open ? `${summary.alerts_open} alerta${summary.alerts_open === 1 ? '' : 's'} ativo${summary.alerts_open === 1 ? '' : 's'}` : 'Sem alertas ativos'}</p></div></div>
          <div class="ops-kpi"><span class="ops-kpi-icon">${icon('spark')}</span><div><small>Com otimização ativa</small><strong>${optimized}<em>/${summary.devices}</em></strong><p>${optimized ? 'Perfis aplicados' : 'Nenhum perfil ativo'}</p></div></div>
          <div class="ops-kpi"><span class="ops-kpi-icon">${icon('update')}</span><div><small>Atualizações</small><strong>${updatesPending}</strong><p>${rebootRequired ? `${rebootRequired} aguardando reinício` : 'Sem reinício pendente'}</p></div></div>
        </div>

        <div class="ops-main-grid">
          <section class="card ops-computers-panel"><div class="ops-section-head"><div><span>OPERAÇÃO AGORA</span><h2>Computadores agora</h2><p>Estado, atividade em foco, saúde e desempenho de cada máquina.</p></div><button class="btn small" data-ops="devices">Ver todos</button></div><div class="ops-device-list">${computerCards}</div></section>
          <aside class="card ops-attention-panel"><div class="ops-section-head"><div><span>PRIORIDADE</span><h2>Precisa da sua atenção</h2><p>Só o que exige alguma ação.</p></div><button class="btn small" data-ops="alerts">Ver alertas</button></div><div class="ops-attention-list">${attentionHtml}</div></aside>
        </div>

        <div class="ops-secondary-grid">
          <section class="card"><div class="ops-section-head"><div><span>EQUIPE EM ATIVIDADE</span><h2>Em foco agora</h2><p>O que está aberto em primeiro plano em cada computador.</p></div><span class="ops-section-count">${focusDevices.length} em atividade</span></div><div class="ops-activity-list">${focusHtml}</div></section>
          <section class="card"><div class="ops-section-head"><div><span>ÚLTIMAS 24 HORAS</span><h2>Resumo da operação</h2><p>Ações realizadas pelo CoreControl e pela administração.</p></div></div><div class="ops-summary-grid"><div><span>${icon('spark')}</span><strong>${Number(last24.optimizations || 0)}</strong><small>Otimizações</small></div><div><span>${icon('gauge')}</span><strong>${Number(last24.diagnostics || 0)}</strong><small>Diagnósticos</small></div><div><span>${icon('remote')}</span><strong>${Number(last24.remote_sessions || 0)}</strong><small>Acessos remotos</small></div><div><span>${icon('disk')}</span><strong>${Number(last24.cleanups || 0)}</strong><small>Limpezas seguras</small></div></div></section>
        </div>

        <div class="ops-secondary-grid ops-bottom-grid">
          <section class="card"><div class="ops-section-head"><div><span>SAÚDE E OTIMIZAÇÃO</span><h2>Visão técnica simplificada</h2><p>Indicadores que ajudam a decidir quando agir.</p></div><button class="btn small" data-ops="optimize-first" ${onlineDevices.length ? '' : 'disabled'}>Analisar computador</button></div><div class="ops-health-list"><div><span class="ops-health-icon">${icon('shield')}</span><span><strong>Proteção do Windows</strong><small>${securitySamples.length ? (securityProblems ? `${securityProblems} computador${securityProblems === 1 ? '' : 'es'} com proteção incompleta` : 'Defender e Firewall sem problemas detectados') : 'Sem leitura disponível'}</small></span><b class="${securityProblems ? 'warn' : 'good'}">${securityProblems ? 'Atenção' : 'Normal'}</b></div><div><span class="ops-health-icon">${icon('disk')}</span><span><strong>Armazenamento</strong><small>${Number.isFinite(minDiskFree) ? `Menor espaço livre: ${CT.fmtNum(minDiskFree, 1)} GB` : 'Sem leitura disponível'}</small></span><b class="${Number.isFinite(minDiskFree) && minDiskFree < 15 ? 'warn' : 'good'}">${Number.isFinite(minDiskFree) && minDiskFree < 15 ? 'Atenção' : 'Normal'}</b></div><div><span class="ops-health-icon">${icon('temperature')}</span><span><strong>Temperatura</strong><small>${maxTemp ? `Maior leitura atual: ${CT.fmtNum(maxTemp, 0)} °C` : 'Sensor não disponível nos computadores atuais'}</small></span><b class="${maxTemp >= 80 ? 'warn' : 'good'}">${maxTemp >= 80 ? 'Atenção' : 'Normal'}</b></div><div><span class="ops-health-icon">${icon('clock')}</span><span><strong>Tempo ligado</strong><small>${onlineDevices.length ? `Maior uptime atual: ${duration(Math.max(...onlineDevices.map((device) => Number(device.telemetry?.uptime_seconds || 0))))}` : 'Nenhum computador online'}</small></span><b class="good">Informativo</b></div></div></section>
          <section class="card"><div class="ops-section-head"><div><span>HISTÓRICO RECENTE</span><h2>Últimos acontecimentos</h2><p>O que mudou recentemente na operação.</p></div></div><div class="ops-events-list">${recentHtml}</div></section>
        </div>
      </section>`;

    CT.$$('[data-ops]').forEach((button) => {
      button.addEventListener('click', async () => {
        if (button.disabled) return;
        const action = button.dataset.ops;
        const deviceId = Number(button.dataset.device || 0);
        if (action === 'device' && deviceId) return CT.navigate('device', deviceId);
        if (action === 'remote' && deviceId) return CT.openRemoteSession(deviceId);
        if (action === 'power' && deviceId) {
          const device = devices.find((item) => Number(item.id) === deviceId);
          if (!device) return;
          return runPowerAction(button, device, button.dataset.powerAction || (powerIsOn(device) ? 'off' : 'wake'));
        }
        if (action === 'optimize' && deviceId) {
          await CT.navigate('device', deviceId);
          window.requestAnimationFrame(() => CT.$('.optimization-card')?.scrollIntoView({ behavior: 'smooth', block: 'start' }));
          return;
        }
        if (action === 'optimize-first') {
          const target = onlineDevices[0];
          if (!target) return;
          await CT.navigate('device', target.id);
          window.requestAnimationFrame(() => CT.$('.optimization-card')?.scrollIntoView({ behavior: 'smooth', block: 'start' }));
          return;
        }
        if (action === 'devices') return CT.navigate('devices');
        if (action === 'remote-page') return CT.navigate('remote');
        if (action === 'alerts') return CT.navigate('alerts');
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

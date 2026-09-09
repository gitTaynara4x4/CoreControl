(function () {
  'use strict';

  const CT = window.CoreTuner;
  let activeTab = 'overview';

  function icon(name) {
    const icons = {
      shield: '<svg viewBox="0 0 24 24"><path d="M12 3.5 19 6v5.6c0 4.3-2.6 7.4-7 8.9-4.4-1.5-7-4.6-7-8.9V6l7-2.5Z"/><path d="m8.8 12 2 2 4.4-4.5"/></svg>',
      computer: '<svg viewBox="0 0 24 24"><rect x="3.5" y="4.5" width="17" height="12" rx="2"/><path d="M8 20h8M12 16.5V20"/></svg>',
      download: '<svg viewBox="0 0 24 24"><path d="M12 4v10"/><path d="m8.5 10.5 3.5 3.5 3.5-3.5"/><path d="M5 19h14"/></svg>',
      alert: '<svg viewBox="0 0 24 24"><path d="M12 4 3.8 19h16.4L12 4Z"/><path d="M12 9v4M12 16.5h.01"/></svg>',
      restart: '<svg viewBox="0 0 24 24"><path d="M20 11a8 8 0 1 0-2.35 5.65"/><path d="M20 4v7h-7"/></svg>',
      windows: '<svg viewBox="0 0 24 24"><path d="M4 5.5 11 4.5v7H4v-6ZM13 4.2 20 3v8.5h-7V4.2ZM4 13h7v7L4 19v-6ZM13 13h7v8l-7-1.2V13Z"/></svg>',
      driver: '<svg viewBox="0 0 24 24"><rect x="5" y="5" width="14" height="14" rx="3"/><path d="M9 2.5V5M15 2.5V5M9 19v2.5M15 19v2.5M2.5 9H5M2.5 15H5M19 9h2.5M19 15h2.5"/><circle cx="12" cy="12" r="2.3"/></svg>',
      app: '<svg viewBox="0 0 24 24"><rect x="4" y="4" width="6" height="6" rx="1"/><rect x="14" y="4" width="6" height="6" rx="1"/><rect x="4" y="14" width="6" height="6" rx="1"/><rect x="14" y="14" width="6" height="6" rx="1"/></svg>',
      policy: '<svg viewBox="0 0 24 24"><path d="M7 4h10M7 8h10M7 12h6"/><rect x="4" y="2.5" width="16" height="19" rx="2"/><path d="m13.5 17 1.5 1.5 3-3"/></svg>',
      search: '<svg viewBox="0 0 24 24"><circle cx="10.5" cy="10.5" r="6"/><path d="m15 15 5 5"/></svg>',
      check: '<svg viewBox="0 0 24 24"><path d="m5 12.5 4.2 4.2L19 7"/></svg>',
      clock: '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="8"/><path d="M12 7.5V12l3 2"/></svg>',
    };
    return icons[name] || icons.computer;
  }

  function statusPill(label, tone = '') {
    return `<span class="pill ${tone}">${CT.esc(label)}</span>`;
  }

  function commandLabel(device) {
    if (!device.agent_supports_updates) return statusPill('Agente desatualizado', 'warning');
    const status = device.status;
    if (status === 'queued') return statusPill('Na fila', 'warning');
    if (status === 'scanning') return statusPill('Verificando', 'warning');
    if (status === 'installing') return statusPill('Instalando', 'warning');
    if (status === 'error') return statusPill('Falha', 'critical');
    if (!device.last_scan_at) return statusPill('Não verificado');
    if (device.pending_total === 0) return statusPill('Atualizado', 'resolved');
    return statusPill(`${device.pending_total} pendente${device.pending_total === 1 ? '' : 's'}`, device.critical_pending ? 'critical' : 'warning');
  }

  function pendingCell(value, scanned) {
    if (!scanned) return '<span class="updates-muted-value">—</span>';
    if (!value) return '<span class="updates-zero">0</span>';
    return `<strong class="updates-count">${CT.esc(value)}</strong>`;
  }

  function sum(devices, field) {
    return devices.reduce((total, device) => total + Number(device[field] || 0), 0);
  }

  function overviewState(summary) {
    if (!summary.devices) {
      return {
        tone: 'neutral',
        iconName: 'computer',
        title: 'Nenhum computador cadastrado',
        text: 'Cadastre um computador para começar a acompanhar Windows, drivers e aplicativos.',
      };
    }
    if (summary.busy) {
      return {
        tone: 'working',
        iconName: 'download',
        title: 'Verificação em andamento',
        text: `${summary.busy} computador${summary.busy === 1 ? '' : 'es'} processando uma operação de atualização.`,
      };
    }
    if (summary.scanned < summary.devices) {
      const missing = summary.devices - summary.scanned;
      return {
        tone: 'attention',
        iconName: 'clock',
        title: 'Faça a primeira verificação',
        text: `${missing} computador${missing === 1 ? '' : 'es'} ainda não ${missing === 1 ? 'foi verificado' : 'foram verificados'}.`,
      };
    }
    if (summary.critical) {
      return {
        tone: 'critical',
        iconName: 'alert',
        title: 'Atualizações críticas encontradas',
        text: `${summary.critical} atualização${summary.critical === 1 ? '' : 'ões'} crítica${summary.critical === 1 ? '' : 's'} do Windows precisa${summary.critical === 1 ? '' : 'm'} de atenção.`,
      };
    }
    if (summary.pending) {
      return {
        tone: 'attention',
        iconName: 'download',
        title: 'Há atualizações disponíveis',
        text: `${summary.pending} atualização${summary.pending === 1 ? '' : 'ões'} pendente${summary.pending === 1 ? '' : 's'} entre Windows, drivers e aplicativos.`,
      };
    }
    return {
      tone: 'good',
      iconName: 'shield',
      title: 'Computadores atualizados',
      text: 'Nenhuma atualização pendente foi encontrada na última verificação.',
    };
  }

  function kpiCard(label, value, detail, tone, iconName) {
    return `
      <article class="updates-kpi ${tone || ''}">
        <span class="updates-kpi-icon" aria-hidden="true">${icon(iconName)}</span>
        <div class="updates-kpi-copy">
          <span>${CT.esc(label)}</span>
          <strong>${CT.esc(value)}</strong>
          <small>${CT.esc(detail)}</small>
        </div>
      </article>`;
  }

  function sourceCard(iconName, title, source, pending, description, scanned, tone = '') {
    const value = scanned ? `${pending} pendente${pending === 1 ? '' : 's'}` : 'Aguardando verificação';
    return `
      <article class="updates-source-card ${tone}">
        <div class="updates-source-head">
          <span class="updates-source-icon" aria-hidden="true">${icon(iconName)}</span>
          <div>
            <strong>${CT.esc(title)}</strong>
            <small>${CT.esc(source)}</small>
          </div>
        </div>
        <p>${CT.esc(description)}</p>
        <div class="updates-source-foot">
          <span class="updates-source-value">${CT.esc(value)}</span>
          ${scanned && pending === 0 ? '<span class="updates-source-ok">✓ Em dia</span>' : ''}
        </div>
      </article>`;
  }

  function renderOverview(data) {
    const summary = data.summary;
    const devices = data.devices || [];
    const state = overviewState(summary);
    const windowsPending = sum(devices, 'windows_pending');
    const driversPending = sum(devices, 'driver_pending');
    const appsPending = sum(devices, 'app_pending');
    const allScanned = summary.devices > 0 && summary.scanned === summary.devices;
    const recent = devices
      .slice()
      .sort((a, b) => (b.critical_pending - a.critical_pending) || (b.pending_total - a.pending_total) || String(a.device_name).localeCompare(String(b.device_name)))
      .slice(0, 8);

    const rows = recent.map((device) => `
      <tr>
        <td>
          <div class="updates-device-cell">
            <span class="updates-device-dot ${device.online ? 'online' : 'offline'}"></span>
            <div><strong>${CT.esc(device.device_name)}</strong><small>${CT.esc(device.company_name || device.hostname || '—')}</small></div>
          </div>
        </td>
        <td>${commandLabel(device)}</td>
        <td>${pendingCell(device.windows_pending, device.last_scan_at)}</td>
        <td>${pendingCell(device.driver_pending, device.last_scan_at)}</td>
        <td>${pendingCell(device.app_pending, device.last_scan_at)}</td>
        <td><span class="updates-date">${device.last_scan_at ? CT.fmtDate(device.last_scan_at) : 'Nunca'}</span></td>
        <td class="table-actions-col"><button class="btn small" type="button" data-update-detail="${device.device_id}">Detalhes</button></td>
      </tr>`).join('');

    return `
      <div class="updates-status-card ${state.tone}">
        <div class="updates-status-main">
          <span class="updates-status-icon" aria-hidden="true">${icon(state.iconName)}</span>
          <div>
            <span class="updates-eyebrow">STATUS DAS ATUALIZAÇÕES</span>
            <h2>${CT.esc(state.title)}</h2>
            <p>${CT.esc(state.text)}</p>
          </div>
        </div>
        <div class="updates-status-meta">
          <span><b>${summary.scanned}</b> de ${summary.devices} verificados</span>
          <span><b>${summary.busy}</b> em andamento</span>
        </div>
      </div>

      <div class="updates-kpi-grid">
        ${kpiCard('Computadores verificados', `${summary.scanned}/${summary.devices}`, summary.scanned === summary.devices && summary.devices ? 'Inventário atualizado' : 'Aguardando coleta', summary.scanned === summary.devices && summary.devices ? 'good' : 'neutral', 'computer')}
        ${kpiCard('Pendentes', summary.pending, 'Windows + drivers + aplicativos', summary.pending ? 'attention' : 'good', 'download')}
        ${kpiCard('Críticas', summary.critical, 'Segurança e correções do Windows', summary.critical ? 'critical' : 'good', 'alert')}
        ${kpiCard('Reinício necessário', summary.reboot_required, 'Reinício continua sob aprovação', summary.reboot_required ? 'attention' : 'good', 'restart')}
      </div>

      <section class="updates-section-block">
        <div class="updates-section-heading">
          <div><span class="updates-eyebrow">FONTES</span><h2>O que o CoreControl atualiza</h2><p>“Atualizações” é o módulo geral. Windows Update, drivers e aplicativos são categorias dentro dele.</p></div>
        </div>
        <div class="updates-source-grid">
          ${sourceCard('windows', 'Windows', 'Windows Update', windowsPending, 'Correções do sistema, segurança e atualizações cumulativas disponibilizadas pela Microsoft.', summary.scanned > 0, windowsPending ? 'attention' : '')}
          ${sourceCard('driver', 'Drivers', 'Windows Update', driversPending, 'Drivers de hardware publicados no catálogo do Windows Update, sempre sob aprovação.', summary.scanned > 0, driversPending ? 'attention' : '')}
          ${sourceCard('app', 'Aplicativos', 'Windows Package Manager', appsPending, 'Programas compatíveis detectados e atualizados pelo winget quando disponível.', summary.scanned > 0, appsPending ? 'attention' : '')}
          <article class="updates-source-card policy">
            <div class="updates-source-head">
              <span class="updates-source-icon" aria-hidden="true">${icon('policy')}</span>
              <div><strong>Políticas</strong><small>Automação</small></div>
            </div>
            <p>Defina horários de verificação e instalação por empresa sem forçar reinicialização.</p>
            <div class="updates-source-foot"><button class="updates-inline-action" type="button" data-go-policies>Configurar políticas →</button></div>
          </article>
        </div>
      </section>

      <section class="card updates-table-card">
        <div class="updates-card-header">
          <div><span class="updates-eyebrow">COMPUTADORES</span><h2>Situação atual</h2><p>Veja rapidamente quais máquinas precisam ser verificadas ou têm atualizações disponíveis.</p></div>
          <button class="btn" type="button" data-go-computers>Ver todos</button>
        </div>
        ${summary.devices && !allScanned ? `<div class="updates-notice"><span>${icon('clock')}</span><div><strong>${summary.devices - summary.scanned} computador${summary.devices - summary.scanned === 1 ? '' : 'es'} aguardando verificação</strong><p>Use “Verificar agora” para coletar Windows Update, drivers e aplicativos.</p></div></div>` : ''}
        <div class="table-wrap updates-table-wrap">
          <table class="updates-table">
            <thead><tr><th>Computador</th><th>Status</th><th>Windows</th><th>Drivers</th><th>Apps</th><th>Última verificação</th><th></th></tr></thead>
            <tbody>${rows || '<tr><td colspan="7" class="table-empty-cell">Nenhum computador cadastrado.</td></tr>'}</tbody>
          </table>
        </div>
      </section>`;
  }

  function renderComputers(devices) {
    const rows = devices.map((device) => {
      const rowStatus = !device.last_scan_at ? 'not-scanned' : device.pending_total ? 'pending' : 'updated';
      return `
        <tr data-update-device-row data-search="${CT.esc(`${device.device_name} ${device.hostname || ''} ${device.company_name || ''}`.toLowerCase())}" data-update-status="${rowStatus}">
          <td>
            <div class="updates-device-cell">
              <span class="updates-device-dot ${device.online ? 'online' : 'offline'}"></span>
              <div><strong>${CT.esc(device.device_name)}</strong><small>${CT.esc(device.hostname || '—')}</small></div>
            </div>
          </td>
          <td>${CT.esc(device.company_name || '—')}</td>
          <td>${device.online ? '<span class="status"><i class="dot online"></i>Online</span>' : '<span class="status"><i class="dot offline"></i>Offline</span>'}</td>
          <td>${commandLabel(device)}</td>
          <td>${pendingCell(device.windows_pending, device.last_scan_at)}</td>
          <td>${pendingCell(device.driver_pending, device.last_scan_at)}</td>
          <td>${pendingCell(device.app_pending, device.last_scan_at)}</td>
          <td>${device.reboot_required ? statusPill('Necessário', 'warning') : '<span class="updates-muted-value">—</span>'}</td>
          <td class="table-actions-col update-row-actions">
            <button class="btn small" type="button" data-update-check="${device.device_id}" ${!device.agent_supports_updates || ['queued','scanning','installing'].includes(device.status) ? 'disabled' : ''}>Verificar</button>
            <button class="btn small" type="button" data-update-detail="${device.device_id}">Detalhes</button>
          </td>
        </tr>`;
    }).join('');

    return `
      <section class="card updates-table-card updates-computers-card">
        <div class="updates-card-header updates-card-header-wrap">
          <div><span class="updates-eyebrow">GERENCIAMENTO</span><h2>Atualizações por computador</h2><p>Verifique, revise e instale atualizações em cada máquina separadamente.</p></div>
          <span class="updates-count-badge">${devices.length} computador${devices.length === 1 ? '' : 'es'}</span>
        </div>
        <div class="updates-filterbar">
          <label class="updates-search-field">
            <span aria-hidden="true">${icon('search')}</span>
            <input type="search" data-updates-device-search placeholder="Buscar computador ou empresa" autocomplete="off">
          </label>
          <select class="updates-filter-select" data-updates-status-filter aria-label="Filtrar status de atualização">
            <option value="all">Todos os status</option>
            <option value="pending">Com pendências</option>
            <option value="updated">Atualizados</option>
            <option value="not-scanned">Não verificados</option>
          </select>
        </div>
        <div class="table-wrap updates-table-wrap">
          <table class="updates-table updates-computers-table">
            <thead><tr><th>Computador</th><th>Empresa</th><th>Conexão</th><th>Status</th><th>Windows</th><th>Drivers</th><th>Apps</th><th>Reinício</th><th></th></tr></thead>
            <tbody>${rows || '<tr><td colspan="9" class="table-empty-cell">Nenhum computador cadastrado.</td></tr>'}</tbody>
          </table>
        </div>
        <div class="updates-filter-empty hidden" data-updates-filter-empty>Nenhum computador corresponde aos filtros.</div>
      </section>`;
  }

  function daysText(days) {
    const labels = ['Seg', 'Ter', 'Qua', 'Qui', 'Sex', 'Sáb', 'Dom'];
    return (days || []).map((day) => labels[day] || '?').join(', ');
  }

  function policyTypes(policy) {
    const values = [];
    if (policy.include_windows) values.push('Windows');
    if (policy.include_drivers) values.push('Drivers');
    if (policy.include_apps) values.push('Aplicativos');
    return values.join(', ') || 'Nenhum';
  }

  function renderPolicies(policies) {
    const rows = policies.map((policy) => `
      <tr>
        <td><strong>${CT.esc(policy.name)}</strong><span class="entity-meta">${CT.esc(policy.company_name || '—')}</span></td>
        <td>${policy.active ? statusPill('Ativa', 'resolved') : statusPill('Desativada')}</td>
        <td>${policy.auto_scan ? `A cada ${CT.esc(policy.scan_interval_hours)}h` : 'Manual'}</td>
        <td>${policy.auto_install ? statusPill('Automática', 'warning') : 'Aprovação manual'}</td>
        <td>${CT.esc(policyTypes(policy))}</td>
        <td>${CT.esc(daysText(policy.allowed_days))}<span class="entity-meta">${String(policy.start_hour).padStart(2, '0')}:00–${String(policy.end_hour).padStart(2, '0')}:00</span></td>
        <td class="table-actions-col update-row-actions"><button class="btn small" type="button" data-policy-edit="${policy.id}">Editar</button><button class="btn small danger" type="button" data-policy-delete="${policy.id}">Excluir</button></td>
      </tr>`).join('');

    return `
      <div class="updates-policy-intro">
        <div class="updates-policy-intro-icon">${icon('policy')}</div>
        <div><span class="updates-eyebrow">AUTOMAÇÃO</span><h2>Políticas de atualização</h2><p>Escolha quando o CoreControl pode verificar e instalar atualizações. Reinicializações continuam manuais.</p></div>
        <button class="btn primary" type="button" data-policy-new>Nova política</button>
      </div>
      <section class="card updates-table-card">
        <div class="updates-card-header"><div><h2>Políticas configuradas</h2><p>As regras são aplicadas aos computadores da empresa dentro da janela permitida.</p></div><span class="updates-count-badge">${policies.length}</span></div>
        <div class="table-wrap updates-table-wrap">
          <table class="updates-table"><thead><tr><th>Política</th><th>Status</th><th>Verificação</th><th>Instalação</th><th>Tipos</th><th>Janela</th><th></th></tr></thead><tbody>${rows || '<tr><td colspan="7" class="table-empty-cell">Nenhuma política criada.</td></tr>'}</tbody></table>
        </div>
      </section>`;
  }

  function itemMeta(item) {
    if (item.source === 'app') return `${item.current_version || '—'} → ${item.available_version || '—'}${item.source_name ? ` · ${item.source_name}` : ''}`;
    const parts = [];
    if (item.kb) parts.push(`KB ${item.kb}`);
    if (item.severity) parts.push(item.severity);
    if (item.downloaded) parts.push('Baixada');
    return parts.join(' · ') || 'Atualização disponível';
  }

  function sourceTitle(source) {
    return source === 'driver' ? 'Drivers' : source === 'app' ? 'Aplicativos' : 'Windows';
  }

  function sourceSubtitle(source) {
    return source === 'app' ? 'Windows Package Manager' : 'Windows Update';
  }

  function updateDetailHtml(device) {
    const groups = ['windows', 'driver', 'app'].map((source) => {
      const items = (device.items || []).filter((item) => item.source === source);
      const rows = items.map((item) => `
        <label class="update-item-row">
          <input type="checkbox" data-update-item value="${CT.esc(item.key)}">
          <span class="update-item-copy"><strong>${CT.esc(item.title || item.id)}</strong><small>${CT.esc(itemMeta(item))}</small></span>
          ${item.severity ? statusPill(item.severity, String(item.severity).toLowerCase() === 'critical' ? 'critical' : '') : ''}
        </label>`).join('');
      const sourceIcon = source === 'windows' ? 'windows' : source === 'driver' ? 'driver' : 'app';
      return `
        <section class="update-group">
          <div class="update-group-head">
            <div class="update-group-title"><span>${icon(sourceIcon)}</span><div><strong>${sourceTitle(source)}</strong><small>${sourceSubtitle(source)}</small></div></div>
            <span class="updates-count-badge">${items.length}</span>
          </div>
          ${rows || '<div class="update-group-empty"><span>✓</span><div><strong>Nenhuma atualização pendente</strong><small>Esta categoria está em dia na última verificação.</small></div></div>'}
        </section>`;
    }).join('');

    return `
      <div class="update-modal-head">
        <div><span class="updates-eyebrow">ATUALIZAÇÕES DO COMPUTADOR</span><h2>${CT.esc(device.device_name)}</h2><p>${CT.esc(device.company_name || '')}${device.hostname ? ` · ${CT.esc(device.hostname)}` : ''}</p></div>
        <div class="update-modal-state">${commandLabel(device)}<span class="status"><i class="dot ${device.online ? 'online' : 'offline'}"></i>${device.online ? 'Online' : 'Offline'}</span></div>
      </div>
      <div class="update-modal-summary">
        <span><small>Pendentes</small><b>${device.pending_total}</b></span>
        <span><small>Críticas</small><b>${device.critical_pending}</b></span>
        <span><small>Reinício</small><b>${device.reboot_required ? 'Necessário' : 'Não'}</b></span>
        <span><small>Última verificação</small><b>${device.last_scan_at ? CT.fmtDate(device.last_scan_at) : 'Nunca'}</b></span>
      </div>
      ${!device.agent_supports_updates ? `<div class="update-warning"><strong>CoreControl Agent precisa ser atualizado</strong><span>Este computador está usando ${CT.esc(device.agent_version || 'uma versão antiga')}. Reinstale/atualize o CoreControl Agent antes de gerenciar atualizações.</span></div>` : ''}
      ${device.last_error ? `<div class="update-warning"><strong>Última verificação com aviso</strong><span>${CT.esc(device.last_error)}</span></div>` : ''}
      <div class="update-select-bar"><label><input id="selectAllUpdates" type="checkbox"> Selecionar todas</label><small>As instalações são executadas pelo Agent neste computador.</small></div>
      <div class="update-groups">${groups}</div>
      <div class="modal-actions update-modal-actions"><button id="cancelModal" class="btn" type="button">Fechar</button><button id="checkAgainBtn" class="btn" type="button" ${!device.agent_supports_updates || ['queued','scanning','installing'].includes(device.status) ? 'disabled' : ''}>Verificar novamente</button><button id="installSelectedBtn" class="btn primary" type="button" disabled>Instalar selecionadas</button></div>`;
  }

  async function openUpdateDetail(deviceId) {
    try {
      const device = await CT.api(`/updates/devices/${deviceId}`);
      CT.openModal(updateDetailHtml(device));
      CT.$('#cancelModal').onclick = CT.closeModal;
      const checks = () => CT.$$('[data-update-item]', CT.$('#modal'));
      const sync = () => {
        const selected = checks().filter((input) => input.checked);
        CT.$('#installSelectedBtn').disabled = selected.length === 0 || ['queued','scanning','installing'].includes(device.status);
        const all = checks();
        CT.$('#selectAllUpdates').checked = all.length > 0 && selected.length === all.length;
      };
      CT.$('#selectAllUpdates').onchange = (event) => {
        checks().forEach((input) => { input.checked = event.target.checked; });
        sync();
      };
      checks().forEach((input) => { input.onchange = sync; });
      CT.$('#checkAgainBtn').onclick = async () => {
        try {
          await CT.api('/updates/check', { method: 'POST', body: JSON.stringify({ device_ids: [deviceId] }) });
          CT.closeModal();
          CT.toast('Verificação enviada para o computador.');
          await refreshView();
        } catch (error) { CT.toast(error.message, true); }
      };
      CT.$('#installSelectedBtn').onclick = async () => {
        const itemKeys = checks().filter((input) => input.checked).map((input) => input.value);
        if (!itemKeys.length) return;
        if (!window.confirm(`Instalar ${itemKeys.length} atualização(ões) neste computador? O CoreControl não reiniciará o Windows automaticamente.`)) return;
        try {
          await CT.api('/updates/install', { method: 'POST', body: JSON.stringify({ device_id: deviceId, item_keys: itemKeys }) });
          CT.closeModal();
          CT.toast('Instalação enviada para o computador.');
          await refreshView();
        } catch (error) { CT.toast(error.message, true); }
      };
      sync();
    } catch (error) {
      CT.toast(error.message, true);
    }
  }

  function policyModalHtml(policy, companies) {
    const editing = Boolean(policy);
    const selectedCompany = policy?.company_id || '';
    const global = CT.isGlobalAdmin();
    const companyField = global ? `<label>Empresa<select id="policyCompany" required><option value="">Selecione</option>${companies.map((company) => `<option value="${company.id}" ${Number(selectedCompany) === company.id ? 'selected' : ''}>${CT.esc(company.name)}</option>`).join('')}</select></label>` : '';
    const selectedDays = new Set(policy?.allowed_days || [0, 1, 2, 3, 4]);
    const dayLabels = ['Seg', 'Ter', 'Qua', 'Qui', 'Sex', 'Sáb', 'Dom'];
    return `
      <div class="update-policy-modal-head"><span class="updates-eyebrow">POLÍTICA DE ATUALIZAÇÃO</span><h2>${editing ? 'Editar política' : 'Nova política'}</h2><p>Defina quando o CoreControl pode verificar e instalar atualizações. Reinicializações permanecem manuais.</p></div>
      <form id="updatePolicyForm" class="stack update-policy-form">
        ${companyField}
        <label>Nome<input id="policyName" maxlength="160" required value="${CT.esc(policy?.name || '')}" placeholder="Ex.: Manutenção noturna"></label>
        <div class="update-policy-switches">
          <label><input id="policyActive" type="checkbox" ${policy?.active !== false ? 'checked' : ''}> <span><strong>Política ativa</strong><small>Permite que esta regra seja executada.</small></span></label>
          <label><input id="policyAutoScan" type="checkbox" ${policy?.auto_scan !== false ? 'checked' : ''}> <span><strong>Verificação automática</strong><small>Procura novas atualizações no intervalo definido.</small></span></label>
          <label><input id="policyAutoInstall" type="checkbox" ${policy?.auto_install ? 'checked' : ''}> <span><strong>Instalação automática</strong><small>Instala somente dentro da janela permitida.</small></span></label>
        </div>
        <div class="update-warning subtle"><strong>Reinicialização permanece manual</strong><span>Mesmo com instalação automática, o CoreControl não força o computador a reiniciar.</span></div>
        <fieldset class="update-policy-fieldset"><legend>Tipos incluídos</legend><div class="update-policy-types"><label><input id="policyWindows" type="checkbox" ${policy?.include_windows !== false ? 'checked' : ''}> Windows</label><label><input id="policyDrivers" type="checkbox" ${policy?.include_drivers ? 'checked' : ''}> Drivers</label><label><input id="policyApps" type="checkbox" ${policy?.include_apps ? 'checked' : ''}> Aplicativos</label></div></fieldset>
        <div class="form-grid-3"><label>Intervalo de verificação<input id="policyInterval" type="number" min="1" max="168" value="${policy?.scan_interval_hours || 24}"><small>Em horas</small></label><label>Início da janela<input id="policyStart" type="number" min="0" max="23" value="${policy?.start_hour ?? 1}"><small>Hora cheia</small></label><label>Fim da janela<input id="policyEnd" type="number" min="0" max="23" value="${policy?.end_hour ?? 5}"><small>Hora cheia</small></label></div>
        <fieldset class="update-policy-fieldset"><legend>Dias permitidos</legend><div class="update-days">${dayLabels.map((label, index) => `<label><input type="checkbox" data-policy-day="${index}" ${selectedDays.has(index) ? 'checked' : ''}>${label}</label>`).join('')}</div></fieldset>
        <label>Fuso horário<input id="policyTimezone" value="${CT.esc(policy?.timezone || 'America/Sao_Paulo')}" maxlength="80"></label>
        <div class="modal-actions"><button id="cancelModal" class="btn" type="button">Cancelar</button><button class="btn primary" type="submit">Salvar política</button></div>
      </form>`;
  }

  async function openPolicyModal(policy = null) {
    try {
      const companies = CT.isGlobalAdmin() ? await CT.api('/companies') : [];
      CT.openModal(policyModalHtml(policy, companies));
      CT.$('#cancelModal').onclick = CT.closeModal;
      CT.$('#updatePolicyForm').onsubmit = async (event) => {
        event.preventDefault();
        const days = CT.$$('[data-policy-day]', CT.$('#modal')).filter((input) => input.checked).map((input) => Number(input.dataset.policyDay));
        if (!days.length) return CT.toast('Selecione pelo menos um dia da semana.', true);
        if (!CT.$('#policyWindows').checked && !CT.$('#policyDrivers').checked && !CT.$('#policyApps').checked) return CT.toast('Selecione pelo menos um tipo de atualização.', true);
        const payload = {
          name: CT.$('#policyName').value,
          active: CT.$('#policyActive').checked,
          auto_scan: CT.$('#policyAutoScan').checked,
          auto_install: CT.$('#policyAutoInstall').checked,
          include_windows: CT.$('#policyWindows').checked,
          include_drivers: CT.$('#policyDrivers').checked,
          include_apps: CT.$('#policyApps').checked,
          scan_interval_hours: Number(CT.$('#policyInterval').value),
          allowed_days: days,
          start_hour: Number(CT.$('#policyStart').value),
          end_hour: Number(CT.$('#policyEnd').value),
          timezone: CT.$('#policyTimezone').value,
        };
        if (CT.isGlobalAdmin()) payload.company_id = Number(CT.$('#policyCompany').value);
        try {
          await CT.api(policy ? `/updates/policies/${policy.id}` : '/updates/policies', {
            method: policy ? 'PATCH' : 'POST',
            body: JSON.stringify(payload),
          });
          CT.closeModal();
          CT.toast('Política salva.');
          activeTab = 'policies';
          await refreshView();
        } catch (error) { CT.toast(error.message, true); }
      };
    } catch (error) { CT.toast(error.message, true); }
  }

  function setTabState(tab) {
    CT.$$('[data-module-tab]', CT.$('.page-updates')).forEach((button) => {
      const selected = button.dataset.moduleTab === tab;
      button.classList.toggle('active', selected);
      button.setAttribute('aria-selected', String(selected));
    });
    const policyButton = CT.$('#updatesPolicyBtn');
    if (policyButton) policyButton.classList.toggle('hidden', tab === 'policies');
  }

  function setScanButtonState(data) {
    const button = CT.$('#updatesCheckBtn');
    if (!button) return;
    const devices = data?.summary?.devices ?? null;
    button.disabled = devices === 0;
    button.title = devices === 0 ? 'Cadastre um computador antes de verificar atualizações.' : 'Verificar atualizações em todos os computadores disponíveis.';
  }

  function renderLoading() {
    return `
      <div class="updates-loading">
        <span class="updates-loading-spinner"></span>
        <div><strong>Carregando atualizações</strong><small>Buscando o estado mais recente dos computadores.</small></div>
      </div>`;
  }

  function filterComputerRows() {
    const page = CT.$('.page-updates');
    if (!page) return;
    const search = (page.querySelector('[data-updates-device-search]')?.value || '').trim().toLowerCase();
    const status = page.querySelector('[data-updates-status-filter]')?.value || 'all';
    const rows = Array.from(page.querySelectorAll('[data-update-device-row]'));
    let visible = 0;
    rows.forEach((row) => {
      const matchesSearch = !search || String(row.dataset.search || '').includes(search);
      const matchesStatus = status === 'all' || row.dataset.updateStatus === status;
      const show = matchesSearch && matchesStatus;
      row.classList.toggle('hidden', !show);
      if (show) visible += 1;
    });
    const empty = page.querySelector('[data-updates-filter-empty]');
    if (empty) empty.classList.toggle('hidden', visible > 0 || rows.length === 0);
  }

  async function renderTab(tab) {
    const view = CT.$('#updatesView');
    if (!view) return;
    activeTab = tab;
    setTabState(tab);
    view.innerHTML = renderLoading();

    if (tab === 'policies') {
      const policies = await CT.api('/updates/policies');
      view.innerHTML = renderPolicies(policies);
      setScanButtonState(null);
      return;
    }

    const data = await CT.api('/updates');
    setScanButtonState(data);
    view.innerHTML = tab === 'computers' ? renderComputers(data.devices) : renderOverview(data);
  }

  async function refreshView() {
    if (!CT.$('.page-updates')) return;
    try { await renderTab(activeTab); } catch (error) {
      const view = CT.$('#updatesView');
      if (view) view.innerHTML = `<div class="updates-load-error"><strong>Não foi possível carregar Atualizações</strong><span>${CT.esc(error.message || 'Tente novamente.')}</span><button class="btn" type="button" data-updates-retry>Tentar novamente</button></div>`;
      CT.toast(error.message, true);
    }
  }

  async function checkAllUpdates(button) {
    if (!button) return;
    const previous = button.innerHTML;
    button.disabled = true;
    button.classList.add('is-loading');
    button.innerHTML = '<span class="updates-loading-spinner tiny"></span><span>Enviando...</span>';
    try {
      const result = await CT.api('/updates/check', { method: 'POST', body: JSON.stringify({}) });
      const extra = result.unsupported ? ` · ${result.unsupported} Agent${result.unsupported === 1 ? '' : 's'} precisa${result.unsupported === 1 ? '' : 'm'} ser atualizado${result.unsupported === 1 ? '' : 's'}` : '';
      CT.toast(`${result.queued} verificação${result.queued === 1 ? '' : 'ões'} enviada${result.queued === 1 ? '' : 's'}${extra}.`);
      await refreshView();
    } catch (error) {
      CT.toast(error.message, true);
    } finally {
      button.classList.remove('is-loading');
      button.innerHTML = previous;
      if (CT.$('.page-updates')) {
        try {
          const data = activeTab === 'policies' ? null : await CT.api('/updates');
          setScanButtonState(data);
        } catch (_) {
          button.disabled = false;
        }
      }
    }
  }

  CT.registerPage('updates', async function renderUpdates() {
    await CT.mountPage('updates');
    const page = CT.$('.page-updates');
    if (!page) return;

    page.addEventListener('input', (event) => {
      if (event.target.matches('[data-updates-device-search]')) filterComputerRows();
    });
    page.addEventListener('change', (event) => {
      if (event.target.matches('[data-updates-status-filter]')) filterComputerRows();
    });

    page.addEventListener('click', async (event) => {
      const tab = event.target.closest('[data-module-tab]');
      if (tab) return renderTab(tab.dataset.moduleTab);
      if (event.target.closest('[data-go-policies]') || event.target.closest('#updatesPolicyBtn')) return renderTab('policies');
      if (event.target.closest('[data-go-computers]')) return renderTab('computers');
      if (event.target.closest('[data-updates-retry]')) return refreshView();

      const detail = event.target.closest('[data-update-detail]');
      if (detail) return openUpdateDetail(Number(detail.dataset.updateDetail));

      const check = event.target.closest('[data-update-check]');
      if (check) {
        check.disabled = true;
        try {
          await CT.api('/updates/check', { method: 'POST', body: JSON.stringify({ device_ids: [Number(check.dataset.updateCheck)] }) });
          CT.toast('Verificação enviada para o computador.');
          return refreshView();
        } catch (error) {
          check.disabled = false;
          return CT.toast(error.message, true);
        }
      }

      const checkAll = event.target.closest('#updatesCheckBtn');
      if (checkAll) return checkAllUpdates(checkAll);

      if (event.target.closest('[data-policy-new]')) return openPolicyModal();
      const edit = event.target.closest('[data-policy-edit]');
      if (edit) {
        try {
          const policies = await CT.api('/updates/policies');
          return openPolicyModal(policies.find((item) => item.id === Number(edit.dataset.policyEdit)) || null);
        } catch (error) { return CT.toast(error.message, true); }
      }
      const remove = event.target.closest('[data-policy-delete]');
      if (remove) {
        if (!window.confirm('Excluir esta política de atualização?')) return;
        try {
          await CT.api(`/updates/policies/${Number(remove.dataset.policyDelete)}`, { method: 'DELETE' });
          CT.toast('Política excluída.');
          return refreshView();
        } catch (error) { return CT.toast(error.message, true); }
      }
    });

    await renderTab(activeTab);
  });
})();

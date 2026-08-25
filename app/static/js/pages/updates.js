(function () {
  'use strict';

  const CT = window.CoreTuner;
  let activeTab = 'overview';

  function statusPill(label, tone = '') {
    return `<span class="pill ${tone}">${CT.esc(label)}</span>`;
  }

  function commandLabel(device) {
    if (!device.agent_supports_updates) return statusPill('Agente antigo', 'warning');
    const status = device.status;
    if (status === 'queued') return statusPill('Na fila', 'warning');
    if (status === 'scanning') return statusPill('Verificando', 'warning');
    if (status === 'installing') return statusPill('Instalando', 'warning');
    if (status === 'error') return statusPill('Erro', 'critical');
    if (!device.last_scan_at) return statusPill('Não verificado');
    if (device.pending_total === 0) return statusPill('Atualizado', 'resolved');
    return statusPill(`${device.pending_total} pendente(s)`, device.critical_pending ? 'critical' : 'warning');
  }

  function pendingCell(value, scanned) {
    if (!scanned) return '—';
    if (!value) return '<span class="update-zero">0</span>';
    return `<strong class="update-count">${CT.esc(value)}</strong>`;
  }

  function renderOverview(data) {
    const summary = data.summary;
    const recent = data.devices
      .slice()
      .sort((a, b) => (b.pending_total - a.pending_total) || (b.critical_pending - a.critical_pending))
      .slice(0, 8);
    const rows = recent.map((device) => `
      <tr>
        <td><strong>${CT.esc(device.device_name)}</strong><span class="entity-meta">${CT.esc(device.company_name || '—')}</span></td>
        <td>${commandLabel(device)}</td>
        <td>${pendingCell(device.windows_pending, device.last_scan_at)}</td>
        <td>${pendingCell(device.driver_pending, device.last_scan_at)}</td>
        <td>${pendingCell(device.app_pending, device.last_scan_at)}</td>
        <td>${device.last_scan_at ? CT.fmtDate(device.last_scan_at) : '—'}</td>
        <td class="table-actions-col"><button class="btn small" type="button" data-update-detail="${device.device_id}">Ver</button></td>
      </tr>`).join('');

    return `
      <div class="module-kpis">
        ${CT.stat('Computadores verificados', `${summary.scanned}/${summary.devices}`, 'Inventário de atualizações coletado')}
        ${CT.stat('Atualizações pendentes', summary.pending, 'Windows, drivers e aplicativos', summary.pending ? 'var(--amber)' : 'var(--green)')}
        ${CT.stat('Atualizações críticas', summary.critical, 'Classificação crítica do Windows Update', summary.critical ? 'var(--red)' : 'var(--green)')}
        ${CT.stat('Reinício necessário', summary.reboot_required, 'O CoreControl não reinicia automaticamente', summary.reboot_required ? 'var(--amber)' : 'var(--green)')}
      </div>
      <div class="module-feature-grid">
        <article class="module-feature"><div><strong>Windows Update</strong><p>Consulta as atualizações pendentes pelo mecanismo oficial do Windows e permite instalar as selecionadas.</p></div><span>Funcional</span></article>
        <article class="module-feature"><div><strong>Drivers</strong><p>Consulta drivers publicados no catálogo do Windows Update e mantém a instalação sob aprovação.</p></div><span>Funcional</span></article>
        <article class="module-feature"><div><strong>Aplicativos</strong><p>Usa o Windows Package Manager (winget) quando ele estiver disponível no computador.</p></div><span>Funcional</span></article>
        <article class="module-feature"><div><strong>Políticas</strong><p>Agenda verificações e, opcionalmente, instalações automáticas por empresa e janela de manutenção.</p></div><button class="text-action" type="button" data-go-policies>Configurar</button></article>
      </div>
      <div class="card module-section">
        <div class="card-header"><div><h2>Situação dos computadores</h2><p>A execução ocorre quando o CoreControl Agent do computador se conecta ao servidor.</p></div><span class="module-count">${summary.busy} operação(ões) em andamento</span></div>
        <div class="table-wrap"><table><thead><tr><th>Computador</th><th>Status</th><th>Windows</th><th>Drivers</th><th>Apps</th><th>Última verificação</th><th></th></tr></thead><tbody>${rows || '<tr><td colspan="7" class="table-empty-cell">Nenhum computador cadastrado.</td></tr>'}</tbody></table></div>
      </div>`;
  }

  function renderComputers(devices) {
    const rows = devices.map((device) => `
      <tr>
        <td><strong>${CT.esc(device.device_name)}</strong><span class="entity-meta">${CT.esc(device.hostname || '—')}</span></td>
        <td>${CT.esc(device.company_name || '—')}</td>
        <td>${device.online ? '<span class="status"><i class="dot online"></i>Online</span>' : '<span class="status"><i class="dot offline"></i>Offline</span>'}</td>
        <td>${commandLabel(device)}</td>
        <td>${pendingCell(device.windows_pending, device.last_scan_at)}</td>
        <td>${pendingCell(device.driver_pending, device.last_scan_at)}</td>
        <td>${pendingCell(device.app_pending, device.last_scan_at)}</td>
        <td>${device.reboot_required ? statusPill('Necessário', 'warning') : '—'}</td>
        <td class="table-actions-col update-row-actions">
          <button class="btn small" type="button" data-update-check="${device.device_id}" ${!device.agent_supports_updates || ['queued','scanning','installing'].includes(device.status) ? 'disabled' : ''}>Verificar</button>
          <button class="btn small" type="button" data-update-detail="${device.device_id}">Detalhes</button>
        </td>
      </tr>`).join('');
    return `
      <div class="card module-section">
        <div class="card-header"><div><h2>Atualizações por computador</h2><p>Consulte, aprove e instale atualizações individualmente.</p></div><span class="module-count">${devices.length} computador(es)</span></div>
        <div class="table-wrap"><table><thead><tr><th>Computador</th><th>Empresa</th><th>Conexão</th><th>Status</th><th>Windows</th><th>Drivers</th><th>Apps</th><th>Reinício</th><th></th></tr></thead><tbody>${rows || '<tr><td colspan="9" class="table-empty-cell">Nenhum computador cadastrado.</td></tr>'}</tbody></table></div>
      </div>`;
  }

  function daysText(days) {
    const labels = ['Seg', 'Ter', 'Qua', 'Qui', 'Sex', 'Sáb', 'Dom'];
    return (days || []).map((day) => labels[day] || '?').join(', ');
  }

  function policyTypes(policy) {
    const values = [];
    if (policy.include_windows) values.push('Windows');
    if (policy.include_drivers) values.push('Drivers');
    if (policy.include_apps) values.push('Apps');
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
      <div class="card module-section">
        <div class="card-header"><div><h2>Políticas de atualização</h2><p>A automação só atua dentro da janela configurada. Reinicialização continua sendo manual.</p></div><button class="btn primary" type="button" data-policy-new>Nova política</button></div>
        <div class="table-wrap"><table><thead><tr><th>Política</th><th>Status</th><th>Verificação</th><th>Instalação</th><th>Tipos</th><th>Janela</th><th></th></tr></thead><tbody>${rows || '<tr><td colspan="7" class="table-empty-cell">Nenhuma política criada.</td></tr>'}</tbody></table></div>
      </div>`;
  }

  function itemMeta(item) {
    if (item.source === 'app') return `${item.current_version || '—'} → ${item.available_version || '—'}${item.source_name ? ` · ${item.source_name}` : ''}`;
    const parts = [];
    if (item.kb) parts.push(`KB ${item.kb}`);
    if (item.severity) parts.push(item.severity);
    if (item.downloaded) parts.push('baixada');
    return parts.join(' · ') || 'Atualização disponível';
  }

  function sourceTitle(source) {
    return source === 'driver' ? 'Drivers' : source === 'app' ? 'Aplicativos' : 'Windows Update';
  }

  function updateDetailHtml(device) {
    const groups = ['windows', 'driver', 'app'].map((source) => {
      const items = (device.items || []).filter((item) => item.source === source);
      const rows = items.map((item) => `
        <label class="update-item-row">
          <input type="checkbox" data-update-item value="${CT.esc(item.key)}">
          <span><strong>${CT.esc(item.title || item.id)}</strong><small>${CT.esc(itemMeta(item))}</small></span>
          ${item.severity ? statusPill(item.severity, String(item.severity).toLowerCase() === 'critical' ? 'critical' : '') : ''}
        </label>`).join('');
      return `<section class="update-group"><div class="update-group-head"><strong>${sourceTitle(source)}</strong><span>${items.length}</span></div>${rows || '<div class="update-group-empty">Nenhuma atualização pendente.</div>'}</section>`;
    }).join('');

    return `
      <div class="update-modal-head"><div><h2>${CT.esc(device.device_name)}</h2><p>${CT.esc(device.company_name || '')} · ${CT.esc(device.hostname || '')}</p></div>${commandLabel(device)}</div>
      <div class="update-modal-summary"><span><b>${device.pending_total}</b> pendentes</span><span><b>${device.critical_pending}</b> críticas</span><span><b>${device.reboot_required ? 'Sim' : 'Não'}</b> reinício</span></div>
      ${!device.agent_supports_updates ? `<div class="update-warning"><strong>CoreControl Agent precisa ser atualizado</strong><span>Este computador está usando ${CT.esc(device.agent_version || 'uma versão antiga')}. Execute novamente o instalador do CoreControl para receber o agente 0.5.0 ou superior.</span></div>` : ''}
      ${device.last_error ? `<div class="update-warning">${CT.esc(device.last_error)}</div>` : ''}
      <div class="update-select-bar"><label><input id="selectAllUpdates" type="checkbox"> Selecionar todas</label><small>Última verificação: ${device.last_scan_at ? CT.fmtDate(device.last_scan_at) : 'nunca'}</small></div>
      <div class="update-groups">${groups}</div>
      <div class="modal-actions"><button id="cancelModal" class="btn" type="button">Fechar</button><button id="checkAgainBtn" class="btn" type="button" ${!device.agent_supports_updates || ['queued','scanning','installing'].includes(device.status) ? 'disabled' : ''}>Verificar novamente</button><button id="installSelectedBtn" class="btn primary" type="button" disabled>Instalar selecionadas</button></div>`;
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
    const selectedDays = new Set(policy?.allowed_days || [0,1,2,3,4]);
    const dayLabels = ['Seg', 'Ter', 'Qua', 'Qui', 'Sex', 'Sáb', 'Dom'];
    return `
      <h2>${editing ? 'Editar política' : 'Nova política de atualização'}</h2>
      <p>Defina quando o agente pode verificar e instalar atualizações. Reinicializações permanecem manuais.</p>
      <form id="updatePolicyForm" class="stack update-policy-form">
        ${companyField}
        <label>Nome<input id="policyName" maxlength="160" required value="${CT.esc(policy?.name || '')}" placeholder="Ex.: Manutenção noturna"></label>
        <div class="update-policy-switches">
          <label><input id="policyActive" type="checkbox" ${policy?.active !== false ? 'checked' : ''}> Política ativa</label>
          <label><input id="policyAutoScan" type="checkbox" ${policy?.auto_scan !== false ? 'checked' : ''}> Verificar automaticamente</label>
          <label><input id="policyAutoInstall" type="checkbox" ${policy?.auto_install ? 'checked' : ''}> Instalar automaticamente</label>
        </div>
        <div class="update-warning subtle"><strong>Instalação automática</strong><span>Use somente em empresas com janela de manutenção definida. O CoreControl não força reinicialização.</span></div>
        <div class="update-policy-types"><label><input id="policyWindows" type="checkbox" ${policy?.include_windows !== false ? 'checked' : ''}> Windows</label><label><input id="policyDrivers" type="checkbox" ${policy?.include_drivers ? 'checked' : ''}> Drivers</label><label><input id="policyApps" type="checkbox" ${policy?.include_apps ? 'checked' : ''}> Aplicativos</label></div>
        <div class="form-grid-3"><label>Intervalo (horas)<input id="policyInterval" type="number" min="1" max="168" value="${policy?.scan_interval_hours || 24}"></label><label>Início<input id="policyStart" type="number" min="0" max="23" value="${policy?.start_hour ?? 1}"></label><label>Fim<input id="policyEnd" type="number" min="0" max="23" value="${policy?.end_hour ?? 5}"></label></div>
        <label>Dias permitidos<div class="update-days">${dayLabels.map((label, index) => `<label><input type="checkbox" data-policy-day="${index}" ${selectedDays.has(index) ? 'checked' : ''}>${label}</label>`).join('')}</div></label>
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

  async function renderTab(tab) {
    const view = CT.$('#updatesView');
    if (!view) return;
    activeTab = tab;
    CT.$$('[data-module-tab]', CT.$('.page-updates')).forEach((button) => button.classList.toggle('active', button.dataset.moduleTab === tab));
    if (tab === 'policies') {
      const policies = await CT.api('/updates/policies');
      view.innerHTML = renderPolicies(policies);
      return;
    }
    const data = await CT.api('/updates');
    view.innerHTML = tab === 'computers' ? renderComputers(data.devices) : renderOverview(data);
  }

  async function refreshView() {
    if (!CT.$('.page-updates')) return;
    try { await renderTab(activeTab); } catch (error) { CT.toast(error.message, true); }
  }

  CT.registerPage('updates', async function renderUpdates() {
    await CT.mountPage('updates');
    const page = CT.$('.page-updates');
    page.addEventListener('click', async (event) => {
      const tab = event.target.closest('[data-module-tab]');
      if (tab) return renderTab(tab.dataset.moduleTab);
      if (event.target.closest('[data-go-policies]') || event.target.closest('#updatesPolicyBtn')) return renderTab('policies');
      const detail = event.target.closest('[data-update-detail]');
      if (detail) return openUpdateDetail(Number(detail.dataset.updateDetail));
      const check = event.target.closest('[data-update-check]');
      if (check) {
        try {
          await CT.api('/updates/check', { method: 'POST', body: JSON.stringify({ device_ids: [Number(check.dataset.updateCheck)] }) });
          CT.toast('Verificação enviada para o computador.');
          return refreshView();
        } catch (error) { return CT.toast(error.message, true); }
      }
      if (event.target.closest('#updatesCheckBtn')) {
        try {
          const result = await CT.api('/updates/check', { method: 'POST', body: JSON.stringify({}) });
          CT.toast(`${result.queued} verificação(ões) enviada(s)${result.unsupported ? ` · ${result.unsupported} agente(s) precisam ser atualizados` : ''}.`);
          return refreshView();
        } catch (error) { return CT.toast(error.message, true); }
      }
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

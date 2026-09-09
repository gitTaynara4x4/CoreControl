(function () {
  'use strict';

  const CT = window.CoreTuner;

  CT.stat = function stat(label, value, hint, color = 'var(--ink)') {
    const normalized = String(label || '').toLowerCase();
    let tone = 'blue';
    if (normalized.includes('online') || normalized.includes('disponível')) tone = 'green';
    if (normalized.includes('offline')) tone = 'red';
    if (normalized.includes('alerta') || normalized.includes('indispon')) tone = 'amber';

    return `<div class="stat-card" data-tone="${tone}"><div class="stat-top"><span class="label">${CT.esc(label)}</span></div><div class="value">${CT.esc(value)}</div><div class="hint">${CT.esc(hint)}</div></div>`;
  };

  CT.companyCard = function companyCard(company) {
    const status = company.active
      ? '<span class="pill resolved">Ativa</span>'
      : '<span class="pill critical">Desativada</span>';
    return `<article class="card company-card${company.active ? '' : ' entity-inactive'}" data-company="${company.id}"><div class="company-title"><div><h3>${CT.esc(company.name)}</h3><small class="entity-meta">Empresa #${company.id}</small></div><div class="company-title-actions">${status}<span class="pill">${company.devices_total} PCs</span></div></div><div class="company-metrics"><div class="mini"><strong>${company.devices_online}</strong><span>Online</span></div><div class="mini"><strong>${Math.max(0, company.devices_total - company.devices_online)}</strong><span>Offline</span></div><div class="mini"><strong>${company.alerts_open}</strong><span>Alertas</span></div></div></article>`;
  };

  CT.alertRow = function alertRow(alert) {
    return `<div class="info-row"><div><span class="severity-marker ${CT.esc(alert.severity)}"></span><strong>${CT.esc(alert.title)}</strong><small style="display:block;color:var(--muted);margin:5px 0 0 18px">${CT.esc(alert.device_name)}</small></div><span class="pill ${CT.esc(alert.severity)}">${alert.severity === 'critical' ? 'Crítico' : 'Atenção'}</span></div>`;
  };

  CT.remoteLabel = function remoteLabel(device) {
    if (!device.remote?.enabled) return '<span class="pill">Não configurado</span>';
    if (device.remote?.running) return '<span class="pill resolved">Disponível</span>';
    if (device.remote?.installed) return '<span class="pill warning">Parado</span>';
    return '<span class="pill critical">Não instalado</span>';
  };

  CT.healthAvailable = function healthAvailable(device) {
    return Boolean(device?.online && device?.telemetry && device?.health_score != null && Number.isFinite(Number(device.health_score)));
  };

  CT.healthMarkup = function healthMarkup(device, options = {}) {
    const prefix = options.prefix === true ? 'Saúde ' : '';
    if (!CT.healthAvailable(device)) {
      const title = CT.devicePowerIsOn?.(device)
        ? 'Aguardando comunicação atual do CoreControl Agent.'
        : 'Computador desligado. Saúde indisponível.';
      return `<span class="health unavailable" title="${CT.esc(title)}">${prefix}—</span>`;
    }
    return `<span class="health ${CT.healthClass(device.health_score)}">${prefix}${device.health_score}/100</span>`;
  };

  CT.deviceTable = function deviceTable(devices) {
    return `<div class="table-wrap"><table><thead><tr><th>Computador</th><th>Empresa / setor</th><th>Status</th><th>Saúde</th><th>CPU</th><th>Memória</th><th>Disco</th><th>Remoto</th><th>Alertas</th></tr></thead><tbody>${devices.map((device) => {
      const economy = CT.deviceEconomyModeActive?.(device);
      const status = economy
        ? '<span class="status"><i class="dot economy"></i>Modo econômico</span>'
        : `<span class="status"><i class="dot ${device.online ? 'online' : 'offline'}"></i>${device.online ? 'Online' : 'Offline'}</span>`;
      return `<tr data-device="${device.id}" class="${device.active === false ? 'entity-inactive' : ''}" style="cursor:pointer"><td><strong>${CT.esc(device.name)}</strong><small style="display:block;color:var(--muted);margin-top:3px">${CT.esc(device.hostname)}</small></td><td><strong class="table-company-name">${CT.esc(device.company_name || `Empresa #${device.company_id}`)}</strong><small style="display:block;color:var(--muted);margin-top:3px">${CT.esc(device.sector || 'Setor não informado')}</small></td><td>${device.active === false ? '<span class="pill critical">Desativado</span>' : status}</td><td>${CT.healthMarkup(device)}</td><td>${CT.fmtNum(device.telemetry?.cpu_percent)}%</td><td>${CT.fmtNum(device.telemetry?.memory_percent)}%</td><td>${CT.fmtNum(device.telemetry?.disk_percent)}%</td><td>${CT.remoteLabel(device)}</td><td>${device.alerts_open ? `<span class="pill critical">${device.alerts_open}</span>` : '—'}</td></tr>`;
    }).join('')}</tbody></table></div>`;
  };

  CT.metric = function metric(label, value, suffix, percent, extra = '') {
    const formatted = value == null ? '—' : CT.fmtNum(value);
    const level = extra || (Number(percent) >= 90 ? 'critical' : Number(percent) >= 75 ? 'warning' : '');
    return `<div class="metric ${level}"><small>${CT.esc(label)}</small><div class="big">${formatted}${value == null ? '' : suffix}</div><div class="bar"><span style="width:${Math.max(0, Math.min(100, Number(percent) || 0))}%"></span></div></div>`;
  };

  CT.info = function info(label, value) {
    return `<div class="info-row"><span>${CT.esc(label)}</span><strong>${CT.esc(value || '—')}</strong></div>`;
  };

  CT.drawChart = function drawChart(canvas, history) {
    if (!canvas) return;
    const context = canvas.getContext('2d');
    const rectangle = canvas.getBoundingClientRect();
    const ratio = window.devicePixelRatio || 1;
    canvas.width = rectangle.width * ratio;
    canvas.height = rectangle.height * ratio;
    context.scale(ratio, ratio);

    const width = rectangle.width;
    const height = rectangle.height;
    const padding = 24;
    context.clearRect(0, 0, width, height);
    const themeStyles = getComputedStyle(document.documentElement);
    context.strokeStyle = themeStyles.getPropertyValue('--border').trim() || '#e5ebf3';
    context.lineWidth = 1;

    for (let index = 0; index <= 4; index += 1) {
      const y = padding + ((height - padding * 2) * index) / 4;
      context.beginPath();
      context.moveTo(padding, y);
      context.lineTo(width - padding, y);
      context.stroke();
    }

    const series = [
      ['cpu_percent', '#1769ff'],
      ['memory_percent', '#1ca650'],
      ['disk_percent', '#7c3aed'],
    ];

    series.forEach(([key, color]) => {
      const values = history.map((item) => item?.[key]).filter((value) => value != null);
      if (values.length < 2) return;

      context.strokeStyle = color;
      context.lineWidth = 2;
      context.beginPath();
      let started = false;
      history.forEach((item, index) => {
        const value = item?.[key];
        if (value == null) return;
        const x = padding + ((width - padding * 2) * index) / Math.max(1, history.length - 1);
        const y = height - padding - ((height - padding * 2) * value) / 100;
        if (!started) {
          context.moveTo(x, y);
          started = true;
        } else {
          context.lineTo(x, y);
        }
      });
      context.stroke();
    });

    context.font = '12px Inter';
    context.fillStyle = themeStyles.getPropertyValue('--muted').trim() || '#66758d';
    context.fillText('CPU', padding, 15);
    context.fillStyle = '#1ca650';
    context.fillText('RAM', padding + 36, 15);
    context.fillStyle = '#7c3aed';
    context.fillText('Disco', padding + 76, 15);
  };

  CT.bindDeviceRows = function bindDeviceRows() {
    CT.$$('[data-device]').forEach((row) => {
      row.onclick = () => CT.navigate('device', Number(row.dataset.device));
    });
  };

  CT.bindCommonActions = function bindCommonActions() {
    CT.$$('[data-go]').forEach((button) => {
      button.onclick = () => CT.navigate(button.dataset.go);
    });
    CT.$$('[data-company]').forEach((company) => {
      company.onclick = () => CT.navigate('company', Number(company.dataset.company));
    });
    CT.$('#newCompanyBtn')?.addEventListener('click', CT.openCompanyModal);
    CT.bindDeviceRows();
  };

  CT.setAlertBadge = function setAlertBadge(number) {
    const badge = CT.$('#navAlertCount');
    badge.textContent = number;
    badge.classList.toggle('hidden', !number);
  };

  CT.mountGlobalComponents = async function mountGlobalComponents() {
    await CT.mountTemplate('#remoteViewerMount', 'components/remote-viewer.html');
    CT.$('#remoteViewerClose').onclick = CT.closeRemoteViewer;
    CT.$('#remoteViewerTabClose').onclick = CT.closeRemoteViewer;
    CT.$('#remoteViewerDockClose').onclick = CT.closeRemoteViewer;
    CT.$('#remoteViewerMinimize').onclick = CT.minimizeRemoteViewer;
    CT.$('#remoteViewerRestore').onclick = CT.restoreRemoteViewer;
    CT.$('#remoteViewerFullscreen').onclick = CT.toggleRemoteViewerFullscreen;
    document.addEventListener('keydown', (event) => {
      const viewer = CT.$('#remoteViewer');
      if (event.key === 'Escape' && document.fullscreenElement) {
        document.exitFullscreen?.();
        return;
      }
      if (event.key === 'Escape' && viewer && !viewer.classList.contains('hidden')) {
        CT.closeRemoteViewer();
      }
    });
    document.addEventListener('fullscreenchange', () => {
      CT.syncRemoteViewerFullscreenUi();
    });
  };

  CT.syncRemoteViewerDock = function syncRemoteViewerDock() {
    const viewer = CT.$('#remoteViewer');
    const dock = CT.$('#remoteViewerDock');
    if (!viewer || !dock) return;
    const minimized = viewer.classList.contains('is-minimized');
    dock.classList.toggle('hidden', !minimized);
  };

  CT.syncRemoteViewerFullscreenUi = function syncRemoteViewerFullscreenUi() {
    const btn = CT.$('#remoteViewerFullscreen');
    if (!btn) return;
    const label = CT.$('#remoteViewerFullscreenLabel');
    const value = document.fullscreenElement ? 'Restaurar' : 'Maximizar';
    if (label) label.textContent = value;
    btn.setAttribute('aria-label', value);
    btn.setAttribute('title', value);
  };

  CT.minimizeRemoteViewer = function minimizeRemoteViewer() {
    const viewer = CT.$('#remoteViewer');
    if (!viewer || viewer.classList.contains('hidden')) return;
    viewer.classList.add('is-minimized');
    CT.$('#remoteViewerDockTitle').textContent = CT.$('#remoteViewerTitle')?.textContent || 'Acesso remoto';
    CT.$('#remoteViewerDockStatus').textContent = 'Sessão minimizada';
    CT.syncRemoteViewerDock();
    document.body.classList.remove('remote-viewer-open');
  };

  CT.restoreRemoteViewer = function restoreRemoteViewer() {
    const viewer = CT.$('#remoteViewer');
    if (!viewer) return;
    viewer.classList.remove('hidden', 'is-minimized');
    document.body.classList.add('remote-viewer-open');
    CT.syncRemoteViewerDock();
  };

  CT.toggleRemoteViewerFullscreen = async function toggleRemoteViewerFullscreen() {
    const viewer = CT.$('#remoteViewer');
    if (!viewer) return;
    try {
      if (document.fullscreenElement) {
        await document.exitFullscreen?.();
      } else {
        await viewer.requestFullscreen?.();
      }
    } catch (error) {
      CT.toast('Não foi possível alternar a visualização máxima.', true);
    } finally {
      CT.syncRemoteViewerFullscreenUi();
    }
  };

  CT.closeRemoteViewer = function closeRemoteViewer() {
    CT.remoteViewerRequestToken = (CT.remoteViewerRequestToken || 0) + 1;
    const viewer = CT.$('#remoteViewer');
    const frame = CT.$('#remoteViewerFrame');
    if (document.fullscreenElement) {
      document.exitFullscreen?.().catch(() => {});
    }
    if (frame) frame.src = 'about:blank';
    if (viewer) viewer.classList.add('hidden');
    if (viewer) viewer.classList.remove('is-minimized');
    CT.syncRemoteViewerDock();
    document.body.classList.remove('remote-viewer-open');
  };

  CT.requestRemoteUrl = function requestRemoteUrl(deviceId) {
    return CT.api(`/devices/${deviceId}/remote-session`, { method: 'POST' });
  };

  CT.devicePowerIsOn = function devicePowerIsOn(device) {
    // Modo econômico NÃO é desligamento. O Windows e o Agent continuam vivos,
    // então o painel deve continuar tratando a máquina como fisicamente ligada.
    if (device?.actual_online || device?.online) return true;

    const remote = device?.remote || {};
    if (remote.enabled && remote.mesh_node_id && remote.checked_at) return Boolean(remote.mesh_connected);
    return false;
  };

  CT.deviceEconomyModeActive = function deviceEconomyModeActive(device) {
    return Boolean(device?.power?.economy_mode_active ?? device?.power?.managed_off_active ?? device?.economy_mode ?? device?.managed_off);
  };

  CT.powerPendingLabel = function powerPendingLabel(action) {
    const value = String(action || '').toLowerCase();
    if (value === 'activate' || value === 'wake') return 'Ativando...';
    if (value === 'economy') return 'Ativando modo...';
    return 'Desligando...';
  };

  CT.powerPendingFeedbackText = function powerPendingFeedbackText(action) {
    const value = String(action || '').toLowerCase();
    if (value === 'activate' || value === 'wake') return 'Restaurando o computador';
    if (value === 'economy') return 'Aplicando Modo econômico';
    return 'Aguardando confirmação do desligamento';
  };

  CT.powerPendingButtonHtml = function powerPendingButtonHtml(action) {
    return `<span class="power-spinner" aria-hidden="true"></span><span>${CT.powerPendingLabel(action)}</span>`;
  };

  CT.powerPendingFeedbackHtml = function powerPendingFeedbackHtml(action) {
    return `<span>${CT.powerPendingFeedbackText(action)}</span><span class="power-wait-dots" aria-hidden="true"><i></i><i></i><i></i></span>`;
  };

  CT.setPowerPendingButton = function setPowerPendingButton(button, action) {
    if (!button) return;
    button.disabled = true;
    button.classList.add('power-pending-button');
    button.setAttribute('aria-busy', 'true');
    button.innerHTML = CT.powerPendingButtonHtml(action);
  };

  CT.clearPowerPendingButton = function clearPowerPendingButton(button) {
    if (!button) return;
    button.classList.remove('power-pending-button');
    button.removeAttribute('aria-busy');
  };

  CT.setPowerPendingStatus = function setPowerPendingStatus(statusEl, action) {
    if (!statusEl) return;
    statusEl.classList.add('power-pending-status');
    statusEl.setAttribute('aria-busy', 'true');
    statusEl.innerHTML = `<i class="dot power-pulse"></i><span>${CT.powerPendingLabel(action)}</span>`;
  };

  CT.clearPowerPendingStatus = function clearPowerPendingStatus(statusEl) {
    if (!statusEl) return;
    statusEl.classList.remove('power-pending-status');
    statusEl.removeAttribute('aria-busy');
  };

  CT.setPowerPendingFeedback = function setPowerPendingFeedback(feedbackEl, action) {
    if (!feedbackEl) return;
    feedbackEl.classList.remove('hidden');
    feedbackEl.innerHTML = CT.powerPendingFeedbackHtml(action);
  };

  CT.clearPowerPendingFeedback = function clearPowerPendingFeedback(feedbackEl) {
    if (!feedbackEl) return;
    feedbackEl.classList.add('hidden');
    feedbackEl.innerHTML = '';
  };

  CT.confirmDeviceShutdown = function confirmDeviceShutdown(device) {
    return new Promise((resolve) => {
      const name = CT.esc(device?.name || 'este computador');
      CT.openModal(`
        <div class="danger-modal-head">
          <div class="danger-modal-icon" aria-hidden="true">!</div>
          <div>
            <h2>Desligar completamente?</h2>
            <p>Você está prestes a desligar <strong>${name}</strong> de verdade.</p>
          </div>
        </div>
        <div class="danger-summary">
          <strong>O Modo econômico é a opção recomendada</strong>
          <span>Depois de um desligamento completo, o Agent para de funcionar. O CoreControl não consegue garantir que este PC ligará remotamente sem hardware ou Wake-on-LAN compatível.</span>
        </div>
        <label class="checkbox-row" style="margin-top:14px">
          <input type="checkbox" id="confirmFullShutdownCheck">
          <span>Entendo que talvez seja necessário ligar este computador presencialmente.</span>
        </label>
        <div class="modal-actions">
          <button class="btn" type="button" id="cancelFullShutdown">Cancelar</button>
          <button class="btn danger" type="button" id="confirmFullShutdown" disabled>Desligar completamente</button>
        </div>`);

      let settled = false;
      const finish = (accepted) => {
        if (settled) return;
        settled = true;
        CT.closeModal();
        resolve(Boolean(accepted));
      };
      const check = CT.$('#confirmFullShutdownCheck');
      const confirm = CT.$('#confirmFullShutdown');
      check.onchange = () => { confirm.disabled = !check.checked; };
      CT.$('#cancelFullShutdown').onclick = () => finish(false);
      confirm.onclick = () => finish(check.checked);
    });
  };

  // Compatibility name for any stale code path. It now confirms full shutdown,
  // never the default software-only Economy Mode.
  CT.confirmDevicePowerOff = CT.confirmDeviceShutdown;

  CT.requestDevicePower = async function requestDevicePower(device, action, options = {}) {
    let normalized = String(action || '').toLowerCase();
    if (normalized === 'off') normalized = 'economy';
    if (normalized === 'wake') normalized = 'activate';
    if (!device?.id || !['economy', 'activate', 'shutdown'].includes(normalized)) throw new Error('Ação de energia inválida.');

    const readiness = await CT.api(`/devices/${device.id}/power-readiness?ts=${Date.now()}`);
    if (normalized === 'economy' && !readiness.economy_available) {
      throw new Error(readiness.reason || 'O Modo econômico não está disponível neste computador.');
    }
    if (normalized === 'activate' && !readiness.activate_available && !readiness.economy_mode_active) {
      if (CT.devicePowerIsOn(device)) return { ok: true, status: 'online', message: 'O computador já está no modo normal.' };
      throw new Error(readiness.reason || 'Este computador está realmente offline e não pode ser ativado apenas por software.');
    }
    if (normalized === 'shutdown') {
      if (!readiness.shutdown_available) throw new Error(readiness.reason || 'O desligamento completo não está disponível.');
      const accepted = await CT.confirmDeviceShutdown(device);
      if (!accepted) return null;
    }
    if (typeof options.onDispatch === 'function') options.onDispatch(readiness);
    return CT.api(`/devices/${device.id}/power?action=${encodeURIComponent(normalized)}`, { method: 'POST' });
  };

  CT.waitForDevicePower = async function waitForDevicePower(deviceId, expectedOn, options = {}) {
    const attempts = Math.max(1, Number(options.attempts || 60));
    const delayMs = Math.max(1000, Number(options.delayMs || 1500));
    const retryWake = Boolean(expectedOn && options.retryWake);
    const retryEveryAttempts = Math.max(3, Number(options.retryEveryAttempts || 10));
    const maxWakeRetries = Math.max(0, Number(options.maxWakeRetries ?? 8));
    // Limite por relógio real, não só por quantidade de tentativas. Antes, se
    // cada chamada ao MeshCentral demorasse muitos segundos, "120 tentativas"
    // podiam virar mais de cinco minutos na tela.
    const defaultElapsedMs = expectedOn ? 300000 : 90000;
    const maxElapsedMs = Math.max(15000, Number(options.maxElapsedMs || defaultElapsedMs));
    const deadline = Date.now() + maxElapsedMs;
    let wakeRetries = 0;
    let lastStatus = null;

    for (let attempt = 0; attempt < attempts && Date.now() < deadline; attempt += 1) {
      if (attempt > 0) {
        const remaining = deadline - Date.now();
        if (remaining <= 0) break;
        await new Promise((resolve) => window.setTimeout(resolve, Math.min(delayMs, remaining)));
      }
      if (Date.now() >= deadline) break;
      try {
        const status = await CT.api(`/devices/${deviceId}/remote-status?ts=${Date.now()}`);
        lastStatus = status;
        if (typeof options.onPoll === 'function') options.onPoll(status, attempt);
        const observedPowerOn = typeof status.power_on === 'boolean' ? status.power_on : Boolean(status.mesh_connected);
        if (Boolean(observedPowerOn) === Boolean(expectedOn)) return { changed: true, status };
      } catch (_) {
        // Durante inicialização/desligamento o serviço pode oscilar por alguns segundos.
      }

      // Wake-on-LAN é idempotente. Um único pacote pode se perder no roteador,
      // no broadcast ou enquanto a NIC troca de estado. Reenvie em intervalos
      // controlados, inclusive quando esta espera foi retomada depois de um F5.
      if (retryWake && attempt > 0 && attempt % retryEveryAttempts === 0 && wakeRetries < maxWakeRetries && Date.now() < deadline) {
        wakeRetries += 1;
        try {
          await CT.api(`/devices/${deviceId}/power?action=wake&ts=${Date.now()}`, { method: 'POST' });
          if (typeof options.onWakeRetry === 'function') options.onWakeRetry(wakeRetries);
        } catch (_) {
          // O polling continua; uma rota pode voltar a ficar disponível na próxima tentativa.
        }
      }
    }
    return { changed: false, status: lastStatus };
  };

  CT.openRemoteSession = async function openRemoteSession(deviceId) {
    const requestToken = (CT.remoteViewerRequestToken || 0) + 1;
    CT.remoteViewerRequestToken = requestToken;
    const viewer = CT.$('#remoteViewer');
    const frame = CT.$('#remoteViewerFrame');
    const loading = CT.$('#remoteViewerLoading');
    if (!viewer || !frame || !loading) {
      throw new Error('Visualizador remoto não foi carregado');
    }

    viewer.classList.remove('hidden', 'is-minimized');
    CT.syncRemoteViewerDock();
    CT.syncRemoteViewerFullscreenUi();
    document.body.classList.add('remote-viewer-open');
    loading.classList.remove('hidden');
    frame.classList.remove('ready');
    frame.src = 'about:blank';
    CT.$('#remoteViewerTitle').textContent = 'Acesso remoto';
    CT.$('#remoteViewerConnectionId').textContent = 'Autorizando';
    CT.$('#remoteViewerStatus').textContent = 'Gerando acesso temporário...';
    CT.$('#remoteViewerDockTitle').textContent = 'Acesso remoto';
    CT.$('#remoteViewerDockStatus').textContent = 'Gerando acesso temporário...';

    try {
      const data = await CT.requestRemoteUrl(deviceId);
      if (requestToken !== CT.remoteViewerRequestToken || viewer.classList.contains('hidden')) return;
      CT.$('#remoteViewerTitle').textContent = `Acesso remoto — ${data.device_name}`;
      CT.$('#remoteViewerConnectionId').textContent = data.device_name || `PC ${deviceId}`;
      CT.$('#remoteViewerStatus').textContent = 'Sessão autorizada pelo CoreControl';
      CT.$('#remoteViewerDockTitle').textContent = `Acesso remoto — ${data.device_name}`;
      CT.$('#remoteViewerDockStatus').textContent = 'Sessão minimizada';
      frame.onload = () => {
        loading.classList.add('hidden');
        frame.classList.add('ready');
      };
      frame.src = data.url;

      CT.$('#remoteViewerNewTab').onclick = async () => {
        const tab = window.open('about:blank', '_blank');
        try {
          const fresh = await CT.requestRemoteUrl(deviceId);
          if (tab) {
            tab.opener = null;
            tab.location = fresh.url;
          }
        } catch (error) {
          if (tab) tab.close();
          CT.toast(error.message, true);
        }
      };
      CT.toast('Conexão remota autorizada.');
    } catch (error) {
      CT.closeRemoteViewer();
      CT.toast(error.message, true);
    }
  };
})();

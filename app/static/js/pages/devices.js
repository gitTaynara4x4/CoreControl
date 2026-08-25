(function () {
  'use strict';

  const CT = window.CoreTuner;

  CT.registerPage('devices', async function renderDevices() {
    const devices = await CT.api('/devices');
    await CT.mountPage('devices');

    CT.setAlertBadge(devices.reduce((sum, device) => sum + device.alerts_open, 0));
    CT.$('#devicesCount').textContent = `${devices.length} equipamento(s) autorizado(s).`;

    const renderTable = (items, emptyMessage) => {
      CT.$('#deviceTableArea').innerHTML = items.length
        ? CT.deviceTable(items)
        : `<div class="empty">${emptyMessage}</div>`;
      CT.bindDeviceRows();
    };

    renderTable(devices, 'Nenhum computador vinculado.');
    CT.$('#deviceSearch').oninput = (event) => {
      const query = event.target.value.toLowerCase();
      const filtered = devices.filter((device) => [
        device.name,
        device.hostname,
        device.company_name,
        device.sector,
        device.location,
        device.manufacturer,
        device.model,
      ].some((value) => String(value || '').toLowerCase().includes(query)));
      renderTable(filtered, 'Nenhum resultado.');
    };
  });

  function activityVersionSupported(version) {
    const parts = String(version || '').replace(/^v/i, '').split('.').slice(0, 3).map((piece) => Number.parseInt(piece, 10) || 0);
    while (parts.length < 3) parts.push(0);
    return parts[0] > 0 || parts[1] > 6 || (parts[1] === 6 && parts[2] >= 0);
  }

  function activityDuration(seconds) {
    let value = Math.max(0, Math.round(Number(seconds) || 0));
    if (value < 60) return `${value}s`;
    const minutes = Math.floor(value / 60);
    if (minutes < 60) return `${minutes}m`;
    return `${Math.floor(minutes / 60)}h ${String(minutes % 60).padStart(2, '0')}m`;
  }

  function activitySegments(history) {
    const samples = (history || []).filter((sample) => sample?.activity?.process_name && sample.recorded_at);
    const segments = [];
    for (const sample of samples) {
      const activity = sample.activity;
      const at = new Date(sample.recorded_at);
      if (Number.isNaN(at.getTime())) continue;
      const key = `${activity.process_name}\u0000${activity.window_title || ''}`;
      const current = segments[segments.length - 1];
      if (!current || current.key !== key) {
        segments.push({
          key,
          process: activity.process_name,
          title: activity.window_title || 'Janela sem título',
          started: at,
          last: at,
        });
      } else {
        current.last = at;
      }
    }
    return segments.reverse().slice(0, 6).map((segment, index) => ({
      ...segment,
      seconds: Math.max(30, Math.round((segment.last - segment.started) / 1000) + 30),
      current: index === 0,
    }));
  }

  function renderActivityCurrent(device) {
    const activity = device.telemetry?.activity;
    const target = CT.$('#activityCurrent');
    if (!target) return;
    if (!activity?.process_name) {
      target.innerHTML = `<div class="activity-empty">${activityVersionSupported(device.agent_version) ? 'Aguardando a próxima amostra de atividade do computador.' : 'Atualize o CoreControl Agent para 0.6.0 ou superior para receber atividade.'}</div>`;
      return;
    }
    target.innerHTML = `
      <div class="activity-current-main">
        <span class="activity-current-icon">●</span>
        <div class="activity-current-copy"><strong>${CT.esc(activity.process_name)}</strong><span>${CT.esc(activity.window_title || 'Janela em primeiro plano')}</span></div>
        <span class="activity-current-meta">Em uso · ${CT.fmtDate(device.telemetry.recorded_at)}</span>
      </div>`;
  }

  function renderActivityTimeline(history) {
    const target = CT.$('#activityTimeline');
    if (!target) return;
    const items = activitySegments(history);
    target.innerHTML = items.length ? items.map((item) => `
      <div class="activity-event ${item.current ? 'current' : ''}">
        <i></i><div><strong>${CT.esc(item.process)}</strong><small>${CT.esc(item.title)}</small></div>
        <time>${activityDuration(item.seconds)}</time>
      </div>`).join('') : '<div class="activity-empty">A linha do tempo aparecerá quando o Agent registrar mudanças de aplicativo.</div>';
  }

  function renderActivitySnapshot(snapshot) {
    const status = CT.$('#activitySnapshotStatus');
    const area = CT.$('#activityAppsArea');
    if (!status || !area) return;
    const command = snapshot?.command;
    if (!snapshot?.agent_supports_activity) {
      status.textContent = 'Agent antigo';
      area.innerHTML = '<div class="activity-empty">Execute novamente o CoreControl Setup para instalar o Agent 0.6.0 ou superior.</div>';
      return;
    }
    if (!command) {
      status.textContent = 'Ainda não consultado';
      area.innerHTML = '<div class="activity-empty">Clique em “Atualizar aplicativos” para consultar as janelas abertas neste computador.</div>';
      return;
    }
    if (['queued', 'running'].includes(command.status)) {
      status.textContent = command.status === 'running' ? 'Coletando...' : 'Na fila...';
      area.innerHTML = '<div class="activity-empty">O Agent vai devolver a lista na próxima comunicação com a Central.</div>';
      return;
    }
    if (command.status === 'failed') {
      status.textContent = 'Falhou';
      area.innerHTML = `<div class="activity-empty">${CT.esc(command.error || 'Não foi possível consultar os aplicativos.')}</div>`;
      return;
    }
    const apps = command.result?.apps || [];
    const browserTabs = command.result?.browser_tabs || [];
    status.textContent = command.finished_at ? `Atualizado ${CT.fmtDate(command.finished_at)}` : 'Atualizado';
    const tabsHtml = browserTabs.length ? `<div class="activity-browser-block"><div class="activity-browser-title">Abas do navegador</div><div class="table-wrap"><table class="activity-table"><thead><tr><th>Página</th><th>Site</th><th>Status</th></tr></thead><tbody>${browserTabs.map((tab) => `
      <tr><td><div class="activity-app-name"><i class="${tab.active ? 'focused' : ''}"></i><span>${CT.esc(tab.title || 'Página')}</span></div></td><td><div class="activity-window-title" title="${CT.esc(tab.url || '')}">${CT.esc(tab.domain || '—')}</div></td><td>${tab.active ? '<span class="pill resolved">Em uso</span>' : '<span class="pill">Aba aberta</span>'}</td></tr>`).join('')}</tbody></table></div></div>` : '';
    const appsHtml = apps.length ? `<div class="activity-browser-title">Aplicativos com janela aberta</div><div class="table-wrap"><table class="activity-table"><thead><tr><th>Aplicativo</th><th>Janela</th><th>CPU</th><th>Memória</th><th>Status</th></tr></thead><tbody>${apps.slice(0, 14).map((app) => `
      <tr><td><div class="activity-app-name"><i class="${app.focused ? 'focused' : ''}"></i><span>${CT.esc(app.process_name || '—')}</span></div></td><td><div class="activity-window-title" title="${CT.esc(app.window_title || '')}">${CT.esc(app.window_title || '—')}</div></td><td>${CT.fmtNum(app.cpu_percent, 1)}%</td><td>${CT.fmtNum(app.memory_mb, 0)} MB</td><td>${app.focused ? '<span class="pill resolved">Em uso</span>' : '<span class="pill">Aberto</span>'}</td></tr>`).join('')}</tbody></table></div>` : '';
    area.innerHTML = tabsHtml + appsHtml || '<div class="activity-empty">Nenhuma atividade visível foi encontrada.</div>';
  }

  async function loadActivitySnapshot(deviceId) {
    try {
      const snapshot = await CT.api(`/devices/${deviceId}/activity/snapshot`);
      renderActivitySnapshot(snapshot);
      return snapshot;
    } catch (error) {
      const area = CT.$('#activityAppsArea');
      if (area) area.innerHTML = `<div class="activity-empty">${CT.esc(error.message || 'Falha ao carregar atividade.')}</div>`;
      return null;
    }
  }

  async function requestActivitySnapshot(device) {
    const button = CT.$('#activityRefreshBtn');
    if (!button || button.disabled) return;
    button.disabled = true;
    button.textContent = 'Solicitando...';
    try {
      const response = await CT.api(`/devices/${device.id}/activity/snapshot`, { method: 'POST' });
      renderActivitySnapshot(response);
      const commandId = response.command?.id;
      for (let attempt = 0; attempt < 18; attempt += 1) {
        await new Promise((resolve) => setTimeout(resolve, 2000));
        const current = await loadActivitySnapshot(device.id);
        if (!current?.command || current.command.id !== commandId || !['queued', 'running'].includes(current.command.status)) break;
      }
    } catch (error) {
      CT.toast(error.message || 'Não foi possível consultar a atividade.', 'error');
    } finally {
      button.disabled = !device.online || !activityVersionSupported(device.agent_version);
      button.textContent = 'Atualizar aplicativos';
    }
  }

  CT.registerPage('device', async function renderDevice() {
    const device = await CT.api(`/devices/${CT.state.selectedDevice}`);
    CT.state.selectedDevice = device.id;
    await CT.mountPage('device');

    CT.$('#pageTitle').textContent = device.name;
    const telemetry = device.telemetry || {};

    CT.$('#deviceOnlineStatus').innerHTML = `<i class="dot ${device.online ? 'online' : 'offline'}"></i>${device.online ? 'Online' : 'Offline'}`;
    CT.$('#deviceHealthStatus').className = `health ${CT.healthClass(device.health_score)}`;
    CT.$('#deviceHealthStatus').textContent = `Saúde ${device.health_score}/100`;

    CT.$('#deviceMetrics').innerHTML = [
      CT.metric('Processador', telemetry.cpu_percent, '%', telemetry.cpu_percent),
      CT.metric('Memória RAM', telemetry.memory_percent, '%', telemetry.memory_percent),
      CT.metric('Disco principal', telemetry.disk_percent, '%', telemetry.disk_percent),
      CT.metric(
        'Temperatura',
        telemetry.temperature_c,
        ' °C',
        telemetry.temperature_c ? Math.min(100, telemetry.temperature_c) : 0,
        telemetry.temperature_c >= 85 ? 'critical' : '',
      ),
    ].join('');

    renderActivityCurrent(device);
    renderActivityTimeline(device.history);
    const activityButton = CT.$('#activityRefreshBtn');
    activityButton.disabled = !device.online || !activityVersionSupported(device.agent_version);
    activityButton.title = !activityVersionSupported(device.agent_version) ? 'Atualize o CoreControl Agent para 0.6.0 ou superior.' : !device.online ? 'O computador precisa estar online.' : '';
    activityButton.onclick = () => requestActivitySnapshot(device);
    loadActivitySnapshot(device.id);

    CT.$('#deviceHistoryCount').textContent = `Últimas ${device.history.length} amostras recebidas.`;
    CT.$('#deviceCompanyName').textContent = device.company_name;
    CT.$('#deviceIdentity').innerHTML = [
      CT.info('Fabricante', device.manufacturer),
      CT.info('Modelo', device.model),
      CT.info('Número de série', device.serial_number),
      CT.info('Windows', `${device.os_name || ''} ${device.os_version || ''}`.trim()),
      CT.info('IP local', telemetry.ip_local),
      CT.info('Último contato', CT.fmtDate(device.last_seen)),
      CT.info('Agente', device.agent_version),
      CT.info('Perfil aplicado', device.profile || 'Nenhum'),
    ].join('');

    CT.$('#deviceProtection').innerHTML = [
      CT.info('Memória instalada', telemetry.memory_total_gb == null ? '—' : `${CT.fmtNum(telemetry.memory_total_gb, 1)} GB`),
      CT.info('Memória usada', telemetry.memory_used_gb == null ? '—' : `${CT.fmtNum(telemetry.memory_used_gb, 1)} GB`),
      CT.info('Disco total', telemetry.disk_total_gb == null ? '—' : `${CT.fmtNum(telemetry.disk_total_gb, 1)} GB`),
      CT.info('Espaço livre', telemetry.disk_free_gb == null ? '—' : `${CT.fmtNum(telemetry.disk_free_gb, 1)} GB`),
      CT.info('Microsoft Defender', telemetry.defender_active == null ? 'Não informado' : telemetry.defender_active ? 'Ativo' : 'Desativado'),
      CT.info('Firewall', telemetry.firewall_active == null ? 'Não informado' : telemetry.firewall_active ? 'Ativo' : 'Desativado'),
    ].join('');

    CT.$('#deviceRemoteLabel').innerHTML = CT.remoteLabel(device);
    CT.$('#deviceRemoteText').textContent = device.remote?.running
      ? 'O agente remoto está conectado e pronto para suporte.'
      : device.remote?.installed
        ? 'O módulo está instalado, mas não está conectado.'
        : 'Instale novamente pelo CoreControl Setup autorizando o acesso remoto.';

    const remoteButton = CT.$('#remoteAccessBtn');
    remoteButton.disabled = !device.remote?.available;
    remoteButton.addEventListener('click', () => CT.openRemoteSession(device.id));
    CT.$('#backDevices').onclick = () => CT.navigate('devices');
    const editDeviceBtn = CT.$('#editDeviceBtn');
    if (['global_admin', 'platform_admin', 'company_admin'].includes(CT.state.user.role)) {
      editDeviceBtn.classList.remove('hidden');
      editDeviceBtn.onclick = () => CT.openDeviceEditModal(device);
    }
    CT.drawChart(CT.$('#telemetryChart'), device.history);
  });
})();

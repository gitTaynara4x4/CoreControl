(function () {
  'use strict';

  const CT = window.CoreTuner;

  CT.stat = function stat(label, value, hint, color = 'var(--ink)') {
    return `<div class="stat-card"><div class="label">${CT.esc(label)}</div><div class="value" style="color:${color}">${CT.esc(value)}</div><div class="hint">${CT.esc(hint)}</div></div>`;
  };

  CT.companyCard = function companyCard(company) {
    return `<article class="card company-card" data-company="${company.id}"><div class="company-title"><h3>${CT.esc(company.name)}</h3><span class="pill">${company.devices_total} PCs</span></div><div class="company-metrics"><div class="mini"><strong>${company.devices_online}</strong><span>Online</span></div><div class="mini"><strong>${company.devices_total - company.devices_online}</strong><span>Offline</span></div><div class="mini"><strong>${company.alerts_open}</strong><span>Alertas</span></div></div></article>`;
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

  CT.deviceTable = function deviceTable(devices) {
    return `<div class="table-wrap"><table><thead><tr><th>Computador</th><th>Empresa/Setor</th><th>Status</th><th>Saúde</th><th>CPU</th><th>Memória</th><th>Disco</th><th>Remoto</th><th>Alertas</th></tr></thead><tbody>${devices.map((device) => `<tr data-device="${device.id}" style="cursor:pointer"><td><strong>${CT.esc(device.name)}</strong><small style="display:block;color:var(--muted);margin-top:3px">${CT.esc(device.hostname)}</small></td><td>${CT.esc(device.sector || 'Não informado')}</td><td><span class="status"><i class="dot ${device.online ? 'online' : 'offline'}"></i>${device.online ? 'Online' : 'Offline'}</span></td><td><span class="health ${CT.healthClass(device.health_score)}">${device.health_score}/100</span></td><td>${CT.fmtNum(device.telemetry?.cpu_percent)}%</td><td>${CT.fmtNum(device.telemetry?.memory_percent)}%</td><td>${CT.fmtNum(device.telemetry?.disk_percent)}%</td><td>${CT.remoteLabel(device)}</td><td>${device.alerts_open ? `<span class="pill critical">${device.alerts_open}</span>` : '—'}</td></tr>`).join('')}</tbody></table></div>`;
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
    context.strokeStyle = '#e5ebf3';
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

    context.font = '12px Segoe UI';
    context.fillStyle = '#66758d';
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
    document.addEventListener('keydown', (event) => {
      const viewer = CT.$('#remoteViewer');
      if (event.key === 'Escape' && viewer && !viewer.classList.contains('hidden')) {
        CT.closeRemoteViewer();
      }
    });
  };

  CT.closeRemoteViewer = function closeRemoteViewer() {
    const viewer = CT.$('#remoteViewer');
    const frame = CT.$('#remoteViewerFrame');
    if (frame) frame.src = 'about:blank';
    if (viewer) viewer.classList.add('hidden');
    document.body.classList.remove('remote-viewer-open');
  };

  CT.requestRemoteUrl = function requestRemoteUrl(deviceId) {
    return CT.api(`/devices/${deviceId}/remote-session`, { method: 'POST' });
  };

  CT.openRemoteSession = async function openRemoteSession(deviceId) {
    const viewer = CT.$('#remoteViewer');
    const frame = CT.$('#remoteViewerFrame');
    const loading = CT.$('#remoteViewerLoading');
    if (!viewer || !frame || !loading) {
      throw new Error('Visualizador remoto não foi carregado');
    }

    viewer.classList.remove('hidden');
    document.body.classList.add('remote-viewer-open');
    loading.classList.remove('hidden');
    frame.classList.remove('ready');
    frame.src = 'about:blank';
    CT.$('#remoteViewerTitle').textContent = 'Acesso remoto';
    CT.$('#remoteViewerStatus').textContent = 'Gerando acesso temporário...';

    try {
      const data = await CT.requestRemoteUrl(deviceId);
      CT.$('#remoteViewerTitle').textContent = `Acesso remoto — ${data.device_name}`;
      CT.$('#remoteViewerStatus').textContent = 'Sessão autorizada pelo CoreTuner';
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

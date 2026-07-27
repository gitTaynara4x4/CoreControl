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
        device.sector,
        device.manufacturer,
        device.model,
      ].some((value) => String(value || '').toLowerCase().includes(query)));
      renderTable(filtered, 'Nenhum resultado.');
    };
  });

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
        : 'Instale novamente pelo CoreTuner Setup autorizando o acesso remoto.';

    const remoteButton = CT.$('#remoteAccessBtn');
    remoteButton.disabled = !device.remote?.available;
    remoteButton.addEventListener('click', () => CT.openRemoteSession(device.id));
    CT.$('#backDevices').onclick = () => CT.navigate('devices');
    CT.drawChart(CT.$('#telemetryChart'), device.history);
  });
})();

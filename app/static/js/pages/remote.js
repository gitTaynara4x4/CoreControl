(function () {
  'use strict';

  const CT = window.CoreTuner;

  CT.registerPage('remote', async function renderRemote() {
    const devices = await CT.api('/devices');
    await CT.mountPage('remote');

    const ready = devices.filter((device) => device.remote?.available);
    const unavailable = devices.filter((device) => !device.remote?.available);

    CT.$('#remoteStats').innerHTML = [
      CT.stat('Remoto disponível', ready.length, 'Prontos para conexão', 'var(--green)'),
      CT.stat('Indisponíveis', unavailable.length, 'Sem agente conectado', 'var(--amber)'),
      CT.stat('Total', devices.length, 'Computadores autorizados'),
    ].join('');

    CT.$('#remoteDevicesArea').innerHTML = devices.length
      ? `<div class="remote-list">${devices.map((device) => `<div class="remote-row"><div><strong>${CT.esc(device.name)}</strong><small>${CT.esc(device.hostname)} · ${CT.esc(device.sector || 'Sem setor')}</small></div>${CT.remoteLabel(device)}<button class="btn small ${device.remote?.available ? 'primary' : ''}" data-remote="${device.id}" ${device.remote?.available ? '' : 'disabled'}>Acessar</button></div>`).join('')}</div>`
      : '<div class="empty">Nenhum computador vinculado.</div>';

    CT.$$('[data-remote]').forEach((button) => {
      button.onclick = () => CT.openRemoteSession(Number(button.dataset.remote));
    });
  });
})();

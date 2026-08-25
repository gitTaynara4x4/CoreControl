(function () {
  'use strict';
  const CT = window.CoreTuner;

  function overview(devices) {
    const online = devices.filter((device) => device.online).length;
    const withIp = devices.filter((device) => device.telemetry?.ip_local).length;
    const rows = devices.map((device) => `
      <tr>
        <td><strong>${CT.esc(device.name)}</strong><span class="entity-meta">${CT.esc(device.hostname || '—')}</span></td>
        <td>${CT.esc(device.company_name || '—')}</td>
        <td>${CT.esc(device.telemetry?.ip_local || '—')}</td>
        <td>${CT.esc(device.telemetry?.network_name || '—')}</td>
        <td>—</td><td>—</td>
        <td>${device.online ? '<span class="status"><i class="dot online"></i>Online</span>' : '<span class="status"><i class="dot offline"></i>Offline</span>'}</td>
        <td class="table-actions-col"><button class="btn small" data-open-device="${device.id}">Detalhes</button></td>
      </tr>`).join('');
    return `
      <div class="module-kpis">
        ${CT.stat('Online', online, 'Computadores comunicando', 'var(--green)')}
        ${CT.stat('Offline', devices.length - online, 'Sem comunicação', 'var(--red)')}
        ${CT.stat('IP identificado', withIp, 'Última telemetria recebida')}
        ${CT.stat('Problemas de rede', '—', 'Aguardando testes ativos', 'var(--amber)')}
      </div>
      <div class="card module-section"><div class="card-header"><div><h2>Visão geral da rede</h2><p>Dados já disponíveis pela telemetria; gateway e DNS entram na próxima coleta do agente.</p></div><span class="module-count">${devices.length} computador(es)</span></div><div class="table-wrap"><table><thead><tr><th>Computador</th><th>Empresa</th><th>IP</th><th>Rede</th><th>Gateway</th><th>DNS</th><th>Status</th><th></th></tr></thead><tbody>${rows || '<tr><td colspan="8" class="table-empty-cell">Nenhum computador cadastrado.</td></tr>'}</tbody></table></div></div>`;
  }

  function tests(devices) {
    const options = devices.map((device) => `<option value="${device.id}">${CT.esc(device.name)} — ${CT.esc(device.company_name || '—')}</option>`).join('');
    return `<div class="module-layout-2"><div class="card module-section"><div class="card-header"><div><h2>Ferramentas de rede</h2><p>Escolha o computador e o tipo de diagnóstico.</p></div></div><div class="module-form"><label>Computador<select id="networkTestDevice"><option value="">Selecione...</option>${options}</select></label><label>Teste<select id="networkTestType"><option>Ping</option><option>Traceroute</option><option>DNS Lookup</option><option>Gateway</option><option>Internet</option><option>Porta TCP</option></select></label><button class="btn primary" data-network-run>Executar teste</button></div></div><div class="card module-section"><div class="card-header"><div><h2>Resultado</h2><p>A saída do diagnóstico aparecerá aqui.</p></div></div><div class="module-console">Nenhum teste executado.</div></div></div>`;
  }

  function discovered() {
    return `<div class="card module-section"><div class="card-header"><div><h2>Dispositivos descobertos</h2><p>PCs, impressoras, roteadores, switches, câmeras e outros equipamentos.</p></div></div><div class="table-wrap"><table><thead><tr><th>Nome</th><th>IP</th><th>MAC</th><th>Fabricante</th><th>Tipo</th><th>Detectado por</th></tr></thead><tbody><tr><td colspan="6" class="table-empty-cell">A descoberta ativa de rede ainda não foi executada.</td></tr></tbody></table></div></div>`;
  }

  CT.registerPage('network', async function renderNetwork() {
    const devices = await CT.api('/devices');
    await CT.mountPage('network');
    const page = CT.$('.page-network');
    const view = CT.$('#networkView');
    const render = (tab) => {
      CT.$$('[data-module-tab]', page).forEach((button) => button.classList.toggle('active', button.dataset.moduleTab === tab));
      view.innerHTML = tab === 'tests' ? tests(devices) : tab === 'devices' ? discovered() : overview(devices);
    };
    page.addEventListener('click', (event) => {
      const tab = event.target.closest('[data-module-tab]');
      if (tab) return render(tab.dataset.moduleTab);
      if (event.target.closest('#networkTestBtn')) return render('tests');
      const device = event.target.closest('[data-open-device]');
      if (device) return CT.navigate('device', Number(device.dataset.openDevice));
      if (event.target.closest('[data-network-run]')) CT.toast('O executor de testes será conectado ao agente na próxima etapa.');
    });
    render('overview');
  });
})();

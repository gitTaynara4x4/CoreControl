(function () {
  'use strict';

  const CT = window.CoreTuner;
  const NETWORK_TABS = new Set(['overview', 'tests', 'devices']);

  const icons = {
    network: '<svg viewBox="0 0 24 24"><circle cx="12" cy="5" r="2.2"/><circle cx="5" cy="18" r="2.2"/><circle cx="19" cy="18" r="2.2"/><path d="M12 7.2v4M12 11.2 5.8 15.8M12 11.2l6.2 4.6"/></svg>',
    online: '<svg viewBox="0 0 24 24"><path d="M5 12.5a7 7 0 0 1 14 0"/><path d="M8 15.5a4.3 4.3 0 0 1 8 0"/><circle cx="12" cy="19" r="1.2"/></svg>',
    offline: '<svg viewBox="0 0 24 24"><path d="M4 4l16 16"/><path d="M6.2 10.3A7 7 0 0 1 18 9"/><path d="M8.7 14.5a4.3 4.3 0 0 1 5.8-.8"/><path d="M12 19h.01"/></svg>',
    ip: '<svg viewBox="0 0 24 24"><rect x="4" y="5" width="16" height="14" rx="2"/><path d="M8 9h8M8 13h5"/></svg>',
    alert: '<svg viewBox="0 0 24 24"><path d="M12 4 3.8 19h16.4L12 4Z"/><path d="M12 9v4M12 16.5h.01"/></svg>',
    computer: '<svg viewBox="0 0 24 24"><rect x="3.5" y="4.5" width="17" height="12" rx="2"/><path d="M8 20h8M12 16.5V20"/></svg>',
    route: '<svg viewBox="0 0 24 24"><circle cx="6" cy="18" r="2"/><circle cx="18" cy="6" r="2"/><path d="M7.5 16.5 16.5 7.5"/><path d="M6 6h5v5"/></svg>',
    dns: '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="8"/><path d="M4 12h16M12 4c2.2 2.2 3.2 4.8 3.2 8S14.2 17.8 12 20c-2.2-2.2-3.2-4.8-3.2-8S9.8 6.2 12 4Z"/></svg>',
    test: '<svg viewBox="0 0 24 24"><path d="M9 3h6M10 3v6l-5 8.4A2.4 2.4 0 0 0 7 21h10a2.4 2.4 0 0 0 2-3.6L14 9V3"/><path d="M8 15h8"/></svg>',
    search: '<svg viewBox="0 0 24 24"><circle cx="10.5" cy="10.5" r="6"/><path d="m15 15 5 5"/></svg>',
    device: '<svg viewBox="0 0 24 24"><rect x="4" y="4" width="16" height="16" rx="3"/><path d="M8 8h8M8 12h5M8 16h3"/></svg>',
    clock: '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="8"/><path d="M12 7.5V12l3 2"/></svg>',
    check: '<svg viewBox="0 0 24 24"><path d="m5 12.5 4.2 4.2L19 7"/></svg>',
  };

  function readTab() {
    const value = new URL(window.location.href).searchParams.get('tab');
    return NETWORK_TABS.has(value) ? value : 'overview';
  }

  function writeTab(tab) {
    if (!NETWORK_TABS.has(tab) || CT.state.page !== 'network') return;
    const url = new URL(window.location.href);
    url.searchParams.set('page', 'network');
    url.searchParams.set('tab', tab);
    window.history.replaceState({ corecontrol: true, page: 'network', tab }, '', url);
  }

  function deviceOnline(device) {
    return Boolean(device?.online);
  }

  function telemetry(device, key, fallback = '—') {
    const value = device?.telemetry?.[key];
    return value == null || value === '' ? fallback : value;
  }

  function networkType(device) {
    return telemetry(device, 'network_name', telemetry(device, 'connection_type', '—'));
  }

  function statusBadge(device) {
    if (deviceOnline(device)) return '<span class="network-status online"><i></i>Online</span>';
    return '<span class="network-status offline"><i></i>Offline</span>';
  }

  function kpi(label, value, detail, iconName, tone = '') {
    return `
      <article class="network-kpi ${tone}">
        <span class="network-kpi-icon" aria-hidden="true">${icons[iconName] || icons.network}</span>
        <div class="network-kpi-copy">
          <span>${CT.esc(label)}</span>
          <strong>${CT.esc(value)}</strong>
          <small>${CT.esc(detail)}</small>
        </div>
      </article>`;
  }

  function connectionHealth(devices) {
    const total = devices.length;
    const online = devices.filter(deviceOnline).length;
    if (!total) return { tone: 'neutral', title: 'Nenhum computador cadastrado', text: 'Cadastre um computador para começar a acompanhar a conectividade da rede.' };
    if (online === total) return { tone: 'good', title: 'Rede operacional', text: `${online} de ${total} computador${total === 1 ? '' : 'es'} comunicando normalmente com o CoreControl.` };
    if (!online) return { tone: 'critical', title: 'Sem comunicação', text: `Nenhum dos ${total} computadores está comunicando com o CoreControl neste momento.` };
    return { tone: 'attention', title: 'Conectividade parcial', text: `${total - online} computador${total - online === 1 ? '' : 'es'} sem comunicação neste momento.` };
  }

  function overview(devices) {
    const online = devices.filter(deviceOnline).length;
    const offline = devices.length - online;
    const withIp = devices.filter((device) => telemetry(device, 'ip_local', '')).length;
    const state = connectionHealth(devices);
    const rows = devices.map((device) => `
      <tr>
        <td>
          <div class="network-device-cell">
            <span class="network-device-icon" aria-hidden="true">${icons.computer}</span>
            <div><strong>${CT.esc(device.name)}</strong><small>${CT.esc(device.hostname || '—')}</small></div>
          </div>
        </td>
        <td>${CT.esc(device.company_name || '—')}</td>
        <td><span class="network-mono">${CT.esc(telemetry(device, 'ip_local'))}</span></td>
        <td>${CT.esc(networkType(device))}</td>
        <td><span class="network-muted">${CT.esc(telemetry(device, 'gateway'))}</span></td>
        <td><span class="network-muted">${CT.esc(telemetry(device, 'dns'))}</span></td>
        <td>${statusBadge(device)}</td>
        <td class="table-actions-col"><button class="btn small" type="button" data-open-device="${device.id}">Detalhes</button></td>
      </tr>`).join('');

    return `
      <section class="network-status-card ${state.tone}">
        <div class="network-status-main">
          <span class="network-status-icon" aria-hidden="true">${state.tone === 'good' ? icons.online : state.tone === 'critical' ? icons.offline : icons.network}</span>
          <div>
            <span class="network-eyebrow">STATUS DA REDE</span>
            <h2>${CT.esc(state.title)}</h2>
            <p>${CT.esc(state.text)}</p>
          </div>
        </div>
        <div class="network-status-meta">
          <span><b>${online}</b> online</span>
          <span><b>${offline}</b> offline</span>
        </div>
      </section>

      <div class="network-kpi-grid">
        ${kpi('Online', online, 'Computadores comunicando', 'online', 'good')}
        ${kpi('Offline', offline, offline ? 'Precisam de atenção' : 'Nenhum sem comunicação', 'offline', offline ? 'critical' : '')}
        ${kpi('IP identificado', `${withIp}/${devices.length}`, 'Telemetria de rede recebida', 'ip')}
        ${kpi('Diagnóstico', devices.length ? 'Pronto' : '—', 'Testes sob demanda', 'test', devices.length ? 'good' : '')}
      </div>

      <section class="card network-table-card">
        <div class="network-card-header">
          <div>
            <span class="network-eyebrow">COMPUTADORES</span>
            <h2>Visão geral da rede</h2>
            <p>Endereçamento e conectividade dos computadores monitorados.</p>
          </div>
          <span class="network-count-badge">${devices.length} computador${devices.length === 1 ? '' : 'es'}</span>
        </div>
        <div class="network-filterbar">
          <label class="network-search-field">
            <span aria-hidden="true">${icons.search}</span>
            <input id="networkDeviceSearch" type="search" placeholder="Buscar computador, empresa ou IP" autocomplete="off">
          </label>
          <select id="networkStatusFilter" class="network-filter-select" aria-label="Filtrar por status">
            <option value="all">Todos os status</option>
            <option value="online">Online</option>
            <option value="offline">Offline</option>
          </select>
        </div>
        <div class="table-wrap network-table-wrap">
          <table class="network-table">
            <thead><tr><th>Computador</th><th>Empresa</th><th>IP</th><th>Conexão</th><th>Gateway</th><th>DNS</th><th>Status</th><th></th></tr></thead>
            <tbody id="networkDeviceRows">${rows || '<tr><td colspan="8"><div class="network-inline-empty"><span>'+icons.computer+'</span><strong>Nenhum computador cadastrado</strong><small>Os dispositivos monitorados aparecerão aqui.</small></div></td></tr>'}</tbody>
          </table>
        </div>
      </section>`;
  }

  function testTypeOptions() {
    return [
      ['internet', 'Internet', 'Verifica acesso externo e latência básica.'],
      ['ping', 'Ping', 'Testa resposta de um host ou endereço IP.'],
      ['dns', 'DNS', 'Valida resolução de nomes.'],
      ['gateway', 'Gateway', 'Verifica o gateway padrão do computador.'],
      ['traceroute', 'Traceroute', 'Mostra o caminho até o destino.'],
      ['tcp', 'Porta TCP', 'Testa acesso a uma porta específica.'],
    ];
  }

  function tests(devices) {
    const options = devices.map((device) => `<option value="${device.id}" ${deviceOnline(device) ? '' : 'disabled'}>${CT.esc(device.name)}${device.company_name ? ` — ${CT.esc(device.company_name)}` : ''}${deviceOnline(device) ? '' : ' (offline)'}</option>`).join('');
    const typeCards = testTypeOptions().map(([value, label, description], index) => `
      <label class="network-test-type ${index === 0 ? 'selected' : ''}">
        <input type="radio" name="networkTestType" value="${value}" ${index === 0 ? 'checked' : ''}>
        <span class="network-test-type-icon" aria-hidden="true">${value === 'dns' ? icons.dns : value === 'gateway' || value === 'traceroute' ? icons.route : icons.test}</span>
        <span><strong>${CT.esc(label)}</strong><small>${CT.esc(description)}</small></span>
      </label>`).join('');

    return `
      <section class="network-test-intro">
        <span class="network-test-intro-icon" aria-hidden="true">${icons.test}</span>
        <div><span class="network-eyebrow">DIAGNÓSTICO DE REDE</span><h2>Teste a conectividade de um computador</h2><p>Escolha o computador e o diagnóstico. O resultado fica vinculado à execução para facilitar a análise técnica.</p></div>
      </section>

      <div class="network-test-layout">
        <section class="card network-test-config">
          <div class="network-card-header compact">
            <div><h2>Novo teste</h2><p>Defina de onde e o que será verificado.</p></div>
          </div>
          <div class="network-test-form">
            <label class="network-field">
              <span>Computador</span>
              <select id="networkTestDevice">
                <option value="">Selecione um computador...</option>
                ${options}
              </select>
              <small>${devices.filter(deviceOnline).length} ${devices.filter(deviceOnline).length === 1 ? 'computador disponível' : 'computadores disponíveis'} para teste.</small>
            </label>
            <div class="network-field">
              <span>Tipo de teste</span>
              <div class="network-test-types">${typeCards}</div>
            </div>
            <div id="networkTestTargetWrap" class="network-field hidden">
              <span>Destino</span>
              <input id="networkTestTarget" type="text" placeholder="Ex.: 8.8.8.8, google.com ou 192.168.0.1:443">
            </div>
            <button class="btn primary network-run-btn" type="button" data-network-run>Executar teste</button>
          </div>
        </section>

        <section class="card network-result-card">
          <div class="network-card-header compact">
            <div><h2>Resultado</h2><p>Saída e status do diagnóstico selecionado.</p></div>
            <span class="network-result-status idle"><i></i>Aguardando</span>
          </div>
          <div id="networkTestResult" class="network-result-empty">
            <span>${icons.route}</span>
            <strong>Nenhum teste executado</strong>
            <p>Execute um diagnóstico para visualizar latência, rota, resolução e possíveis falhas.</p>
          </div>
        </section>
      </div>`;
  }

  function discovered(devices) {
    const known = devices.map((device) => `
      <tr>
        <td><div class="network-device-cell"><span class="network-device-icon">${icons.computer}</span><div><strong>${CT.esc(device.name)}</strong><small>Computador gerenciado</small></div></div></td>
        <td><span class="network-mono">${CT.esc(telemetry(device, 'ip_local'))}</span></td>
        <td><span class="network-muted">${CT.esc(telemetry(device, 'mac_address', '—'))}</span></td>
        <td>CoreControl Agent</td>
        <td>${statusBadge(device)}</td>
        <td class="table-actions-col"><button class="btn small" type="button" data-open-device="${device.id}">Detalhes</button></td>
      </tr>`).join('');

    return `
      <section class="network-devices-intro">
        <div class="network-devices-intro-main">
          <span class="network-devices-intro-icon" aria-hidden="true">${icons.device}</span>
          <div><span class="network-eyebrow">INVENTÁRIO DE REDE</span><h2>Dispositivos conhecidos</h2><p>Computadores gerenciados e, futuramente, equipamentos descobertos na rede local.</p></div>
        </div>
        <div class="network-status-meta"><span><b>${devices.length}</b> gerenciado${devices.length === 1 ? '' : 's'}</span></div>
      </section>

      <section class="card network-table-card">
        <div class="network-card-header">
          <div><h2>Computadores monitorados</h2><p>Equipamentos que possuem o CoreControl Agent instalado.</p></div>
          <span class="network-count-badge">${devices.length}</span>
        </div>
        <div class="table-wrap network-table-wrap">
          <table class="network-table">
            <thead><tr><th>Dispositivo</th><th>IP</th><th>MAC</th><th>Origem</th><th>Status</th><th></th></tr></thead>
            <tbody>${known || '<tr><td colspan="6"><div class="network-inline-empty"><span>'+icons.device+'</span><strong>Nenhum dispositivo conhecido</strong><small>Instale o Agent em um computador para começar.</small></div></td></tr>'}</tbody>
          </table>
        </div>
      </section>

      <section class="card network-discovery-card">
        <span class="network-discovery-icon" aria-hidden="true">${icons.search}</span>
        <div><strong>Descoberta de rede</strong><p>A descoberta ativa de impressoras, roteadores, switches, câmeras e outros equipamentos ainda não está habilitada nesta versão.</p></div>
        <span class="pill">Em breve</span>
      </section>`;
  }

  function filterOverviewRows(devices) {
    const tbody = CT.$('#networkDeviceRows');
    if (!tbody || !devices.length) return;
    const query = String(CT.$('#networkDeviceSearch')?.value || '').trim().toLowerCase();
    const status = CT.$('#networkStatusFilter')?.value || 'all';
    const filtered = devices.filter((device) => {
      const online = deviceOnline(device);
      if (status === 'online' && !online) return false;
      if (status === 'offline' && online) return false;
      if (!query) return true;
      return [device.name, device.hostname, device.company_name, telemetry(device, 'ip_local', ''), networkType(device)]
        .some((value) => String(value || '').toLowerCase().includes(query));
    });

    tbody.innerHTML = filtered.map((device) => `
      <tr>
        <td><div class="network-device-cell"><span class="network-device-icon">${icons.computer}</span><div><strong>${CT.esc(device.name)}</strong><small>${CT.esc(device.hostname || '—')}</small></div></div></td>
        <td>${CT.esc(device.company_name || '—')}</td>
        <td><span class="network-mono">${CT.esc(telemetry(device, 'ip_local'))}</span></td>
        <td>${CT.esc(networkType(device))}</td>
        <td><span class="network-muted">${CT.esc(telemetry(device, 'gateway'))}</span></td>
        <td><span class="network-muted">${CT.esc(telemetry(device, 'dns'))}</span></td>
        <td>${statusBadge(device)}</td>
        <td class="table-actions-col"><button class="btn small" type="button" data-open-device="${device.id}">Detalhes</button></td>
      </tr>`).join('') || '<tr><td colspan="8"><div class="network-inline-empty"><span>'+icons.search+'</span><strong>Nenhum resultado</strong><small>Ajuste a busca ou o filtro de status.</small></div></td></tr>';
  }

  CT.registerPage('network', async function renderNetwork() {
    const devices = await CT.api('/devices');
    await CT.mountPage('network');
    const page = CT.$('.page-network');
    const view = CT.$('#networkView');
    let activeTab = readTab();

    const render = (tab, persist = true) => {
      activeTab = NETWORK_TABS.has(tab) ? tab : 'overview';
      CT.$$('[data-network-tab]', page).forEach((button) => {
        const selected = button.dataset.networkTab === activeTab;
        button.classList.toggle('active', selected);
        button.setAttribute('aria-selected', selected ? 'true' : 'false');
      });
      view.innerHTML = activeTab === 'tests' ? tests(devices) : activeTab === 'devices' ? discovered(devices) : overview(devices);
      if (persist) writeTab(activeTab);
    };

    page.addEventListener('click', (event) => {
      const tab = event.target.closest('[data-network-tab]');
      if (tab) return render(tab.dataset.networkTab);

      if (event.target.closest('#networkTestBtn')) return render('tests');

      const device = event.target.closest('[data-open-device]');
      if (device) return CT.navigate('device', Number(device.dataset.openDevice));

      const testType = event.target.closest('.network-test-type');
      if (testType) {
        CT.$$('.network-test-type', page).forEach((item) => item.classList.toggle('selected', item === testType));
        const value = testType.querySelector('input')?.value;
        const targetWrap = CT.$('#networkTestTargetWrap');
        if (targetWrap) targetWrap.classList.toggle('hidden', !['ping', 'dns', 'traceroute', 'tcp'].includes(value));
        return;
      }

      if (event.target.closest('[data-network-run]')) {
        const deviceId = CT.$('#networkTestDevice')?.value;
        if (!deviceId) return CT.toast('Selecione um computador online para executar o teste.', true);
        const result = CT.$('#networkTestResult');
        const status = page.querySelector('.network-result-status');
        if (status) {
          status.className = 'network-result-status working';
          status.innerHTML = '<i></i>Preparando';
        }
        if (result) {
          result.className = 'network-result-empty';
          result.innerHTML = `<span class="network-loading-ring"></span><strong>Preparando diagnóstico</strong><p>O executor de testes de rede será conectado ao Agent em uma próxima etapa do módulo.</p>`;
        }
        return CT.toast('Interface pronta. O executor de diagnóstico ainda precisa ser conectado ao Agent.');
      }
    });

    page.addEventListener('input', (event) => {
      if (event.target.id === 'networkDeviceSearch') filterOverviewRows(devices);
    });
    page.addEventListener('change', (event) => {
      if (event.target.id === 'networkStatusFilter') filterOverviewRows(devices);
    });

    render(activeTab, false);
  });
})();

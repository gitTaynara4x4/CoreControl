(function () {
  'use strict';

  const CT = window.CoreTuner;

  CT.registerPage('companies', async function renderCompanies() {
    const companies = await CT.api('/companies');
    await CT.mountPage('companies');

    CT.$('#companiesGrid').innerHTML = companies.length
      ? companies.map(CT.companyCard).join('')
      : '<div class="empty">Nenhuma empresa cadastrada.</div>';
    CT.$('#newCompanyBtn').classList.toggle(
      'hidden',
      !CT.isGlobalAdmin(),
    );
    CT.bindCommonActions();
  });

  CT.registerPage('company', async function renderCompany() {
    const companyId = CT.state.selectedCompany;
    const [company, gateways] = await Promise.all([
      CT.api(`/companies/${companyId}`),
      CT.api(`/gateways?company_id=${encodeURIComponent(companyId)}`),
    ]);
    CT.state.selectedCompany = company.id;
    await CT.mountPage('company');

    CT.$('#pageTitle').textContent = company.name;
    const activeDevices = company.devices.filter((device) => device.active !== false);
    const disabledDevices = company.devices.filter((device) => device.active === false);
    CT.$('#companyStats').innerHTML = [
      CT.stat('Computadores', company.devices.length, 'Total vinculado'),
      CT.stat('Online', activeDevices.filter((device) => device.online).length, 'Comunicando', 'var(--green)'),
      CT.stat('Offline', activeDevices.filter((device) => !device.online).length, 'Sem comunicação', 'var(--red)'),
      CT.stat('Desativados', disabledDevices.length, 'Fora de operação'),
      CT.stat('Alertas', company.devices.reduce((sum, device) => sum + device.alerts_open, 0), 'Ativos', 'var(--amber)'),
    ].join('');

    const boxOnline = gateways.some((gateway) => gateway.online);
    CT.$('#companyGatewayArea').innerHTML = CT.isGlobalAdmin()
      ? (gateways.length
          ? gateways.map((gateway) => `
            <div class="callout" style="display:flex;align-items:center;gap:12px;justify-content:space-between;margin:0 0 8px">
              <div>
                <strong>${CT.esc(gateway.name || 'CoreControl Box')}</strong>
                <div style="font-size:12px;opacity:.72;margin-top:4px">${gateway.online ? 'Online e pronta para controle de energia' : `Offline · último contato ${CT.fmtDate(gateway.last_seen)}`} · ${CT.esc((gateway.network_cidrs || []).join(', ') || 'rede aguardando detecção')}</div>
              </div>
              <span class="status ${gateway.online ? 'online' : 'offline'}"><i class="dot ${gateway.online ? 'online' : 'offline'}"></i>${gateway.online ? 'Online' : 'Offline'}</span>
            </div>`).join('')
          : '<div class="empty"><strong>CoreControl Box ainda não instalada.</strong><br>Instale uma Box neste local para padronizar o Ligar/Desligar.</div>')
      : `<div class="callout"><strong>${boxOnline ? 'Controle de energia disponível' : 'Controle de energia aguardando ativação'}</strong><br>${boxOnline ? 'O CoreControl está pronto para ligar e desligar este computador.' : 'A configuração é feita pelo suporte. Nenhuma alteração no roteador é necessária.'}</div>`;

    CT.$('#companyDevicesArea').innerHTML = company.devices.length
      ? CT.deviceTable(company.devices)
      : '<div class="empty">Nenhum computador instalado nesta empresa.</div>';

    CT.$('#backCompanies').onclick = () => CT.navigate('companies');
    CT.$('#enrollBtn').onclick = () => CT.createEnrollmentToken(company.id, company.name);
    const boxBtn = CT.$('#gatewayBtn');
    if (CT.isGlobalAdmin()) {
      boxBtn.classList.remove('hidden');
      boxBtn.onclick = () => CT.openGatewayEnrollmentOptions(company.id, company.name);
    }
    const editCompanyBtn = CT.$('#editCompanyBtn');
    if (CT.isGlobalAdmin()) {
      editCompanyBtn.classList.remove('hidden');
      editCompanyBtn.onclick = () => CT.openCompanyEditModal(company);
    }

    const deleteCompanyBtn = CT.$('#deleteCompanyBtn');
    if (CT.canDestroyCompanies()) {
      deleteCompanyBtn.classList.remove('hidden');
      deleteCompanyBtn.onclick = () => CT.openCompanyDeleteModal(company);
    }
    CT.bindDeviceRows();
  });
})();

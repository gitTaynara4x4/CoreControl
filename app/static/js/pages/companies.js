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
    const economyReady = activeDevices.some((device) => Boolean(device.power?.economy_available || device.power?.economy_mode_active));
    CT.$('#companyGatewayArea').innerHTML = `
      <div class="callout">
        <strong>${economyReady ? 'Modo econômico disponível' : 'Aguardando CoreControl Agent'}</strong><br>
        ${economyReady
          ? 'Os computadores podem entrar e sair do Modo econômico apenas pelo software. O Windows continua ligado e o Agent permanece conectado.'
          : 'Assim que o Agent estiver online, o controle de energia por software fica disponível automaticamente.'}
      </div>
      ${CT.isGlobalAdmin() && gateways.length ? `<div class="callout" style="margin-top:8px"><strong>Módulo físico opcional</strong><br>${boxOnline ? 'Há uma CoreControl Box online neste local.' : 'Há uma CoreControl Box cadastrada, mas offline.'} Ela não é necessária para o Modo econômico.</div>` : ''}`;

    CT.$('#companyDevicesArea').innerHTML = company.devices.length
      ? CT.deviceTable(company.devices)
      : '<div class="empty">Nenhum computador instalado nesta empresa.</div>';

    CT.$('#backCompanies').onclick = () => CT.navigate('companies');
    CT.$('#enrollBtn').onclick = () => CT.createEnrollmentToken(company.id, company.name);
    const boxBtn = CT.$('#gatewayBtn');
    if (boxBtn) boxBtn.classList.add('hidden');
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

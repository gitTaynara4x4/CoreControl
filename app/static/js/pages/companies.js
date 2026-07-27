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
      CT.state.user.role !== 'platform_admin',
    );
    CT.bindCommonActions();
  });

  CT.registerPage('company', async function renderCompany() {
    const company = await CT.api(`/companies/${CT.state.selectedCompany}`);
    CT.state.selectedCompany = company.id;
    await CT.mountPage('company');

    CT.$('#pageTitle').textContent = company.name;
    CT.$('#companyStats').innerHTML = [
      CT.stat('Computadores', company.devices.length, 'Total vinculado'),
      CT.stat('Online', company.devices.filter((device) => device.online).length, 'Comunicando', 'var(--green)'),
      CT.stat('Offline', company.devices.filter((device) => !device.online).length, 'Sem comunicação', 'var(--red)'),
      CT.stat('Alertas', company.devices.reduce((sum, device) => sum + device.alerts_open, 0), 'Ativos', 'var(--amber)'),
    ].join('');

    CT.$('#companyDevicesArea').innerHTML = company.devices.length
      ? CT.deviceTable(company.devices)
      : '<div class="empty">Nenhum computador instalado nesta empresa.</div>';

    CT.$('#backCompanies').onclick = () => CT.navigate('companies');
    CT.$('#enrollBtn').onclick = () => CT.createEnrollmentToken(company.id, company.name);
    CT.bindDeviceRows();
  });
})();

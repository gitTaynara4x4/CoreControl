(function () {
  'use strict';

  const CT = window.CoreTuner;

  CT.registerPage('overview', async function renderOverview() {
    const [summary, companies, devices, alerts] = await Promise.all([
      CT.api('/dashboard/summary'),
      CT.api('/companies'),
      CT.api('/devices'),
      CT.api('/alerts?status_filter=active'),
    ]);

    await CT.mountPage('overview');
    CT.setAlertBadge(summary.alerts_open);

    const attention = devices
      .filter((device) => !device.online || device.health_score < 70)
      .slice(0, 7);

    CT.$('#overviewStats').innerHTML = [
      CT.stat('Empresas', summary.companies, 'Clientes cadastrados'),
      CT.stat('Computadores', summary.devices, 'Agentes vinculados'),
      CT.stat('Online', summary.online, 'Comunicando agora', 'var(--green)'),
      CT.stat('Offline', summary.offline, 'Sem comunicação', 'var(--red)'),
      CT.stat('Alertas ativos', summary.alerts_open, 'Precisam de avaliação', 'var(--amber)'),
    ].join('');

    CT.$('#overviewAttention').innerHTML = attention.length
      ? CT.deviceTable(attention)
      : '<div class="empty">Nenhum computador exige atenção agora.</div>';

    CT.$('#overviewAlerts').innerHTML = alerts.length
      ? alerts.slice(0, 6).map(CT.alertRow).join('')
      : '<div class="empty">Nenhum alerta ativo.</div>';

    CT.$('#overviewCompanies').innerHTML = companies.length
      ? companies.map(CT.companyCard).join('')
      : '<div class="empty">Cadastre a primeira empresa para começar.</div>';

    CT.$('#newCompanyBtn').classList.toggle(
      'hidden',
      CT.state.user.role !== 'platform_admin',
    );
    CT.bindCommonActions();
  });
})();

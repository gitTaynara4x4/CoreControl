(function () {
  'use strict';

  const CT = window.CoreTuner;

  CT.pageMeta = {
    overview: ['Visão geral', 'Acompanhe as empresas e computadores em tempo quase real.'],
    companies: ['Empresas', 'Organize cada cliente e os computadores vinculados.'],
    devices: ['Computadores', 'Veja saúde, uso e alertas de todas as máquinas autorizadas.'],
    alerts: ['Alertas', 'Priorize o que exige atenção técnica.'],
    remote: ['Acesso remoto', 'Acesse computadores autorizados com registro da solicitação.'],
    users: ['Usuários e permissões', 'Controle quem pode visualizar ou administrar cada empresa.'],
    company: ['Detalhes da empresa', 'Computadores, situação e instalação de novos agentes.'],
    device: ['Detalhes do computador', 'Diagnóstico técnico e histórico de telemetria.'],
  };

  CT.setupUser = function setupUser() {
    CT.$('#userName').textContent = CT.state.user.name;
    CT.$('#userRole').textContent = CT.roleName(CT.state.user.role);
    CT.$('#userInitial').textContent = (CT.state.user.name || 'A').slice(0, 1).toUpperCase();

    const usersButton = CT.$('[data-page="users"]');
    usersButton.classList.toggle(
      'hidden',
      !['platform_admin', 'company_admin'].includes(CT.state.user.role),
    );
  };

  CT.startRefresh = function startRefresh() {
    clearInterval(CT.state.refreshTimer);
    CT.state.refreshTimer = setInterval(() => {
      if (['overview', 'devices', 'alerts', 'remote'].includes(CT.state.page)) {
        CT.renderCurrent(false);
      }
    }, 15000);
  };

  CT.navigate = async function navigate(page, context = null) {
    CT.state.page = page;
    if (page === 'company') CT.state.selectedCompany = context;
    if (page === 'device') CT.state.selectedDevice = context;

    CT.$$('.nav-item[data-page]').forEach((button) => {
      button.classList.toggle('active', button.dataset.page === page);
    });

    const [title, subtitle] = CT.pageMeta[page] || ['CoreTuner Central', ''];
    CT.$('#pageTitle').textContent = title;
    CT.$('#pageSubtitle').textContent = subtitle;
    await CT.renderCurrent(true);
  };

  CT.renderCurrent = async function renderCurrent(showBusy = true) {
    const content = CT.$('#content');
    if (showBusy) {
      content.innerHTML = '<div class="card empty">Atualizando informações...</div>';
    }

    try {
      const renderer = CT.pageRenderers[CT.state.page];
      if (typeof renderer !== 'function') {
        throw new Error(`Tela não registrada: ${CT.state.page}`);
      }
      await renderer();
      CT.$('#lastRefresh').textContent = `Atualizado ${new Date().toLocaleTimeString('pt-BR', {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      })}`;
    } catch (error) {
      content.innerHTML = `<div class="card empty">${CT.esc(error.message)}</div>`;
      if (showBusy) CT.toast(error.message, true);
    }
  };

  CT.bindBaseEvents = function bindBaseEvents() {
    CT.$('#logoutBtn').addEventListener('click', async () => {
      try {
        await CT.api('/auth/logout', { method: 'POST' });
      } catch (_) {
        // A tela deve sair mesmo quando a sessão já expirou.
      }
      CT.showLogin();
    });

    CT.$('#refreshBtn').addEventListener('click', () => CT.renderCurrent(true));
    CT.$('#mainNav').addEventListener('click', (event) => {
      const button = event.target.closest('[data-page]');
      if (button) CT.navigate(button.dataset.page);
    });
    CT.$('#modalBackdrop').addEventListener('click', (event) => {
      if (event.target.id === 'modalBackdrop') CT.closeModal();
    });
  };
})();

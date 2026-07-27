(function () {
  'use strict';

  const CT = window.CoreTuner;

  CT.boot = async function boot() {
    try {
      await Promise.all([
        CT.mountLogin(),
        CT.mountGlobalComponents(),
      ]);
      CT.bindAuthEvents();
      CT.bindBaseEvents();

      try {
        CT.state.user = await CT.api('/auth/me');
        CT.showApp();
        CT.setupUser();
        await CT.navigate('overview');
        CT.startRefresh();
      } catch (_) {
        CT.showLogin();
      }
    } catch (error) {
      document.body.innerHTML = `<div class="card empty">${CT.esc(error.message)}</div>`;
      console.error('[CoreTuner] Falha ao iniciar o painel:', error);
    }
  };

  document.addEventListener('DOMContentLoaded', CT.boot);
})();

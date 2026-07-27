(function () {
  'use strict';

  const CT = window.CoreTuner;

  CT.registerPage('users', async function renderUsers() {
    const users = await CT.api('/users');
    const companies = CT.state.user.role === 'platform_admin'
      ? await CT.api('/companies')
      : [];

    await CT.mountPage('users');
    CT.$('#usersArea').innerHTML = `<div class="table-wrap"><table><thead><tr><th>Nome</th><th>E-mail</th><th>Perfil</th><th>Empresa</th><th>Status</th></tr></thead><tbody>${users.map((user) => `<tr><td><strong>${CT.esc(user.name)}</strong></td><td>${CT.esc(user.email)}</td><td>${CT.esc(CT.roleName(user.role))}</td><td>${CT.esc(companies.find((company) => company.id === user.company_id)?.name || 'Todas as empresas')}</td><td><span class="pill ${user.active ? 'resolved' : 'critical'}">${user.active ? 'Ativo' : 'Bloqueado'}</span></td></tr>`).join('')}</tbody></table></div>`;
    CT.$('#newUserBtn').onclick = () => CT.openUserModal(companies);
  });
})();

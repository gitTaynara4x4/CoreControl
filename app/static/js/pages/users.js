(function () {
  'use strict';

  const CT = window.CoreTuner;

  CT.registerPage('users', async function renderUsers() {
    const users = await CT.api('/users');
    const companies = CT.isGlobalAdmin()
      ? await CT.api('/companies')
      : [];

    const companyName = (companyId) => {
      if (companyId == null) return 'Todas as empresas';
      return companies.find((company) => company.id === companyId)?.name
        || CT.state.user.company?.name
        || `Empresa #${companyId}`;
    };

    await CT.mountPage('users');
    CT.$('#usersArea').innerHTML = `<div class="table-wrap"><table><thead><tr><th>Nome</th><th>E-mail</th><th>Perfil</th><th>Empresa</th><th>Status</th><th class="table-actions-col">Ações</th></tr></thead><tbody>${users.map((item) => {
      const protectedGlobalAdmin = item.role === 'global_admin' && CT.state.user.role !== 'global_admin';
      return `<tr><td><strong>${CT.esc(item.name)}</strong></td><td>${CT.esc(item.email)}</td><td>${CT.esc(CT.roleName(item.role))}</td><td>${CT.esc(companyName(item.company_id))}</td><td><span class="pill ${item.active ? 'resolved' : 'critical'}">${item.active ? 'Ativo' : 'Bloqueado'}</span></td><td class="table-actions-col">${protectedGlobalAdmin ? '<span class="pill">Protegido</span>' : `<button class="btn table-action-btn" type="button" data-edit-user="${item.id}">Editar</button>`}</td></tr>`;
    }).join('')}</tbody></table></div>`;

    CT.$('#newUserBtn').onclick = () => CT.openUserModal(companies);
    CT.$$('[data-edit-user]').forEach((button) => {
      button.onclick = () => {
        const target = users.find((item) => item.id === Number(button.dataset.editUser));
        if (target) CT.openUserEditModal(target, companies);
      };
    });
  });
})();

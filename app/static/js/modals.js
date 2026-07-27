(function () {
  'use strict';

  const CT = window.CoreTuner;

  CT.openModal = function openModal(html) {
    CT.$('#modal').innerHTML = html;
    CT.$('#modalBackdrop').classList.remove('hidden');
  };

  CT.openModalTemplate = async function openModalTemplate(templateName) {
    CT.openModal(await CT.loadTemplate(`components/modals/${templateName}.html`));
  };

  CT.closeModal = function closeModal() {
    CT.$('#modalBackdrop').classList.add('hidden');
    CT.$('#modal').innerHTML = '';
  };

  CT.openCompanyModal = async function openCompanyModal() {
    await CT.openModalTemplate('company');
    CT.$('#cancelModal').onclick = CT.closeModal;
    CT.$('#companyForm').onsubmit = async (event) => {
      event.preventDefault();
      try {
        const company = await CT.api('/companies', {
          method: 'POST',
          body: JSON.stringify({ name: CT.$('#companyName').value }),
        });
        CT.closeModal();
        CT.toast('Empresa cadastrada.');
        CT.navigate('company', company.id);
      } catch (error) {
        CT.toast(error.message, true);
      }
    };
  };

  CT.createEnrollmentToken = async function createEnrollmentToken(companyId, companyName) {
    try {
      const data = await CT.api(`/companies/${companyId}/enrollment-token`, { method: 'POST' });
      await CT.openModalTemplate('enrollment-token');
      CT.$('#tokenCompanyName').textContent = companyName;
      CT.$('#tokenText').textContent = data.token;
      CT.$('#tokenExpiration').textContent = `Validade: ${CT.fmtDate(data.expires_at)}. O token deixa de funcionar assim que um computador for vinculado.`;
      CT.$('#closeToken').onclick = CT.closeModal;
      CT.$('#copyToken').onclick = async () => {
        await navigator.clipboard.writeText(data.token);
        CT.toast('Token copiado.');
      };
    } catch (error) {
      CT.toast(error.message, true);
    }
  };

  CT.openUserModal = async function openUserModal(companies) {
    await CT.openModalTemplate('user');

    const roleSelect = CT.$('#newUserRole');
    if (CT.state.user.role === 'platform_admin') {
      roleSelect.insertAdjacentHTML(
        'beforeend',
        '<option value="platform_admin">Administrador da plataforma</option>',
      );
      const companyField = CT.$('#userCompanyField');
      const companySelect = CT.$('#userCompany');
      companyField.classList.remove('hidden');
      companySelect.innerHTML = `<option value="">Todas (somente administrador da plataforma)</option>${companies.map((company) => `<option value="${company.id}">${CT.esc(company.name)}</option>`).join('')}`;
    }

    CT.$('#cancelModal').onclick = CT.closeModal;
    CT.$('#userForm').onsubmit = async (event) => {
      event.preventDefault();
      const companyValue = CT.$('#userCompany')?.value;
      try {
        await CT.api('/users', {
          method: 'POST',
          body: JSON.stringify({
            name: CT.$('#newUserName').value,
            email: CT.$('#newUserEmail').value,
            password: CT.$('#newUserPassword').value,
            role: CT.$('#newUserRole').value,
            company_id: companyValue ? Number(companyValue) : null,
          }),
        });
        CT.closeModal();
        CT.toast('Usuário cadastrado.');
        CT.renderCurrent(true);
      } catch (error) {
        CT.toast(error.message, true);
      }
    };
  };
})();

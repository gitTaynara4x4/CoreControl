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
      const installationUrl = new URL(data.installation_url, window.location.origin).href;
      CT.$('#tokenCompanyName').textContent = companyName;
      CT.$('#installationLink').textContent = installationUrl;
      CT.$('#tokenExpiration').textContent = `Válido até ${CT.fmtDate(data.expires_at)}. O funcionário só precisa abrir o link e executar o instalador.`;
      CT.$('#closeToken').onclick = CT.closeModal;
      CT.$('#downloadHere').onclick = () => window.location.assign(installationUrl);
      CT.$('#copyInstallLink').onclick = async () => {
        await navigator.clipboard.writeText(installationUrl);
        CT.toast('Link de instalação copiado.');
      };
    } catch (error) {
      CT.toast(error.message, true);
    }
  };

  CT.chooseEnrollmentCompany = async function chooseEnrollmentCompany(companies) {
    const active = (companies || []).filter((company) => company.active !== false);
    if (!active.length) {
      CT.toast('Nenhuma empresa ativa disponível.', true);
      return;
    }
    if (active.length === 1) {
      await CT.createEnrollmentToken(active[0].id, active[0].name);
      return;
    }
    CT.openModal(`
      <h2>Adicionar computador</h2>
      <p>Escolha a empresa que receberá este computador.</p>
      <form id="enrollmentCompanyForm" class="stack">
        <label>Empresa
          <select id="enrollmentCompanySelect" required>
            <option value="">Selecione uma empresa</option>
            ${active.map((company) => `<option value="${company.id}">${CT.esc(company.name)}</option>`).join('')}
          </select>
        </label>
        <div class="modal-actions">
          <button class="btn" type="button" id="cancelEnrollmentCompany">Cancelar</button>
          <button class="btn primary" type="submit">Gerar link de instalação</button>
        </div>
      </form>`);
    CT.$('#cancelEnrollmentCompany').onclick = CT.closeModal;
    CT.$('#enrollmentCompanyForm').onsubmit = async (event) => {
      event.preventDefault();
      const id = Number(CT.$('#enrollmentCompanySelect').value);
      const company = active.find((item) => item.id === id);
      if (!company) return;
      CT.closeModal();
      await CT.createEnrollmentToken(company.id, company.name);
    };
  };

  CT.openUserModal = async function openUserModal(companies) {
    await CT.openModalTemplate('user');

    const roleSelect = CT.$('#newUserRole');
    if (CT.isGlobalAdmin()) {
      roleSelect.insertAdjacentHTML(
        'beforeend',
        '<option value="platform_admin">Administrador da plataforma</option>',
      );
      const companyField = CT.$('#userCompanyField');
      const companySelect = CT.$('#userCompany');
      companyField.classList.remove('hidden');
      companySelect.innerHTML = `<option value="">Selecione uma empresa</option>${companies.map((company) => `<option value="${company.id}">${CT.esc(company.name)}${company.active ? '' : ' — desativada'}</option>`).join('')}`;
      const syncNewUserCompany = () => {
        const globalRole = roleSelect.value === 'platform_admin';
        companyField.classList.toggle('hidden', globalRole);
        if (globalRole) companySelect.value = '';
      };
      roleSelect.onchange = syncNewUserCompany;
      syncNewUserCompany();
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

  CT.openCompanyEditModal = async function openCompanyEditModal(company) {
    await CT.openModalTemplate('company-edit');
    CT.$('#editCompanyName').value = company.name || '';
    CT.$('#editCompanyActive').value = String(company.active !== false);
    CT.$('#cancelModal').onclick = CT.closeModal;
    CT.$('#companyEditForm').onsubmit = async (event) => {
      event.preventDefault();
      try {
        await CT.api(`/companies/${company.id}`, {
          method: 'PATCH',
          body: JSON.stringify({
            name: CT.$('#editCompanyName').value,
            active: CT.$('#editCompanyActive').value === 'true',
          }),
        });
        CT.closeModal();
        CT.toast('Empresa atualizada.');
        await CT.renderCurrent(true);
      } catch (error) {
        CT.toast(error.message, true);
      }
    };
  };

  CT.openCompanyDeleteModal = async function openCompanyDeleteModal(company) {
    if (!CT.canDestroyCompanies()) {
      CT.toast('Apenas o Administrador Global pode excluir empresas definitivamente.', true);
      return;
    }

    await CT.openModalTemplate('company-delete');
    const phrase = `EXCLUIR ${company.name}`;
    const confirmationInput = CT.$('#deleteCompanyConfirmation');
    const confirmButton = CT.$('#confirmDeleteCompanyBtn');

    CT.$('#deleteCompanyName').textContent = company.name;
    CT.$('#deleteCompanyImpact').textContent = `${company.devices?.length || company.devices_total || 0} computador(es) e ${company.users_total || 0} usuário(s) vinculados serão removidos.`;
    CT.$('#deleteCompanyPhrase').textContent = phrase;
    CT.$('#cancelModal').onclick = CT.closeModal;

    const normalize = (value) => String(value || '').trim().replace(/\s+/g, ' ').toLocaleLowerCase('pt-BR');
    confirmationInput.oninput = () => {
      confirmButton.disabled = normalize(confirmationInput.value) !== normalize(phrase);
    };

    CT.$('#companyDeleteForm').onsubmit = async (event) => {
      event.preventDefault();
      if (confirmButton.disabled) return;
      try {
        const result = await CT.api(`/companies/${company.id}`, {
          method: 'DELETE',
          body: JSON.stringify({ confirmation: confirmationInput.value }),
        });
        CT.closeModal();
        CT.state.selectedCompany = null;
        CT.toast(result.remote_cleanup?.warning || 'Empresa excluída definitivamente.');
        await CT.navigate('companies');
      } catch (error) {
        CT.toast(error.message, true);
      }
    };
  };

  CT.openDeviceEditModal = async function openDeviceEditModal(device) {
    const companies = CT.isGlobalAdmin() ? await CT.api('/companies') : [];
    await CT.openModalTemplate('device-edit');

    const companyField = CT.$('#editDeviceCompanyField');
    const companySelect = CT.$('#editDeviceCompany');
    if (CT.isGlobalAdmin()) {
      companySelect.innerHTML = companies.map((company) => `<option value="${company.id}">${CT.esc(company.name)}${company.active ? '' : ' — desativada'}</option>`).join('');
      companySelect.value = String(device.company_id);
    } else {
      companyField.classList.add('hidden');
    }

    CT.$('#editDeviceName').value = device.name || '';
    CT.$('#editDeviceHostname').value = device.hostname || '';
    CT.$('#editDeviceSector').value = device.sector || '';
    CT.$('#editDeviceLocation').value = device.location || '';
    CT.$('#editDeviceManufacturer').value = device.manufacturer || '';
    CT.$('#editDeviceModel').value = device.model || '';
    CT.$('#editDeviceSerial').value = device.serial_number || '';
    CT.$('#editDeviceProfile').value = device.profile || '';
    CT.$('#editDeviceActive').value = String(device.active !== false);
    CT.$('#cancelModal').onclick = CT.closeModal;

    CT.$('#deviceEditForm').onsubmit = async (event) => {
      event.preventDefault();
      const payload = {
        name: CT.$('#editDeviceName').value,
        hostname: CT.$('#editDeviceHostname').value,
        sector: CT.$('#editDeviceSector').value,
        location: CT.$('#editDeviceLocation').value,
        manufacturer: CT.$('#editDeviceManufacturer').value,
        model: CT.$('#editDeviceModel').value,
        serial_number: CT.$('#editDeviceSerial').value,
        profile: CT.$('#editDeviceProfile').value,
        active: CT.$('#editDeviceActive').value === 'true',
      };
      if (CT.isGlobalAdmin()) {
        payload.company_id = Number(companySelect.value);
      }
      try {
        await CT.api(`/devices/${device.id}`, {
          method: 'PATCH',
          body: JSON.stringify(payload),
        });
        CT.closeModal();
        CT.toast('Computador atualizado.');
        await CT.renderCurrent(true);
      } catch (error) {
        CT.toast(error.message, true);
      }
    };
  };

  CT.openUserEditModal = async function openUserEditModal(targetUser, companies) {
    await CT.openModalTemplate('user-edit');
    const roleSelect = CT.$('#editUserRole');
    const companyField = CT.$('#editUserCompanyField');
    const companySelect = CT.$('#editUserCompany');

    if (!CT.isGlobalAdmin()) {
      roleSelect.querySelector('option[value="platform_admin"]')?.remove();
      roleSelect.querySelector('option[value="global_admin"]')?.remove();
      companyField.classList.add('hidden');
    } else {
      if (targetUser.role !== 'global_admin') {
        roleSelect.querySelector('option[value="global_admin"]')?.remove();
      }
      companySelect.innerHTML = `<option value="">Sem empresa</option>${companies.map((company) => `<option value="${company.id}">${CT.esc(company.name)}${company.active ? '' : ' — desativada'}</option>`).join('')}`;
      companySelect.value = targetUser.company_id == null ? '' : String(targetUser.company_id);
    }

    CT.$('#editUserName').value = targetUser.name || '';
    CT.$('#editUserEmail').value = targetUser.email || '';
    roleSelect.value = targetUser.role;
    CT.$('#editUserActive').value = String(targetUser.active !== false);

    const syncCompanyField = () => {
      const globalAdmin = ['global_admin', 'platform_admin'].includes(roleSelect.value);
      if (CT.isGlobalAdmin()) {
        companyField.classList.toggle('hidden', globalAdmin);
        if (globalAdmin) companySelect.value = '';
      }
    };
    roleSelect.onchange = syncCompanyField;
    syncCompanyField();

    CT.$('#cancelModal').onclick = CT.closeModal;
    CT.$('#userEditForm').onsubmit = async (event) => {
      event.preventDefault();
      const password = CT.$('#editUserPassword').value;
      const payload = {
        name: CT.$('#editUserName').value,
        email: CT.$('#editUserEmail').value,
        role: roleSelect.value,
        active: CT.$('#editUserActive').value === 'true',
      };
      if (password) payload.password = password;
      if (CT.isGlobalAdmin()) {
        payload.company_id = ['global_admin', 'platform_admin'].includes(roleSelect.value)
          ? null
          : (companySelect.value ? Number(companySelect.value) : null);
      }
      try {
        await CT.api(`/users/${targetUser.id}`, {
          method: 'PATCH',
          body: JSON.stringify(payload),
        });
        CT.closeModal();
        CT.toast('Usuário atualizado.');
        await CT.renderCurrent(true);
      } catch (error) {
        CT.toast(error.message, true);
      }
    };
  };

})();

(function () {
  'use strict';

  const CT = window.CoreTuner;

  CT.setCentralAuthMode = function setCentralAuthMode(mode) {
    const login = mode === 'login';
    CT.$('#centralLoginTab').classList.toggle('active', login);
    CT.$('#centralRegisterTab').classList.toggle('active', !login);
    CT.$('#loginForm').classList.toggle('hidden', !login);
    CT.$('#registerCompanyForm').classList.toggle('hidden', login);
  };

  CT.showLogin = function showLogin() {
    clearInterval(CT.state.refreshTimer);
    CT.state.user = null;
    CT.$('#appView').classList.add('hidden');
    CT.$('#loginMount').classList.remove('hidden');
    CT.$('#loginView').classList.remove('hidden');
    CT.setCentralAuthMode('login');
  };

  CT.showApp = function showApp() {
    CT.$('#loginView').classList.add('hidden');
    CT.$('#loginMount').classList.add('hidden');
    CT.$('#appView').classList.remove('hidden');
  };

  CT.mountLogin = async function mountLogin() {
    await CT.mountTemplate('#loginMount', 'pages/login.html');
  };

  CT.bindAuthEvents = function bindAuthEvents() {
    CT.$('#centralLoginTab').addEventListener('click', () => CT.setCentralAuthMode('login'));
    CT.$('#centralRegisterTab').addEventListener('click', () => CT.setCentralAuthMode('register'));

    CT.$('#centralForgotPassword').addEventListener('click', () => {
      const email = CT.$('#loginEmail').value.trim();
      const target = new URL('/', window.location.origin);
      target.searchParams.set('forgot', '1');
      if (email) target.searchParams.set('email', email);
      window.location.assign(target.toString());
    });

    CT.$('#loginForm').addEventListener('submit', async (event) => {
      event.preventDefault();
      const button = event.submitter;
      button.disabled = true;
      button.textContent = 'Entrando...';

      try {
        await CT.api('/auth/login', {
          method: 'POST',
          body: JSON.stringify({
            email: CT.$('#loginEmail').value,
            password: CT.$('#loginPassword').value,
          }),
        });
        CT.state.user = await CT.api('/auth/me');
        CT.showApp();
        CT.setupUser();
        await CT.navigate('overview');
        CT.startRefresh();
      } catch (error) {
        CT.toast(error.message, true);
      } finally {
        button.disabled = false;
        button.textContent = 'Entrar';
      }
    });

    CT.$('#registerCompanyForm').addEventListener('submit', async (event) => {
      event.preventDefault();
      const button = event.submitter;
      button.disabled = true;
      button.textContent = 'Criando empresa...';

      try {
        await CT.api('/auth/register-company', {
          method: 'POST',
          body: JSON.stringify({
            company_name: CT.$('#centralCompanyName').value,
            responsible_name: CT.$('#centralResponsibleName').value,
            email: CT.$('#centralRegisterEmail').value,
            password: CT.$('#centralRegisterPassword').value,
            password_confirmation: CT.$('#centralRegisterPasswordConfirmation').value,
          }),
        });
        CT.state.user = await CT.api('/auth/me');
        CT.showApp();
        CT.setupUser();
        await CT.navigate('overview');
        CT.startRefresh();
        CT.toast('Empresa criada com sucesso.');
      } catch (error) {
        CT.toast(error.message, true);
      } finally {
        button.disabled = false;
        button.textContent = 'Criar empresa';
      }
    });
  };
})();

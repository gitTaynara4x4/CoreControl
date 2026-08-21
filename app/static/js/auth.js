(function () {
  'use strict';

  const CT = window.CoreTuner;

  function setAuthCopy(login) {
    const title = CT.$('#centralAuthTitle');
    const subtitle = CT.$('#centralAuthSubtitle');
    if (title) title.textContent = login ? 'Acessar minha conta' : 'Criar minha conta';
    if (subtitle) {
      subtitle.textContent = login
        ? 'Entre com seu código, e-mail ou CNPJ.'
        : 'Cadastre sua empresa para fazer o primeiro acesso.';
    }
  }

  CT.setCentralAuthMode = function setCentralAuthMode(mode) {
    const login = mode === 'login';
    const loginTab = CT.$('#centralLoginTab');
    const registerTab = CT.$('#centralRegisterTab');

    loginTab.classList.toggle('active', login);
    registerTab.classList.toggle('active', !login);
    loginTab.setAttribute('aria-selected', String(login));
    registerTab.setAttribute('aria-selected', String(!login));
    CT.$('#loginForm').classList.toggle('hidden', !login);
    CT.$('#registerCompanyForm').classList.toggle('hidden', login);
    setAuthCopy(login);
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

    const createPasswordShortcut = CT.$('#centralCreatePasswordShortcut');
    if (createPasswordShortcut) {
      createPasswordShortcut.addEventListener('click', () => CT.setCentralAuthMode('register'));
    }

    const backToLogin = CT.$('#centralBackToLogin');
    if (backToLogin) {
      backToLogin.addEventListener('click', () => CT.setCentralAuthMode('login'));
    }

    const passwordToggle = CT.$('#toggleLoginPassword');
    if (passwordToggle) {
      passwordToggle.addEventListener('click', () => {
        const input = CT.$('#loginPassword');
        const revealing = input.type === 'password';
        input.type = revealing ? 'text' : 'password';
        passwordToggle.setAttribute('aria-label', revealing ? 'Ocultar senha' : 'Mostrar senha');
        passwordToggle.setAttribute('title', revealing ? 'Ocultar senha' : 'Mostrar senha');
        CT.$('.ct-eye-open', passwordToggle)?.classList.toggle('hidden', revealing);
        CT.$('.ct-eye-closed', passwordToggle)?.classList.toggle('hidden', !revealing);
      });
    }

    const supportLink = CT.$('#centralSupportLink');
    if (supportLink) {
      supportLink.addEventListener('click', () => {
        CT.toast('Entre em contato com o suporte responsável pelo seu CoreTuner.');
      });
    }

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
      const label = button?.querySelector('span');
      button.disabled = true;
      if (label) label.textContent = 'Entrando...';

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
        if (label) label.textContent = 'Entrar no CoreTuner';
      }
    });

    CT.$('#registerCompanyForm').addEventListener('submit', async (event) => {
      event.preventDefault();
      const button = event.submitter;
      const label = button?.querySelector('span');
      button.disabled = true;
      if (label) label.textContent = 'Criando empresa...';

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
        if (label) label.textContent = 'Criar empresa e entrar';
      }
    });
  };
})();

(() => {
  const authModal = document.getElementById('authModal');
  const loginForm = document.getElementById('siteLoginForm');
  const registerForm = document.getElementById('siteRegisterForm');
  const forgotForm = document.getElementById('siteForgotForm');
  const resetForm = document.getElementById('siteResetForm');
  const loginTab = document.getElementById('loginTab');
  const registerTab = document.getElementById('registerTab');
  const authTabs = document.querySelector('.auth-tabs');
  const forgotPasswordButton = document.getElementById('forgotPasswordButton');
  const backToLoginButton = document.getElementById('backToLoginButton');
  const cancelResetButton = document.getElementById('cancelResetButton');
  const closeAuthButton = document.getElementById('closeAuthModal');
  const accountButton = document.getElementById('accountButton');
  const mobileAccountButton = document.getElementById('mobileAccountButton');

  const toast = document.getElementById('toast');
  const menuButton = document.getElementById('menuButton');
  const mobileNav = document.getElementById('mobileNav');

  let currentUser = null;
  let pendingInstall = false;
  let resetToken = '';

  function showToast(message, isError = false) {
    toast.textContent = message;
    toast.className = `toast show${isError ? ' error' : ''}`;
    window.clearTimeout(showToast.timer);
    showToast.timer = window.setTimeout(() => { toast.className = 'toast'; }, 4500);
  }

  async function api(path, options = {}) {
    const response = await fetch(path, {
      credentials: 'same-origin',
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...(options.headers || {})
      }
    });
    const data = await response.json().catch(() => ({}));
    if (!response.ok) {
      const detail = Array.isArray(data.detail)
        ? data.detail.map(item => item.msg).join(' ')
        : data.detail;
      throw new Error(detail || 'Não foi possível concluir a operação.');
    }
    return data;
  }

  function updateAccountButtons() {
    const text = currentUser
      ? (currentUser.company?.name || currentUser.name || 'Minha empresa')
      : 'Entrar';
    if (accountButton) accountButton.textContent = text;
    if (mobileAccountButton) mobileAccountButton.textContent = text;
  }

  async function loadSession() {
    try {
      currentUser = await api('/api/auth/me', { method: 'GET', headers: {} });
    } catch (_) {
      currentUser = null;
    }
    updateAccountButtons();
  }

  function setAuthMode(mode) {
    const login = mode === 'login';
    const register = mode === 'register';
    const forgot = mode === 'forgot';
    const reset = mode === 'reset';
    authTabs.hidden = forgot || reset;
    loginTab.classList.toggle('active', login);
    registerTab.classList.toggle('active', register);
    loginForm.hidden = !login;
    registerForm.hidden = !register;
    forgotForm.hidden = !forgot;
    resetForm.hidden = !reset;
    document.getElementById('siteLoginError').textContent = '';
    document.getElementById('siteRegisterError').textContent = '';
    document.getElementById('siteForgotError').textContent = '';
    document.getElementById('siteResetError').textContent = '';
    if (!forgot) document.getElementById('siteForgotSuccess').textContent = '';
    window.setTimeout(() => {
      const targets = {
        login: document.getElementById('siteLoginEmail'),
        register: document.getElementById('registerCompanyName'),
        forgot: document.getElementById('forgotEmail'),
        reset: document.getElementById('resetPassword')
      };
      targets[mode]?.focus();
    }, 40);
  }

  function openAuthModal(mode = 'login') {
    setAuthMode(mode);
    authModal.classList.add('open');
    authModal.setAttribute('aria-hidden', 'false');
    document.body.style.overflow = 'hidden';
  }

  function closeAuthModal() {
    authModal.classList.remove('open');
    authModal.setAttribute('aria-hidden', 'true');
    document.body.style.overflow = '';
  }

  async function finishAuthentication(data, message) {
    currentUser = {
      ...data.user,
      company: data.company
    };
    updateAccountButtons();
    closeAuthModal();
    showToast(message);
    if (pendingInstall) {
      pendingInstall = false;
      window.location.href = '/instalar';
    }
  }

  loginForm.addEventListener('submit', async event => {
    event.preventDefault();
    const error = document.getElementById('siteLoginError');
    const button = loginForm.querySelector('button[type="submit"]');
    error.textContent = '';
    button.disabled = true;
    button.textContent = 'Entrando…';
    try {
      const data = await api('/api/auth/login', {
        method: 'POST',
        body: JSON.stringify({
          email: document.getElementById('siteLoginEmail').value,
          password: document.getElementById('siteLoginPassword').value
        })
      });
      await finishAuthentication(data, 'Login realizado com sucesso.');
    } catch (err) {
      error.textContent = err.message;
    } finally {
      button.disabled = false;
      button.textContent = 'Entrar';
    }
  });

  registerForm.addEventListener('submit', async event => {
    event.preventDefault();
    const error = document.getElementById('siteRegisterError');
    const button = registerForm.querySelector('button[type="submit"]');
    error.textContent = '';
    button.disabled = true;
    button.textContent = 'Criando empresa…';
    try {
      const data = await api('/api/auth/register-company', {
        method: 'POST',
        body: JSON.stringify({
          company_name: document.getElementById('registerCompanyName').value,
          responsible_name: document.getElementById('registerResponsibleName').value,
          email: document.getElementById('registerEmail').value,
          password: document.getElementById('registerPassword').value,
          password_confirmation: document.getElementById('registerPasswordConfirmation').value
        })
      });
      await finishAuthentication(data, 'Empresa criada e acesso liberado.');
    } catch (err) {
      error.textContent = err.message;
    } finally {
      button.disabled = false;
      button.textContent = 'Criar empresa e entrar';
    }
  });

  forgotForm.addEventListener('submit', async event => {
    event.preventDefault();
    const error = document.getElementById('siteForgotError');
    const success = document.getElementById('siteForgotSuccess');
    const button = forgotForm.querySelector('button[type="submit"]');
    error.textContent = '';
    success.textContent = '';
    button.disabled = true;
    button.textContent = 'Enviando…';
    try {
      const data = await api('/api/auth/password-reset/request', {
        method: 'POST',
        body: JSON.stringify({ email: document.getElementById('forgotEmail').value })
      });
      success.textContent = data.message || 'Se o e-mail estiver cadastrado, enviaremos as instruções.';
      showToast('Solicitação de recuperação registrada.');
    } catch (err) {
      error.textContent = err.message;
      showToast(err.message, true);
    } finally {
      button.disabled = false;
      button.textContent = 'Enviar link de recuperação';
    }
  });

  resetForm.addEventListener('submit', async event => {
    event.preventDefault();
    const error = document.getElementById('siteResetError');
    const button = resetForm.querySelector('button[type="submit"]');
    const password = document.getElementById('resetPassword').value;
    const confirmation = document.getElementById('resetPasswordConfirmation').value;
    error.textContent = '';
    if (!resetToken) {
      error.textContent = 'O link de recuperação não possui um token válido.';
      return;
    }
    if (password !== confirmation) {
      error.textContent = 'As senhas não conferem.';
      return;
    }
    button.disabled = true;
    button.textContent = 'Salvando…';
    try {
      const data = await api('/api/auth/password-reset/confirm', {
        method: 'POST',
        body: JSON.stringify({
          token: resetToken,
          password,
          password_confirmation: confirmation
        })
      });
      resetToken = '';
      window.history.replaceState({}, document.title, window.location.pathname);
      document.getElementById('resetPassword').value = '';
      document.getElementById('resetPasswordConfirmation').value = '';
      setAuthMode('login');
      showToast(data.message || 'Senha redefinida com sucesso.');
    } catch (err) {
      error.textContent = err.message;
      showToast(err.message, true);
    } finally {
      button.disabled = false;
      button.textContent = 'Salvar nova senha';
    }
  });

  document.querySelectorAll('.js-download').forEach(button => button.addEventListener('click', () => {
    window.location.href = '/instalar';
  }));
  document.querySelectorAll('.js-register-download').forEach(button => button.addEventListener('click', () => {
    if (currentUser) {
      window.location.href = '/instalar';
      return;
    }
    pendingInstall = true;
    openAuthModal('register');
  }));
  loginTab.addEventListener('click', () => setAuthMode('login'));
  registerTab.addEventListener('click', () => setAuthMode('register'));
  forgotPasswordButton.addEventListener('click', () => {
    document.getElementById('forgotEmail').value = document.getElementById('siteLoginEmail').value;
    setAuthMode('forgot');
  });
  backToLoginButton.addEventListener('click', () => setAuthMode('login'));
  cancelResetButton.addEventListener('click', () => {
    resetToken = '';
    window.history.replaceState({}, document.title, window.location.pathname);
    setAuthMode('login');
  });
  closeAuthButton.addEventListener('click', closeAuthModal);
  authModal.addEventListener('click', event => { if (event.target === authModal) closeAuthModal(); });
  document.addEventListener('keydown', event => {
    if (event.key === 'Escape' && authModal.classList.contains('open')) closeAuthModal();
  });

  function accountAction() {
    if (currentUser) window.location.href = '/central';
    else openAuthModal('login');
  }
  accountButton?.addEventListener('click', accountAction);
  mobileAccountButton?.addEventListener('click', accountAction);

  menuButton.addEventListener('click', () => {
    const open = mobileNav.classList.toggle('open');
    menuButton.setAttribute('aria-expanded', String(open));
  });
  mobileNav.querySelectorAll('a,button').forEach(item => item.addEventListener('click', () => {
    mobileNav.classList.remove('open');
    menuButton.setAttribute('aria-expanded', 'false');
  }));

  // Calculadora comercial: deixa o visitante usar os próprios números, sem inventar ROI.
  const lossFields = [
    document.getElementById('lossPeople'),
    document.getElementById('lossHourly'),
    document.getElementById('lossHours'),
    document.getElementById('lossEvents')
  ];
  const lossMonthly = document.getElementById('lossMonthly');
  const lossYearly = document.getElementById('lossYearly');
  const money = new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' });

  function safeNumber(input) {
    const value = Number.parseFloat(input?.value || '0');
    return Number.isFinite(value) && value > 0 ? value : 0;
  }

  function updateLossCalculator() {
    if (!lossMonthly || !lossYearly) return;
    const people = safeNumber(lossFields[0]);
    const hourly = safeNumber(lossFields[1]);
    const hours = safeNumber(lossFields[2]);
    const events = safeNumber(lossFields[3]);
    const monthly = people * hourly * hours * events;
    const yearly = monthly * 12;
    lossMonthly.textContent = `${money.format(monthly)} / mês`;
    lossYearly.textContent = `${money.format(yearly)} / ano, se o padrão se repetir.`;
  }

  lossFields.forEach(field => field?.addEventListener('input', updateLossCalculator));
  updateLossCalculator();

  const query = new URLSearchParams(window.location.search);
  resetToken = query.get('reset_token') || '';
  if (resetToken) {
    openAuthModal('reset');
  } else if (query.get('forgot') === '1') {
    openAuthModal('forgot');
  }

  loadSession();
})();

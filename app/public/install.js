(() => {
  const form = document.getElementById('installCodeForm');
  const input = document.getElementById('installCode');
  const error = document.getElementById('installCodeError');
  const button = document.getElementById('installCodeButton');
  const valid = document.getElementById('installValid');
  const companyName = document.getElementById('installCompanyName');
  const fallback = document.getElementById('installFallback');

  function normalizeCode(value) {
    let compact = String(value || '').toUpperCase().replace(/[^A-Z0-9]/g, '');
    if (compact.length === 8) compact = `CC${compact}`;
    if (compact.length !== 10 || !compact.startsWith('CC')) return '';
    const body = compact.slice(2);
    return `CC-${body.slice(0, 4)}-${body.slice(4)}`;
  }

  function formatWhileTyping(value) {
    let compact = String(value || '').toUpperCase().replace(/[^A-Z0-9]/g, '');
    if (compact === 'C') return 'C';
    if (!compact.startsWith('CC')) {
      compact = compact.startsWith('C') ? `CC${compact.slice(1)}` : `CC${compact}`;
    }
    compact = compact.slice(0, 10);
    const body = compact.slice(2);
    if (compact.length <= 2) return compact;
    if (body.length <= 4) return `CC-${body}`;
    return `CC-${body.slice(0, 4)}-${body.slice(4)}`;
  }

  async function readJson(response) {
    return response.json().catch(() => ({}));
  }

  function startDownload(code) {
    const url = `/instalar/codigo/${encodeURIComponent(code)}`;
    fallback.href = url;
    fallback.classList.remove('hidden');
    const link = document.createElement('a');
    link.href = url;
    link.download = `CoreControlSetup--${code}.exe`;
    link.rel = 'noopener';
    link.style.display = 'none';
    document.body.appendChild(link);
    link.click();
    link.remove();
  }

  input.addEventListener('input', () => {
    const cursorAtEnd = input.selectionStart === input.value.length;
    input.value = formatWhileTyping(input.value);
    if (cursorAtEnd) input.setSelectionRange(input.value.length, input.value.length);
    error.textContent = '';
    valid.classList.add('hidden');
    fallback.classList.add('hidden');
  });

  form.addEventListener('submit', async event => {
    event.preventDefault();
    error.textContent = '';
    valid.classList.add('hidden');
    fallback.classList.add('hidden');

    const code = normalizeCode(input.value);
    if (!code) {
      error.textContent = 'Digite um código válido no formato CC-XXXX-XXXX.';
      input.focus();
      return;
    }

    input.value = code;
    button.disabled = true;
    button.textContent = 'Validando código…';
    try {
      const response = await fetch(`/api/enrollment/${encodeURIComponent(code)}/info`, {
        method: 'GET',
        credentials: 'omit',
        headers: { Accept: 'application/json' }
      });
      const data = await readJson(response);
      if (!response.ok) throw new Error(data.detail || 'Código inválido, expirado ou já utilizado.');

      companyName.textContent = data.company_name || 'Empresa autorizada';
      valid.classList.remove('hidden');
      button.textContent = 'Código validado';
      startDownload(code);
    } catch (err) {
      error.textContent = err.message || 'Não foi possível validar o código.';
      button.textContent = 'Validar código e baixar';
      input.select();
    } finally {
      button.disabled = false;
    }
  });

  input.focus();
})();

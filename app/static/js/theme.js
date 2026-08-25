(function () {
  'use strict';

  const STORAGE_KEY = 'corecontrol-theme';
  const THEMES = new Set(['light', 'dark']);

  function readTheme() {
    try {
      const saved = localStorage.getItem(STORAGE_KEY);
      return THEMES.has(saved) ? saved : 'light';
    } catch (_) {
      return 'light';
    }
  }

  function updateControls(theme) {
    const state = document.getElementById('themeState');
    if (state) state.textContent = theme === 'dark' ? 'Escuro' : 'Claro';

    document.querySelectorAll('[data-theme-option]').forEach((button) => {
      const active = button.dataset.themeOption === theme;
      button.classList.toggle('active', active);
      button.setAttribute('aria-checked', String(active));
    });
  }

  function applyTheme(theme, persist) {
    const normalized = THEMES.has(theme) ? theme : 'light';
    document.documentElement.dataset.theme = normalized;
    document.documentElement.style.colorScheme = normalized;

    if (persist) {
      try { localStorage.setItem(STORAGE_KEY, normalized); } catch (_) {}
    }

    updateControls(normalized);
    window.dispatchEvent(new CustomEvent('corecontrol:themechange', {
      detail: { theme: normalized },
    }));
  }

  function closeMenu() {
    const menu = document.getElementById('appearanceMenu');
    const button = document.getElementById('appearanceBtn');
    if (!menu || !button) return;
    menu.classList.add('hidden');
    button.classList.remove('open');
    button.setAttribute('aria-expanded', 'false');
  }

  function bindThemeControls() {
    const button = document.getElementById('appearanceBtn');
    const menu = document.getElementById('appearanceMenu');
    if (!button || !menu) return;

    updateControls(document.documentElement.dataset.theme || readTheme());

    button.addEventListener('click', (event) => {
      event.stopPropagation();
      const opening = menu.classList.contains('hidden');
      menu.classList.toggle('hidden', !opening);
      button.classList.toggle('open', opening);
      button.setAttribute('aria-expanded', String(opening));
    });

    menu.addEventListener('click', (event) => {
      event.stopPropagation();
      const option = event.target.closest('[data-theme-option]');
      if (!option) return;
      applyTheme(option.dataset.themeOption, true);
      closeMenu();
    });

    document.addEventListener('click', (event) => {
      if (!event.target.closest('.settings-wrap')) closeMenu();
    });
    document.addEventListener('keydown', (event) => {
      if (event.key === 'Escape') closeMenu();
    });
  }

  window.CoreControlTheme = { apply: applyTheme, current: readTheme };
  applyTheme(readTheme(), false);
  document.addEventListener('DOMContentLoaded', bindThemeControls);
})();

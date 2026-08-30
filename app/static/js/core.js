(function () {
  'use strict';

  const CT = window.CoreTuner = window.CoreTuner || {};

  CT.VERSION = '20260830-device-reinstall-v1';
  CT.state = {
    user: null,
    page: 'overview',
    refreshTimer: null,
    selectedCompany: null,
    selectedDevice: null,
  };
  CT.pageRenderers = {};
  CT.templateCache = new Map();

  CT.$ = (selector, root = document) => root.querySelector(selector);
  CT.$$ = (selector, root = document) => [...root.querySelectorAll(selector)];

  CT.esc = function esc(value = '') {
    return String(value ?? '').replace(/[&<>'"]/g, (character) => ({
      '&': '&amp;',
      '<': '&lt;',
      '>': '&gt;',
      "'": '&#39;',
      '"': '&quot;',
    })[character]);
  };

  CT.fmtDate = function fmtDate(value) {
    if (!value) return '—';
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString('pt-BR');
  };

  CT.fmtNum = function fmtNum(value, digits = 0) {
    return value == null
      ? '—'
      : Number(value).toLocaleString('pt-BR', {
          maximumFractionDigits: digits,
          minimumFractionDigits: digits,
        });
  };

  CT.roleName = function roleName(role) {
    return ({
      global_admin: 'Administrador Global',
      platform_admin: 'Administrador da plataforma',
      company_admin: 'Administrador da empresa',
      technician: 'Técnico',
      viewer: 'Visualização',
    })[role] || role;
  };

  CT.isGlobalAdmin = function isGlobalAdmin(user = CT.state.user) {
    return ['global_admin', 'platform_admin'].includes(user?.role);
  };

  CT.canDestroyCompanies = function canDestroyCompanies(user = CT.state.user) {
    return user?.role === 'global_admin';
  };

  CT.healthClass = function healthClass(score) {
    return score >= 80 ? 'good' : score >= 55 ? 'warn' : 'bad';
  };

  CT.toast = function toast(message, error = false) {
    const element = CT.$('#toast');
    if (!element) return;
    element.textContent = message;
    element.className = `toast show${error ? ' error' : ''}`;
    clearTimeout(element._timer);
    element._timer = setTimeout(() => {
      element.className = 'toast';
    }, 3200);
  };

  CT.api = async function api(path, options = {}) {
    const requestOptions = {
      credentials: 'same-origin',
      headers: {
        ...(options.body ? { 'Content-Type': 'application/json' } : {}),
        ...(options.headers || {}),
      },
      ...options,
    };

    const response = await fetch(`/api${path}`, requestOptions);
    let data = null;
    try {
      data = await response.json();
    } catch (_) {
      data = null;
    }

    if (
      response.status === 401 &&
      !['/auth/login', '/auth/register-company'].includes(path)
    ) {
      if (typeof CT.showLogin === 'function') CT.showLogin();
      throw new Error('Sessão expirada');
    }

    if (!response.ok) {
      const detail = Array.isArray(data?.detail)
        ? data.detail.map((item) => item.msg).join(' ')
        : data?.detail;
      throw new Error(detail || 'Não foi possível concluir a operação');
    }

    return data;
  };

  CT.loadTemplate = async function loadTemplate(relativePath) {
    const cleanPath = String(relativePath).replace(/^\/+/, '');
    if (CT.templateCache.has(cleanPath)) {
      return CT.templateCache.get(cleanPath);
    }

    const response = await fetch(
      `/static/${cleanPath}?v=${encodeURIComponent(CT.VERSION)}`,
      { credentials: 'same-origin' },
    );
    if (!response.ok) {
      throw new Error(`Não foi possível carregar a tela ${cleanPath}`);
    }

    const html = await response.text();
    CT.templateCache.set(cleanPath, html);
    return html;
  };

  CT.mountTemplate = async function mountTemplate(target, relativePath) {
    const element = typeof target === 'string' ? CT.$(target) : target;
    if (!element) throw new Error(`Área da tela não encontrada: ${target}`);
    element.innerHTML = await CT.loadTemplate(relativePath);
    return element;
  };

  CT.mountPage = async function mountPage(pageName) {
    return CT.mountTemplate('#content', `pages/${pageName}.html`);
  };

  CT.registerPage = function registerPage(pageName, renderer) {
    CT.pageRenderers[pageName] = renderer;
  };
})();

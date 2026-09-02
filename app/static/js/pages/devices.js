(function () {
  'use strict';

  const CT = window.CoreTuner;

  CT.registerPage('devices', async function renderDevices() {
    const devices = await CT.api('/devices');
    await CT.mountPage('devices');

    CT.setAlertBadge(devices.reduce((sum, device) => sum + device.alerts_open, 0));
    CT.$('#devicesCount').textContent = `${devices.length} equipamento(s) autorizado(s).`;

    const renderTable = (items, emptyMessage) => {
      CT.$('#deviceTableArea').innerHTML = items.length
        ? CT.deviceTable(items)
        : `<div class="empty">${emptyMessage}</div>`;
      CT.bindDeviceRows();
    };

    renderTable(devices, 'Nenhum computador vinculado.');
    CT.$('#deviceSearch').oninput = (event) => {
      const query = event.target.value.toLowerCase();
      const filtered = devices.filter((device) => [
        device.name,
        device.hostname,
        device.company_name,
        device.sector,
        device.location,
        device.manufacturer,
        device.model,
      ].some((value) => String(value || '').toLowerCase().includes(query)));
      renderTable(filtered, 'Nenhum resultado.');
    };

    const addComputerBtn = CT.$('#addComputerBtn');
    const canEnroll = ['global_admin', 'platform_admin', 'company_admin', 'technician'].includes(CT.state.user?.role);
    addComputerBtn.classList.toggle('hidden', !canEnroll);
    if (canEnroll) {
      addComputerBtn.onclick = async () => {
        try {
          const companies = await CT.api('/companies');
          await CT.chooseEnrollmentCompany(companies);
        } catch (error) {
          CT.toast(error.message, true);
        }
      };
    }
  });

  function activityVersionAtLeast(version, minimum) {
    const parse = (value) => {
      const parts = String(value || '').replace(/^v/i, '').split('.').slice(0, 3).map((piece) => Number.parseInt(piece, 10) || 0);
      while (parts.length < 3) parts.push(0);
      return parts;
    };
    const current = parse(version);
    const wanted = parse(minimum);
    for (let index = 0; index < 3; index += 1) {
      if (current[index] !== wanted[index]) return current[index] > wanted[index];
    }
    return true;
  }

  function activityVersionSupported(version) {
    return activityVersionAtLeast(version, '0.6.0');
  }

  function activityIconVersionSupported(version) {
    return activityVersionAtLeast(version, '0.8.3');
  }

  function activityBrowserTabsVersionSupported(version) {
    return activityVersionAtLeast(version, '0.8.5');
  }

  function activityDuration(seconds) {
    let value = Math.max(0, Math.round(Number(seconds) || 0));
    if (value < 60) return `${value}s`;
    const minutes = Math.floor(value / 60);
    if (minutes < 60) return `${minutes}m`;
    return `${Math.floor(minutes / 60)}h ${String(minutes % 60).padStart(2, '0')}m`;
  }

  const activityAssets = new Map();
  const activityExpandedGroups = new Set();
  let activityLastDevice = null;
  let activityLastSuccessfulCommand = null;

  function activityGroupKeyFromBrowser(browser) {
    return activityProcessKey(activityBrowserProcess(browser));
  }

  function activityGroupedApplications(apps, browserTabs) {
    const groups = new Map();
    const ensureGroup = (processName, displayName = '') => {
      const key = activityProcessKey(processName) || String(processName || 'app').toLowerCase();
      if (!groups.has(key)) {
        groups.set(key, {
          key,
          process_name: processName,
          display_name: activityFriendlyName(processName, displayName),
          apps: [],
          tabs: [],
        });
      }
      return groups.get(key);
    };

    (apps || []).forEach((app) => {
      const group = ensureGroup(app?.process_name, app?.display_name);
      group.apps.push(app);
      if (app?.focused) group.focused = true;
    });

    (browserTabs || []).forEach((tab) => {
      const processName = activityBrowserProcess(tab?.browser);
      const group = ensureGroup(processName, activityBrowserName(tab?.browser));
      group.tabs.push(tab);
      if (tab?.active) group.focused = true;
    });

    return Array.from(groups.values()).map((group) => {
      const focusedApp = group.apps.find((app) => app?.focused) || group.apps[0] || null;
      const activeTab = group.tabs.find((tab) => tab?.active) || null;
      const useTabs = group.tabs.length > 0;
      const childCount = useTabs ? group.tabs.length : group.apps.length;
      const expandable = useTabs || group.apps.length > 1;
      const cpu = group.apps.reduce((sum, app) => sum + (Number(app?.cpu_percent) || 0), 0);
      const memory = group.apps.reduce((sum, app) => sum + (Number(app?.memory_mb) || 0), 0);
      return {
        ...group,
        focused: Boolean(group.focused),
        expandable,
        childCount,
        useTabs,
        cpu_percent: cpu,
        memory_mb: memory,
        window_title: activeTab?.title || focusedApp?.window_title || group.apps[0]?.window_title || '—',
      };
    });
  }

  function activityRenderGroupRows(group) {
    const asset = activityAssets.get(activityProcessKey(group.process_name));
    const displayName = activityFriendlyName(group.process_name, group.display_name || asset?.display_name);
    const encodedKey = encodeURIComponent(group.key);
    const expanded = activityExpandedGroups.has(group.key);
    const toggle = group.expandable
      ? `<button class="activity-group-toggle ${expanded ? 'expanded' : ''}" type="button" aria-expanded="${expanded ? 'true' : 'false'}" aria-label="${expanded ? 'Recolher' : 'Expandir'} ${CT.esc(displayName)}" data-activity-toggle="${encodedKey}"><span>›</span></button>`
      : '<span class="activity-group-toggle-spacer" aria-hidden="true"></span>';
    const count = group.expandable ? ` <span class="activity-group-number">(${group.childCount})</span>` : '';
    const status = group.focused ? '<span class="pill resolved">Em uso</span>' : '<span class="pill">Aberto</span>';
    let html = `<tr class="activity-group-parent"><td><div class="activity-app-name activity-group-app">${toggle}${activityAppIcon(group.process_name, group.focused)}<span class="activity-group-label" title="${CT.esc(displayName)}">${CT.esc(displayName)}${count}</span></div></td><td><div class="activity-window-title" title="${CT.esc(group.window_title || '')}">${CT.esc(group.window_title || '—')}</div></td><td>${CT.fmtNum(group.cpu_percent, 1)}%</td><td>${CT.fmtNum(group.memory_mb, 0)} MB</td><td>${status}</td></tr>`;

    if (!group.expandable) return html;

    const childRows = group.useTabs
      ? group.tabs.map((tab) => {
          const title = tab?.title || 'Página';
          const context = tab?.domain || tab?.url || 'Aba do navegador';
          return `<tr class="activity-tree-child" data-activity-child="${encodedKey}" ${expanded ? '' : 'hidden'}><td><div class="activity-child-app"><span class="activity-tree-branch" aria-hidden="true"></span>${activityTabIcon(tab, group.process_name, Boolean(tab?.active), 'activity-child-icon')}<span title="${CT.esc(title)}">${CT.esc(title)}</span></div></td><td><div class="activity-child-context" title="${CT.esc(tab?.url || context)}">${CT.esc(context)}</div></td><td class="activity-child-metric">—</td><td class="activity-child-metric">—</td><td>${tab?.active ? '<span class="pill resolved">Em uso</span>' : '<span class="pill">Aba aberta</span>'}</td></tr>`;
        }).join('')
      : group.apps.map((app) => {
          const title = app?.window_title || 'Janela';
          return `<tr class="activity-tree-child" data-activity-child="${encodedKey}" ${expanded ? '' : 'hidden'}><td><div class="activity-child-app"><span class="activity-tree-branch" aria-hidden="true"></span>${activityAppIcon(group.process_name, Boolean(app?.focused), 'activity-child-icon')}<span title="${CT.esc(title)}">${CT.esc(title)}</span></div></td><td><div class="activity-child-context" title="${CT.esc(title)}">${CT.esc(title)}</div></td><td class="activity-child-metric">${CT.fmtNum(app?.cpu_percent, 1)}%</td><td class="activity-child-metric">${CT.fmtNum(app?.memory_mb, 0)} MB</td><td>${app?.focused ? '<span class="pill resolved">Em uso</span>' : '<span class="pill">Aberto</span>'}</td></tr>`;
        }).join('');
    return html + childRows;
  }

  function activityBindGroupToggles(area) {
    area.querySelectorAll('[data-activity-toggle]').forEach((button) => {
      button.addEventListener('click', () => {
        const encodedKey = button.getAttribute('data-activity-toggle') || '';
        const key = decodeURIComponent(encodedKey);
        const expanded = !activityExpandedGroups.has(key);
        if (expanded) activityExpandedGroups.add(key);
        else activityExpandedGroups.delete(key);
        button.classList.toggle('expanded', expanded);
        button.setAttribute('aria-expanded', expanded ? 'true' : 'false');
        button.setAttribute('aria-label', `${expanded ? 'Recolher' : 'Expandir'} aplicativo`);
        area.querySelectorAll(`[data-activity-child="${CSS.escape(encodedKey)}"]`).forEach((row) => {
          row.hidden = !expanded;
        });
      });
    });
  }

  function activityProcessKey(name) {
    return String(name || '').trim().replace(/\.exe$/i, '').toLowerCase();
  }

  function activityFriendlyName(name, providedName) {
    const provided = String(providedName || '').trim();
    if (provided) return provided;
    const clean = String(name || '').trim().replace(/\.exe$/i, '');
    const aliases = {
      chrome: 'Google Chrome',
      msedge: 'Microsoft Edge',
      firefox: 'Mozilla Firefox',
      opera: 'Opera',
      opera_gx: 'Opera GX',
      brave: 'Brave',
      spotify: 'Spotify',
      anydesk: 'AnyDesk',
      teamviewer: 'TeamViewer',
      code: 'Visual Studio Code',
      whatsapp: 'WhatsApp',
      discord: 'Discord',
      slack: 'Slack',
      explorer: 'Explorador de Arquivos',
      systemsettings: 'Configurações',
      applicationframehost: 'Aplicativos do Windows',
      textinputhost: 'Microsoft Text Input',
      searchhost: 'Pesquisa do Windows',
      searchapp: 'Pesquisa do Windows',
      startmenuexperiencehost: 'Menu Iniciar',
      taskmgr: 'Gerenciador de Tarefas',
      notepad: 'Bloco de Notas',
      calculatorapp: 'Calculadora',
      calc: 'Calculadora',
      powershell: 'PowerShell',
      pwsh: 'PowerShell',
      cmd: 'Prompt de Comando',
      'nvidia overlay': 'NVIDIA Overlay',
      nvidiaoverlay: 'NVIDIA Overlay',
      'nvidia share': 'NVIDIA Overlay',
    };
    return aliases[activityProcessKey(clean)] || clean || 'Aplicativo';
  }

  function activityIconData(value) {
    const icon = String(value || '').trim();
    if (icon.length > 131072 || !/^data:image\/png;base64,[a-z0-9+/=]+$/i.test(icon)) return '';
    return icon;
  }

  function activityRememberAssets(result) {
    const assets = result?.app_assets || {};
    Object.entries(assets).forEach(([key, raw]) => {
      const process = raw?.process_name || key;
      const processKey = activityProcessKey(process);
      if (!processKey) return;
      const previous = activityAssets.get(processKey) || {};
      activityAssets.set(processKey, {
        process_name: process,
        display_name: activityFriendlyName(process, raw?.display_name || previous.display_name),
        icon_data: activityIconData(raw?.icon_data) || previous.icon_data || '',
      });
    });
    (result?.apps || []).forEach((app) => {
      const processKey = activityProcessKey(app?.process_name);
      if (!processKey) return;
      const previous = activityAssets.get(processKey) || {};
      activityAssets.set(processKey, {
        process_name: app.process_name,
        display_name: activityFriendlyName(app.process_name, app.display_name || previous.display_name),
        icon_data: previous.icon_data || '',
      });
    });
  }

  function activityResultHasRealIcons(result) {
    return Object.values(result?.app_assets || {}).some((asset) => Boolean(activityIconData(asset?.icon_data)));
  }

  function activityGlyph(processName) {
    const key = activityProcessKey(processName);
    if (key === 'systemsettings') return '⚙';
    if (key === 'chrome' || key === 'msedge' || key === 'firefox' || key === 'opera' || key === 'opera_gx') return '◉';
    if (key === 'spotify') return '♫';
    if (key === 'anydesk' || key === 'teamviewer') return '↔';
    if (key.includes('nvidia')) return 'N';
    const name = activityFriendlyName(processName);
    return (name.match(/[A-ZÀ-Ý0-9]/i)?.[0] || '•').toUpperCase();
  }

  function activityAppIcon(processName, focused = false, extraClass = '') {
    const asset = activityAssets.get(activityProcessKey(processName));
    const icon = activityIconData(asset?.icon_data);
    const classes = ['activity-app-icon', focused ? 'focused' : '', extraClass].filter(Boolean).join(' ');
    if (icon) {
      return `<span class="${classes}" aria-hidden="true"><img src="${icon}" alt=""></span>`;
    }
    return `<span class="${classes} fallback" aria-hidden="true"><b>${CT.esc(activityGlyph(processName))}</b></span>`;
  }

  function activityBrowserProcess(browser) {
    const key = String(browser || '').trim().toLowerCase();
    if (key === 'edge') return 'msedge';
    if (key === 'opera') return 'opera';
    if (key === 'brave') return 'brave';
    return key || 'browser';
  }

  function activityBrowserName(browser) {
    return activityFriendlyName(activityBrowserProcess(browser));
  }

  function activityTabKnownIcon(tab) {
    const title = String(tab?.title || '').trim();
    const haystack = `${title} ${String(tab?.domain || '')} ${String(tab?.url || '')}`.toLowerCase();

    // O fallback por título é importante para as abas coletadas via Windows UI Automation:
    // nesse modo o Agent enxerga todas as abas, mas o Chromium não expõe a URL das abas em segundo plano.
    const known = [
      { test: /(^|\s)gmail(\s|$)|caixa de entrada|@gmail\.com|mail\.google/i, domain: 'mail.google.com' },
      { test: /google\s*(maps|mapas)|maps\.google/i, domain: 'maps.google.com' },
      { test: /google\s*(imagens|images)|images\.google/i, domain: 'images.google.com' },
      { test: /google\s*(agenda|calendar)|calendar\.google/i, domain: 'calendar.google.com' },
      { test: /(contatos do google|google contacts|contacts\.google)/i, domain: 'contacts.google.com' },
      { test: /(google drive|drive\.google)/i, domain: 'drive.google.com' },
      { test: /youtube|youtu\.be/i, domain: 'youtube.com' },
      { test: /instagram/i, domain: 'instagram.com' },
      { test: /facebook/i, domain: 'facebook.com' },
      { test: /linkedin/i, domain: 'linkedin.com' },
      { test: /whatsapp/i, domain: 'web.whatsapp.com' },
      { test: /github/i, domain: 'github.com' },
      { test: /canva/i, domain: 'canva.com' },
      { test: /segware/i, domain: 'segware.com.br' },
    ];
    const match = known.find((item) => item.test.test(haystack));
    if (match) {
      return `https://www.google.com/s2/favicons?sz=64&domain=${encodeURIComponent(match.domain)}`;
    }

    if (/corecontrol/i.test(haystack)) {
      return '/static/corecontrol-mark.png';
    }
    return '';
  }

  function activityTabFaviconSource(tab) {
    // O Browser Bridge usa fav_icon_url. As outras grafias ficam aceitas por compatibilidade.
    const direct = [tab?.fav_icon_url, tab?.favicon_url, tab?.favicon, tab?.icon_url, tab?.icon]
      .map((value) => String(value || '').trim())
      .find((value) => Boolean(value));
    if (direct && (/^https?:\/\//i.test(direct) || /^data:image\//i.test(direct) || direct.startsWith('/'))) {
      return direct;
    }

    const rawUrl = String(tab?.url || '').trim();
    if (/^https?:\/\//i.test(rawUrl)) {
      try {
        const parsed = new URL(rawUrl);
        if (parsed.hostname && !/^(localhost|127\.0\.0\.1)$/i.test(parsed.hostname)) {
          return `https://www.google.com/s2/favicons?sz=64&domain_url=${encodeURIComponent(parsed.origin)}`;
        }
      } catch (_) {}
    }

    const domain = String(tab?.domain || '').trim().replace(/^\.+/, '');
    if (domain && /^[a-z0-9.-]+\.[a-z]{2,}$/i.test(domain)) {
      return `https://www.google.com/s2/favicons?sz=64&domain=${encodeURIComponent(domain)}`;
    }

    return activityTabKnownIcon(tab);
  }

  function activityTabIcon(tab, browserProcess, focused = false, extraClass = '') {
    const classes = ['activity-app-icon', focused ? 'focused' : '', extraClass].filter(Boolean).join(' ');
    const favicon = activityTabFaviconSource(tab);
    if (!favicon) {
      return activityAppIcon(browserProcess, focused, extraClass);
    }

    const browserAsset = activityAssets.get(activityProcessKey(browserProcess));
    const browserIcon = activityIconData(browserAsset?.icon_data);
    const browserFallback = browserIcon
      ? `<img class="activity-tab-browser-fallback" src="${browserIcon}" alt="" hidden>`
      : `<b class="activity-tab-glyph-fallback" hidden>${CT.esc(activityGlyph(browserProcess))}</b>`;

    return `<span class="${classes} activity-tab-site-icon" aria-hidden="true"><img class="activity-tab-favicon" src="${CT.esc(favicon)}" alt="" loading="lazy" referrerpolicy="no-referrer" onerror="this.hidden=true; const fallback=this.parentNode.querySelector('.activity-tab-browser-fallback,.activity-tab-glyph-fallback'); if(fallback) fallback.hidden=false;">${browserFallback}</span>`;
  }

  function activitySegments(history) {
    const samples = (history || []).filter((sample) => sample?.activity?.process_name && sample.recorded_at);
    const segments = [];
    for (const sample of samples) {
      const activity = sample.activity;
      const at = new Date(sample.recorded_at);
      if (Number.isNaN(at.getTime())) continue;
      const key = `${activity.process_name}\u0000${activity.window_title || ''}`;
      const current = segments[segments.length - 1];
      if (!current || current.key !== key) {
        segments.push({
          key,
          process: activity.process_name,
          title: activity.window_title || 'Janela sem título',
          started: at,
          last: at,
        });
      } else {
        current.last = at;
      }
    }
    return segments.reverse().slice(0, 6).map((segment, index) => ({
      ...segment,
      seconds: Math.max(30, Math.round((segment.last - segment.started) / 1000) + 30),
      current: index === 0,
    }));
  }

  function renderActivityCurrent(device) {
    const activity = device.telemetry?.activity;
    const target = CT.$('#activityCurrent');
    if (!target) return;
    if (!activity?.process_name) {
      target.innerHTML = `<div class="activity-empty">${activityVersionSupported(device.agent_version) ? 'Aguardando a próxima amostra de atividade do computador.' : 'Atualize o CoreControl Agent para 0.6.0 ou superior para receber atividade.'}</div>`;
      return;
    }
    const asset = activityAssets.get(activityProcessKey(activity.process_name));
    target.innerHTML = `
      <div class="activity-current-main">
        ${activityAppIcon(activity.process_name, true, 'activity-current-icon')}
        <div class="activity-current-copy"><strong>${CT.esc(activityFriendlyName(activity.process_name, asset?.display_name))}</strong><span>${CT.esc(activity.window_title || 'Janela em primeiro plano')}</span></div>
        <span class="activity-current-meta">Em uso · ${CT.fmtDate(device.telemetry.recorded_at)}</span>
      </div>`;
  }

  function renderActivityTimeline(history) {
    const target = CT.$('#activityTimeline');
    if (!target) return;
    const items = activitySegments(history);
    target.innerHTML = items.length ? items.map((item) => {
      const asset = activityAssets.get(activityProcessKey(item.process));
      return `
      <div class="activity-event ${item.current ? 'current' : ''}">
        ${activityAppIcon(item.process, item.current, 'activity-event-icon')}
        <div><strong>${CT.esc(activityFriendlyName(item.process, asset?.display_name))}</strong><small>${CT.esc(item.title)}</small></div>
        <time>${activityDuration(item.seconds)}</time>
      </div>`;
    }).join('') : '<div class="activity-empty">A linha do tempo aparecerá quando o Agent registrar mudanças de aplicativo.</div>';
  }

  function activityRenderSuccessfulCommand(command, statusText = null) {
    const status = CT.$('#activitySnapshotStatus');
    const area = CT.$('#activityAppsArea');
    if (!status || !area || !command) return;

    if (command.status === 'succeeded' && command.result) {
      activityLastSuccessfulCommand = command;
    }

    const apps = command.result?.apps || [];
    const browserTabs = command.result?.browser_tabs || [];
    activityRememberAssets(command.result || {});
    const assetsList = Object.values(command.result?.app_assets || {});
    const iconCount = assetsList.filter((asset) => Boolean(activityIconData(asset?.icon_data))).length;
    const appRowsWithIcon = apps.filter((app) => Boolean(activityIconData(activityAssets.get(activityProcessKey(app?.process_name))?.icon_data))).length;
    const realIcons = iconCount > 0;
    const tabSuffix = browserTabs.length ? ` · ${browserTabs.length} aba${browserTabs.length === 1 ? '' : 's'}` : '';
    status.textContent = statusText || (!realIcons && (apps.length || browserTabs.length)
      ? `Atualizado · 0 ícones recebidos${tabSuffix}`
      : realIcons && apps.length ? `Atualizado · ${appRowsWithIcon}/${apps.length} janelas com ícone${tabSuffix}`
      : realIcons ? `Atualizado · ${iconCount} ícone(s) real(is)${tabSuffix}`
      : command.finished_at ? `Atualizado ${CT.fmtDate(command.finished_at)}${tabSuffix}` : `Atualizado${tabSuffix}`);

    const groups = activityGroupedApplications(apps, browserTabs);
    if (!groups.length) {
      area.innerHTML = '<div class="activity-empty">Nenhuma atividade visível foi encontrada.</div>';
    } else {
      area.innerHTML = `<div class="activity-browser-title activity-grouped-title">Aplicativos com janela aberta <span class="activity-count">${groups.length}</span><small>Clique na seta para ver todas as abas e janelas agrupadas.</small></div><div class="table-wrap activity-table-scroll"><table class="activity-table activity-grouped-table"><thead><tr><th>Aplicativo</th><th>Janela</th><th>CPU</th><th>Memória</th><th>Status</th></tr></thead><tbody>${groups.map(activityRenderGroupRows).join('')}</tbody></table></div>`;
      activityBindGroupToggles(area);
    }

    if (activityLastDevice) {
      renderActivityCurrent(activityLastDevice);
      renderActivityTimeline(activityLastDevice.history);
    }
  }

  function renderActivitySnapshot(snapshot) {
    const status = CT.$('#activitySnapshotStatus');
    const area = CT.$('#activityAppsArea');
    if (!status || !area) return;
    const command = snapshot?.command;
    const serverCached = snapshot?.cached_command;
    if (serverCached?.status === 'succeeded' && serverCached.result) {
      activityLastSuccessfulCommand = serverCached;
    }
    const cached = serverCached?.status === 'succeeded' ? serverCached : activityLastSuccessfulCommand;
    if (!snapshot?.agent_supports_activity) {
      status.textContent = 'Agent antigo';
      area.innerHTML = '<div class="activity-empty">Execute novamente o CoreControl Setup para instalar o Agent 0.6.0 ou superior.</div>';
      return;
    }
    if (!command) {
      if (cached?.status === 'succeeded') {
        activityRenderSuccessfulCommand(cached);
        return;
      }
      status.textContent = 'Ainda não consultado';
      area.innerHTML = '<div class="activity-empty">A primeira lista será carregada automaticamente em instantes.</div>';
      return;
    }
    if (['queued', 'running'].includes(command.status)) {
      if (cached?.status === 'succeeded') {
        const when = cached.finished_at ? CT.fmtDate(cached.finished_at) : 'anterior';
        activityRenderSuccessfulCommand(cached, `${command.status === 'running' ? 'Atualizando...' : 'Na fila...'} · exibindo ${when}`);
      } else {
        status.textContent = command.status === 'running' ? 'Coletando...' : 'Na fila...';
        area.innerHTML = '<div class="activity-empty">Primeira coleta em andamento. A lista aparecerá assim que o Agent responder.</div>';
      }
      return;
    }
    if (command.status === 'failed') {
      if (cached?.status === 'succeeded') {
        const when = cached.finished_at ? CT.fmtDate(cached.finished_at) : 'anterior';
        activityRenderSuccessfulCommand(cached, `Falha ao atualizar · exibindo ${when}`);
      } else {
        status.textContent = 'Falhou';
        area.innerHTML = `<div class="activity-empty">${CT.esc(command.error || 'Não foi possível consultar os aplicativos.')}</div>`;
      }
      return;
    }

    activityRenderSuccessfulCommand(command);
  }

  async function loadActivitySnapshot(deviceId) {
    try {
      const snapshot = await CT.api(`/devices/${deviceId}/activity/snapshot`);
      renderActivitySnapshot(snapshot);
      return snapshot;
    } catch (error) {
      const status = CT.$('#activitySnapshotStatus');
      const area = CT.$('#activityAppsArea');
      if (activityLastSuccessfulCommand?.status === 'succeeded') {
        activityRenderSuccessfulCommand(activityLastSuccessfulCommand, 'Falha ao atualizar · exibindo última lista');
      } else {
        if (status) status.textContent = 'Falha ao atualizar';
        if (area) area.innerHTML = `<div class="activity-empty">${CT.esc(error.message || 'Falha ao carregar atividade.')}</div>`;
      }
      return null;
    }
  }

  async function requestActivitySnapshot(device, options = {}) {
    const button = CT.$('#activityRefreshBtn');
    const silent = Boolean(options.silent);
    if (!button || button.disabled && !silent) return;
    if (!silent) {
      button.disabled = true;
      button.textContent = 'Solicitando...';
    }
    try {
      const response = await CT.api(`/devices/${device.id}/activity/snapshot`, { method: 'POST' });
      renderActivitySnapshot(response);
      const commandId = response.command?.id;
      for (let attempt = 0; attempt < 18; attempt += 1) {
        await new Promise((resolve) => setTimeout(resolve, 2000));
        const current = await loadActivitySnapshot(device.id);
        if (!current?.command || current.command.id !== commandId || !['queued', 'running'].includes(current.command.status)) break;
      }
    } catch (error) {
      if (!silent) CT.toast(error.message || 'Não foi possível consultar a atividade.', 'error');
    } finally {
      if (!silent) {
        button.disabled = !device.online || !activityVersionSupported(device.agent_version);
        button.textContent = 'Atualizar aplicativos';
      }
    }
  }

  let activityAutoRefreshToken = 0;
  let activityAutoRefreshInFlight = false;

  function activityDevicePageStillOpen(deviceId, token) {
    return token === activityAutoRefreshToken
      && Number(CT.state.selectedDevice) === Number(deviceId)
      && Boolean(CT.$('#activityAppsArea'));
  }

  async function activityRunAutomaticRefresh(device) {
    if (activityAutoRefreshInFlight) return;
    if (!device?.online || !activityVersionSupported(device.agent_version)) return;
    if (!CT.$('#activityAppsArea')) return;

    activityAutoRefreshInFlight = true;
    try {
      await requestActivitySnapshot(device, { silent: true });
    } finally {
      activityAutoRefreshInFlight = false;
    }
  }

  function startActivityAutoRefresh(device) {
    const token = ++activityAutoRefreshToken;
    if (!device?.online || !activityVersionSupported(device.agent_version)) return;

    // Atualiza imediatamente ao abrir a tela, sem apagar a última lista salva.
    activityRunAutomaticRefresh(device);

    const tick = async () => {
      if (!activityDevicePageStillOpen(device.id, token)) return;
      await activityRunAutomaticRefresh(device);
      if (!activityDevicePageStillOpen(device.id, token)) return;
      window.setTimeout(tick, 5000);
    };

    window.setTimeout(tick, 5000);
  }

  const optimizationAutoDiagnoseStarted = new Set();

  function optimizationCanManage() {
    return ['global_admin', 'platform_admin', 'company_admin'].includes(CT.state.user?.role);
  }

  function optimizationStatusLabel(state) {
    const command = state?.command;
    if (!state?.agent_supports_optimization) return ['Agent antigo', 'bad'];
    if (!state?.online) return ['Computador offline', 'bad'];
    if (command && ['queued', 'running'].includes(command.status)) {
      return [command.status === 'running' ? 'Aplicando perfil...' : 'Otimização na fila...', 'busy'];
    }
    if (command?.status === 'failed') return ['Última tentativa falhou', 'bad'];
    if (state?.active_profile_name) return [`Ativo: ${state.active_profile_name}`, 'active'];
    return ['Nenhum perfil ativo', 'good'];
  }

  function optimizationFeedbackHTML(command) {
    if (!command) return '';
    if (['queued', 'running'].includes(command.status)) {
      return '<strong>Otimização em andamento</strong>O CoreControl está medindo o computador, aplicando o perfil e comparando o estado antes e depois.';
    }
    if (command.status === 'failed') {
      const partial = command.result || {};
      const warnings = Array.isArray(partial.warnings) ? partial.warnings : [];
      const list = warnings.length ? `<ul>${warnings.map((item) => `<li>${CT.esc(item)}</li>`).join('')}</ul>` : '';
      return `<strong>Não foi possível concluir</strong>${CT.esc(command.error || 'A otimização falhou.')}${list}`;
    }
    if (command.status !== 'succeeded') return '';
    const result = command.result || {};
    const changed = Array.isArray(result.changed) ? result.changed : [];
    const warnings = Array.isArray(result.warnings) ? result.warnings : [];
    const summary = result.summary || {};
    const title = result.restored ? 'Configurações anteriores restauradas' : `${result.profile_name || 'Perfil'} aplicado com sucesso`;
    const summaryItems = [];
    if (Number(summary.analyzed_items || 0) > 0) summaryItems.push(`<span><b>${Number(summary.analyzed_items)}</b> verificações</span>`);
    if (Number(summary.applied_adjustments || 0) > 0) summaryItems.push(`<span><b>${Number(summary.applied_adjustments)}</b> ajustes aplicados</span>`);
    if (Number(summary.prioritized_apps || 0) > 0) summaryItems.push(`<span><b>${Number(summary.prioritized_apps)}</b> apps priorizados</span>`);
    if (Number(summary.bottlenecks || 0) > 0) summaryItems.push(`<span><b>${Number(summary.bottlenecks)}</b> pontos de atenção</span>`);
    const memoryDelta = Number(summary.memory_delta_mb || 0);
    if (memoryDelta >= 16) summaryItems.push(`<span><b>+${optimizationMetricValue(memoryDelta, ' MB')}</b> memória disponível</span>`);
    const summaryHtml = summaryItems.length ? `<div class="optimization-result-summary">${summaryItems.join('')}</div>` : '';
    const changedHtml = changed.length ? `<ul>${changed.map((item) => `<li>${CT.esc(item)}</li>`).join('')}</ul>` : '';
    const warningHtml = warnings.length ? `<div class="optimization-result-warnings"><b>Avisos:</b><ul>${warnings.map((item) => `<li>${CT.esc(item)}</li>`).join('')}</ul></div>` : '';
    return `<strong>${CT.esc(title)}</strong>${summaryHtml}${changedHtml}${warningHtml}`;
  }

  function optimizationProfileVisual(profile) {
    const id = Number(profile?.id || 0);
    const visuals = {
      1: {
        tone: 'conservative', kicker: 'PERFIL ECONÔMICO',
        icon: '<svg viewBox="0 0 24 24"><path d="M4 17a8 8 0 1 1 16 0"/><path d="m12 17-3.5-4.5"/><path d="M6.5 10.5 8 12"/><path d="M17.5 10.5 16 12"/><path d="M12 7v2"/></svg>'
      },
      2: {
        tone: 'balanced', kicker: 'USO RECOMENDADO',
        icon: '<svg viewBox="0 0 24 24"><path d="M4 7h5M15 7h5M12 4v6M4 17h9M17 17h3M15 14v6"/></svg>'
      },
      3: {
        tone: 'service', kicker: 'FOCO EM OPERAÇÃO',
        icon: '<svg viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="13" rx="2"/><path d="M8 21h8M12 17v4"/><path d="m15.5 10 1.5 1.5 3-3"/></svg>'
      },
      4: {
        tone: 'performance', kicker: 'MÁXIMO DESEMPENHO',
        icon: '<svg viewBox="0 0 24 24"><rect x="7" y="7" width="10" height="10" rx="1.5"/><path d="M9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3"/></svg>'
      }
    };
    return visuals[id] || { tone: 'default', kicker: 'PERFIL CORECONTROL', icon: '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="8"/></svg>' };
  }

  function optimizationProfilePresentation(profile) {
    const id = Number(profile?.id || 0);
    const presentations = {
      1: {
        ideal: 'Computadores administrativos, tarefas leves e uso em que estabilidade é prioridade.',
        actions: [
          'Reduz animações e efeitos visuais desnecessários',
          'Mantém o plano de energia atual do computador',
          'Preserva a prioridade normal dos programas',
          'Cria backup antes da primeira alteração',
          'Não apaga Downloads, Documentos ou arquivos pessoais',
          'Permite restaurar as configurações anteriores'
        ],
        results: ['Windows mais leve', 'Menos efeitos visuais', 'Baixo impacto']
      },
      2: {
        ideal: 'Rotina diária de trabalho e computadores utilizados durante todo o expediente.',
        actions: [
          'Reduz animações de janelas e menus',
          'Simplifica efeitos visuais que consomem recursos',
          'Ativa o plano de energia Equilibrado',
          'Mantém a prioridade normal dos aplicativos',
          'Cria backup antes da primeira alteração',
          'Permite voltar ao estado anterior com segurança'
        ],
        results: ['Resposta equilibrada', 'Estabilidade', 'Uso diário']
      },
      3: {
        ideal: 'Navegador, WhatsApp, CRM, discador e sistemas usados pela equipe de atendimento.',
        actions: [
          'Reduz animações e efeitos visuais do Windows',
          'Mantém o computador no plano Equilibrado',
          'Identifica aplicativos de atendimento compatíveis abertos',
          'Prioriza moderadamente os aplicativos de trabalho detectados',
          'Salva a prioridade original antes de qualquer mudança',
          'Mantém restauração segura disponível'
        ],
        results: ['Foco no atendimento', 'Apps priorizados', 'Mais responsividade']
      },
      4: {
        ideal: 'Períodos de maior demanda e computadores com carga de trabalho mais intensa.',
        actions: [
          'Reduz animações e efeitos visuais do Windows',
          'Identifica aplicativos de trabalho compatíveis',
          'Prioriza moderadamente os aplicativos importantes abertos',
          'Ativa Alto desempenho quando o computador está na tomada',
          'Mantém Equilibrado quando o computador está na bateria',
          'Preserva backup para restauração do estado anterior'
        ],
        results: ['Maior resposta', 'Apps priorizados', 'Uso intenso']
      }
    };
    const fallbackActions = Array.isArray(profile?.actions) ? profile.actions : [];
    return presentations[id] || {
      ideal: 'Uso geral do computador.',
      actions: fallbackActions,
      results: ['Ajuste seguro', 'Aplicação remota', 'Reversível']
    };
  }

  function optimizationCheckIcon() {
    return '<svg viewBox="0 0 20 20" aria-hidden="true"><path d="m5.5 10.2 2.8 2.8 6.2-6.2"/></svg>';
  }

  function optimizationArrowIcon() {
    return '<svg viewBox="0 0 20 20" aria-hidden="true"><path d="M4 10h11M11 6l4 4-4 4"/></svg>';
  }


  function optimizationMetricValue(value, suffix = '', digits = 0) {
    const number = Number(value);
    if (!Number.isFinite(number)) return '—';
    return `${number.toLocaleString('pt-BR', { minimumFractionDigits: digits, maximumFractionDigits: digits })}${suffix}`;
  }

  function optimizationDiagnosticsBusy(state) {
    return ['queued', 'running'].includes(state?.insight_command?.status);
  }

  function optimizationDiagnosticsIcon(kind) {
    const icons = {
      memory: '<svg viewBox="0 0 24 24"><rect x="4" y="6" width="16" height="12" rx="2"/><path d="M8 10h8M8 14h5M7 3v3M12 3v3M17 3v3M7 18v3M12 18v3M17 18v3"/></svg>',
      disk: '<svg viewBox="0 0 24 24"><ellipse cx="12" cy="6" rx="7" ry="3"/><path d="M5 6v6c0 1.7 3.1 3 7 3s7-1.3 7-3V6M5 12v6c0 1.7 3.1 3 7 3s7-1.3 7-3v-6"/></svg>',
      startup: '<svg viewBox="0 0 24 24"><path d="M5 19V8a3 3 0 0 1 3-3h8a3 3 0 0 1 3 3v11"/><path d="M9 9h6M12 9v6M9.5 12.5 12 15l2.5-2.5"/></svg>',
      processes: '<svg viewBox="0 0 24 24"><rect x="4" y="4" width="6" height="6" rx="1"/><rect x="14" y="4" width="6" height="6" rx="1"/><rect x="4" y="14" width="6" height="6" rx="1"/><rect x="14" y="14" width="6" height="6" rx="1"/></svg>',
      temperature: '<svg viewBox="0 0 24 24"><path d="M10 14.8V5a2 2 0 1 1 4 0v9.8a4 4 0 1 1-4 0Z"/><path d="M12 9v7"/></svg>',
      cleanup: '<svg viewBox="0 0 24 24"><path d="M4 7h16M9 7V4h6v3M7 7l1 13h8l1-13"/><path d="M10 11v5M14 11v5"/></svg>',
    };
    return icons[kind] || icons.processes;
  }

  function optimizationDiagnosticsHTML(device, state) {
    const supported = Boolean(state?.agent_supports_optimization_insights);
    const online = Boolean(state?.online);
    const canManage = optimizationCanManage();
    const busy = optimizationDiagnosticsBusy(state);
    const command = state?.insight_command;
    const diagnostics = state?.diagnostics;

    if (!supported) {
      return `
        <div class="optimization-diagnostics-upgrade">
          <div>
            <span>DIAGNÓSTICO AVANÇADO</span>
            <strong>Mais inteligência disponível no Agent 0.9.1</strong>
            <p>Atualize o Agent para medir memória, inicialização, processos, armazenamento, temperatura, serviços opcionais e gargalos em tempo real.</p>
          </div>
          <span class="optimization-diagnostics-version">0.9.1+</span>
        </div>`;
    }

    if (!diagnostics) {
      const label = busy ? (command?.type === 'optimization.cleanup_temp' ? 'Executando limpeza...' : 'Analisando computador...') : 'Analisar computador';
      return `
        <div class="optimization-diagnostics-empty">
          <div>
            <span class="optimization-diagnostics-eyebrow">DIAGNÓSTICO INTELIGENTE</span>
            <strong>${busy ? 'Análise em andamento' : 'Meça o computador antes de otimizar'}</strong>
            <p>O CoreControl verifica recursos, inicialização, processos, armazenamento e possíveis gargalos antes de recomendar ajustes.</p>
          </div>
          <button class="btn optimization-diagnose-btn" type="button" data-optimization-diagnose ${!canManage || !online || busy ? 'disabled' : ''}>${CT.esc(label)}</button>
        </div>`;
    }

    const memoryAvailableGB = Number(diagnostics.memory_available_mb || 0) / 1024;
    const memoryTotalGB = Number(diagnostics.memory_total_mb || 0) / 1024;
    const temp = diagnostics.temperature_c == null ? 'Indisponível' : `${optimizationMetricValue(diagnostics.temperature_c, ' °C', 1)}`;
    const reclaimable = Number(diagnostics.temp_reclaimable_mb || 0);
    const bottlenecks = Array.isArray(diagnostics.bottlenecks) ? diagnostics.bottlenecks : [];
    const opportunities = Array.isArray(diagnostics.opportunities) ? diagnostics.opportunities : [];
    const checked = Number(diagnostics.checked_items || 0);
    const cleanupDisabled = !canManage || !online || busy || reclaimable < 1;
    const statusText = bottlenecks.length ? `${bottlenecks.length} ponto${bottlenecks.length === 1 ? '' : 's'} de atenção` : 'Nenhum gargalo crítico';

    const metrics = [
      ['memory', 'Memória disponível', memoryTotalGB > 0 ? `${optimizationMetricValue(memoryAvailableGB, ' GB', 1)} de ${optimizationMetricValue(memoryTotalGB, ' GB', 1)}` : '—', diagnostics.memory_available_percent == null ? 'Leitura física do Windows' : `${optimizationMetricValue(diagnostics.memory_available_percent, '%', 0)} disponível`],
      ['disk', 'Armazenamento livre', diagnostics.disk_free_gb ? optimizationMetricValue(diagnostics.disk_free_gb, ' GB', 1) : '—', diagnostics.disk_free_percent == null ? 'Disco principal' : `${optimizationMetricValue(diagnostics.disk_free_percent, '%', 0)} do disco`],
      ['startup', 'Inicialização', optimizationMetricValue(diagnostics.startup_apps, ' itens'), 'Programas configurados para iniciar'],
      ['processes', 'Processos ativos', optimizationMetricValue(diagnostics.active_processes), `${optimizationMetricValue(diagnostics.work_apps)} app(s) de trabalho detectado(s)`],
      ['temperature', 'Temperatura', temp, diagnostics.temperature_c == null ? 'Sensor ACPI não exposto' : 'Leitura ACPI disponível'],
      ['cleanup', 'Potencial de limpeza', reclaimable >= 1 ? optimizationMetricValue(reclaimable, ' MB', 0) : 'Limpo', 'Temporários com mais de 7 dias'],
    ];

    const metricHtml = metrics.map(([kind, label, value, meta]) => `
      <div class="optimization-diagnostic-metric">
        <span class="optimization-diagnostic-icon">${optimizationDiagnosticsIcon(kind)}</span>
        <div><span>${CT.esc(label)}</span><strong>${CT.esc(String(value))}</strong><small>${CT.esc(String(meta))}</small></div>
      </div>`).join('');

    const bottleneckHtml = bottlenecks.length ? `
      <div class="optimization-findings">
        <div class="optimization-findings-title"><strong>Gargalos encontrados</strong><span>${CT.esc(statusText)}</span></div>
        <div class="optimization-findings-list">
          ${bottlenecks.slice(0, 4).map((item) => `
            <div class="optimization-finding level-${CT.esc(item.level || 'low')}">
              <span class="optimization-finding-dot"></span>
              <div><strong>${CT.esc(item.title || 'Ponto de atenção')}</strong><span>${CT.esc(item.detail || '')}</span></div>
            </div>`).join('')}
        </div>
      </div>` : `
      <div class="optimization-findings optimization-findings-clear">
        <div class="optimization-findings-title"><strong>Gargalos encontrados</strong><span>Nenhum gargalo crítico nesta leitura</span></div>
        <p>O computador está dentro dos limites de memória, disco e processamento analisados pelo CoreControl.</p>
      </div>`;

    const opportunitiesHtml = opportunities.length ? `
      <div class="optimization-opportunities">
        <strong>Oportunidades identificadas</strong>
        ${opportunities.slice(0, 4).map((item) => `<span>${optimizationCheckIcon()}${CT.esc(item)}</span>`).join('')}
      </div>` : '';

    return `
      <div class="optimization-diagnostics-head">
        <div>
          <span class="optimization-diagnostics-eyebrow">DIAGNÓSTICO INTELIGENTE</span>
          <strong>Visão técnica antes e depois da otimização</strong>
          <p>${checked || 10} verificações reais do Windows. Serviços opcionais e inicialização são analisados, mas nunca desativados automaticamente.</p>
        </div>
        <div class="optimization-diagnostics-head-actions">
          <span class="optimization-analysis-status">${CT.esc(statusText)}</span>
          <button class="btn optimization-diagnose-btn" type="button" data-optimization-diagnose ${!canManage || !online || busy ? 'disabled' : ''}>${busy ? 'Analisando...' : 'Analisar novamente'}</button>
        </div>
      </div>
      <div class="optimization-diagnostic-metrics">${metricHtml}</div>
      <div class="optimization-diagnostics-bottom">
        ${bottleneckHtml}
        ${opportunitiesHtml}
        <div class="optimization-safe-cleanup">
          <div>
            <strong>Limpeza segura de temporários</strong>
            <span>Remove somente arquivos temporários com mais de 7 dias. Não toca em Downloads, Documentos ou arquivos pessoais.</span>
          </div>
          <button class="btn" type="button" data-optimization-cleanup ${cleanupDisabled ? 'disabled' : ''}>${busy && command?.type === 'optimization.cleanup_temp' ? 'Limpando...' : reclaimable >= 1 ? `Liberar até ${optimizationMetricValue(reclaimable, ' MB', 0)}` : 'Nada para limpar'}</button>
        </div>
      </div>`;
  }

  function renderOptimizationDiagnostics(device, state) {
    const area = CT.$('#optimizationDiagnostics');
    if (!area) return;
    area.innerHTML = optimizationDiagnosticsHTML(device, state);

    const diagnoseButton = area.querySelector('[data-optimization-diagnose]');
    if (diagnoseButton) diagnoseButton.addEventListener('click', () => requestOptimizationDiagnosis(device, state));
    const cleanupButton = area.querySelector('[data-optimization-cleanup]');
    if (cleanupButton) cleanupButton.addEventListener('click', () => requestOptimizationCleanup(device, state));
  }

  async function requestOptimizationDiagnosis(device, currentState, options = {}) {
    if (!optimizationCanManage() || !device?.online) return;
    try {
      const response = await CT.api(`/devices/${device.id}/optimization/diagnose`, { method: 'POST', body: '{}' });
      const nextState = { ...currentState, online: true, agent_supports_optimization_insights: true, insight_command: response.command };
      renderOptimizationDiagnostics(device, nextState);
      if (!options.silent) CT.toast('Diagnóstico enviado para o computador.');
      await pollOptimizationInsights(device, response.command?.id);
    } catch (error) {
      if (!options.silent) CT.toast(error.message || 'Não foi possível executar o diagnóstico.', true);
    }
  }

  async function requestOptimizationCleanup(device, currentState) {
    if (!optimizationCanManage() || !device?.online) return;
    const reclaimable = Number(currentState?.diagnostics?.temp_reclaimable_mb || 0);
    if (!window.confirm(`Executar a limpeza segura neste computador?\n\nO CoreControl removerá somente arquivos das pastas temporárias com mais de 7 dias. Downloads, Documentos e arquivos pessoais não são alterados.${reclaimable > 0 ? `\n\nEstimativa atual: até ${optimizationMetricValue(reclaimable, ' MB', 0)}.` : ''}`)) return;
    try {
      const response = await CT.api(`/devices/${device.id}/optimization/cleanup-temp`, { method: 'POST', body: '{}' });
      renderOptimizationDiagnostics(device, { ...currentState, insight_command: response.command });
      CT.toast('Limpeza segura enviada para o computador.');
      await pollOptimizationInsights(device, response.command?.id, { cleanup: true });
    } catch (error) {
      CT.toast(error.message || 'Não foi possível executar a limpeza segura.', true);
    }
  }

  async function pollOptimizationInsights(device, commandId, options = {}) {
    if (!commandId) return;
    for (let attempt = 0; attempt < 45; attempt += 1) {
      await new Promise((resolve) => setTimeout(resolve, 1000));
      const state = await loadOptimization(device, { autoDiagnose: false });
      const command = state?.insight_command;
      if (!command || command.id !== commandId || !['queued', 'running'].includes(command.status)) {
        if (command?.id === commandId && command.status === 'succeeded') {
          const result = command.result || {};
          if (options.cleanup) {
            CT.toast(`${optimizationMetricValue(result.freed_mb || 0, ' MB', 0)} liberados em ${Number(result.files_deleted || 0)} arquivo(s) temporário(s).`);
          } else {
            CT.toast('Diagnóstico inteligente concluído.');
          }
        } else if (command?.id === commandId && command.status === 'failed') {
          CT.toast(command.error || 'A operação não pôde ser concluída.', true);
        }
        break;
      }
    }
  }

  function renderOptimization(device, state) {
    const status = CT.$('#optimizationStatus');
    const profilesArea = CT.$('#optimizationProfiles');
    const feedback = CT.$('#optimizationFeedback');
    if (!status || !profilesArea || !feedback) return;

    const [label, cssClass] = optimizationStatusLabel(state);
    status.className = `optimization-status ${cssClass}`;
    status.textContent = label;
    renderOptimizationDiagnostics(device, state);

    const canManage = optimizationCanManage();
    const supported = Boolean(state?.agent_supports_optimization);
    const online = Boolean(state?.online);
    const busy = ['queued', 'running'].includes(state?.command?.status);
    const activeName = String(state?.active_profile_name || '').trim();
    const profiles = Array.isArray(state?.profiles) ? state.profiles : [];
    const regularProfiles = profiles.filter((profile) => profile.id >= 1 && profile.id <= 4);
    const restoreProfile = profiles.find((profile) => profile.id === 5);
    const disabled = !canManage || !supported || !online || busy;

    const profileCards = regularProfiles.map((profile) => {
      const active = activeName.toLowerCase() === String(profile.name || '').toLowerCase();
      const visual = optimizationProfileVisual(profile);
      const buttonLabel = active ? 'Aplicar novamente' : `Ativar ${profile.name}`;
      const presentation = optimizationProfilePresentation(profile);
      const actions = presentation.actions;
      return `
        <article class="optimization-profile tone-${visual.tone} ${active ? 'active' : ''}">
          <div class="optimization-profile-top">
            <div class="optimization-profile-intro">
              <div class="optimization-profile-head">
                <div class="optimization-profile-icon">${visual.icon}</div>
                <div class="optimization-profile-heading">
                  <span class="optimization-profile-kicker">${CT.esc(visual.kicker)}</span>
                  <h3>${CT.esc(profile.name)}</h3>
                </div>
                ${active ? '<span class="optimization-profile-badge">ATIVO AGORA</span>' : ''}
              </div>
              <p class="optimization-profile-description">${CT.esc(profile.short || '')}</p>
            </div>
            <div class="optimization-profile-ideal">
              <span>IDEAL PARA</span>
              <p>${CT.esc(presentation.ideal)}</p>
            </div>
          </div>
          <div class="optimization-profile-adjustments">
            <strong>O que o CoreControl vai fazer</strong>
            <div class="optimization-profile-actions">
              ${actions.map((item) => `<div class="optimization-profile-action">${optimizationCheckIcon()}<span>${CT.esc(item)}</span></div>`).join('')}
            </div>
          </div>
          <div class="optimization-profile-results">
            <span class="optimization-profile-results-label">RESULTADO ESPERADO</span>
            <div class="optimization-profile-result-chips">
              ${presentation.results.map((item) => `<span>${CT.esc(item)}</span>`).join('')}
            </div>
          </div>
          <button class="optimization-profile-button ${active ? 'active' : ''}" type="button" data-optimization-profile="${profile.id}" ${disabled ? 'disabled' : ''}>
            <span>${CT.esc(buttonLabel)}</span>${optimizationArrowIcon()}
          </button>
        </article>`;
    }).join('');

    const restoreRow = restoreProfile ? `
      <div class="optimization-restore-row">
        <div class="optimization-restore-icon" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="M3 12a9 9 0 1 0 3-6.7"/><path d="M3 4v5h5"/></svg></div>
        <div class="optimization-restore-copy">
          <span class="optimization-restore-kicker">ROLLBACK SEGURO</span>
          <strong>${CT.esc(restoreProfile.name)}</strong>
          <span>${CT.esc(restoreProfile.short || '')}</span>
        </div>
        <div class="optimization-restore-note"><strong>Backup preservado</strong><span>O CoreControl restaura as configurações salvas antes da primeira otimização.</span></div>
        <button class="btn" type="button" data-optimization-profile="5" ${disabled || !activeName ? 'disabled' : ''}>Restaurar original</button>
      </div>` : '';

    if (!supported) {
      profilesArea.innerHTML = `<div class="optimization-unavailable"><div class="optimization-unavailable-icon">↻</div><div><strong>Atualização do Agent necessária</strong><span>Reinstale/atualize este computador para o CoreControl Agent 0.9.0 ou superior. Depois disso os perfis poderão ser aplicados remotamente pelo painel.</span></div></div>`;
    } else {
      profilesArea.innerHTML = profileCards + restoreRow;
    }

    const feedbackHtml = optimizationFeedbackHTML(state?.command);
    feedback.className = `optimization-feedback${state?.command?.status === 'failed' ? ' error' : state?.command?.status === 'succeeded' ? ' success' : ''}${feedbackHtml ? '' : ' hidden'}`;
    feedback.innerHTML = feedbackHtml;

    profilesArea.querySelectorAll('[data-optimization-profile]').forEach((button) => {
      button.addEventListener('click', () => {
        const profileId = Number(button.getAttribute('data-optimization-profile'));
        const profile = profiles.find((item) => Number(item.id) === profileId);
        requestOptimization(device, profile, state);
      });
    });
  }

  async function loadOptimization(device, options = {}) {
    try {
      const state = await CT.api(`/devices/${device.id}/optimization`);
      renderOptimization(device, state);
      const shouldAutoDiagnose = options.autoDiagnose !== false
        && optimizationCanManage()
        && state?.online
        && state?.agent_supports_optimization_insights
        && !state?.diagnostics
        && !optimizationDiagnosticsBusy(state)
        && !optimizationAutoDiagnoseStarted.has(device.id);
      if (shouldAutoDiagnose) {
        optimizationAutoDiagnoseStarted.add(device.id);
        window.setTimeout(() => requestOptimizationDiagnosis(device, state, { silent: true }), 150);
      }
      return state;
    } catch (error) {
      const status = CT.$('#optimizationStatus');
      const area = CT.$('#optimizationProfiles');
      const diagnosticsArea = CT.$('#optimizationDiagnostics');
      if (status) {
        status.className = 'optimization-status bad';
        status.textContent = 'Falha ao carregar';
      }
      if (area) area.innerHTML = `<div class="optimization-unavailable"><strong>Otimização indisponível</strong><span>${CT.esc(error.message || 'Não foi possível carregar os perfis.')}</span></div>`;
      if (diagnosticsArea) diagnosticsArea.innerHTML = '<div class="optimization-diagnostics-loading">Diagnóstico indisponível no momento.</div>';
      return null;
    }
  }

  async function requestOptimization(device, profile, currentState) {
    if (!profile || !optimizationCanManage()) return;
    const restore = Number(profile.id) === 5;
    const message = restore
      ? 'Desativar a otimização e restaurar as configurações salvas antes da primeira aplicação?'
      : `Aplicar o perfil “${profile.name}” neste computador?\n\nO CoreControl cria/preserva um backup automático antes de alterar o Windows.`;
    if (!window.confirm(message)) return;

    try {
      const response = await CT.api(`/devices/${device.id}/optimization`, {
        method: 'POST',
        body: JSON.stringify({ profile: Number(profile.id) }),
      });
      renderOptimization(device, { ...currentState, command: response.command, online: true, agent_supports_optimization: true });
      CT.toast(restore ? 'Restauração enviada para o computador.' : `Perfil ${profile.name} enviado para o computador.`);

      const commandId = response.command?.id;
      for (let attempt = 0; attempt < 35; attempt += 1) {
        await new Promise((resolve) => setTimeout(resolve, 1000));
        const state = await loadOptimization(device);
        const command = state?.command;
        if (!command || command.id !== commandId || !['queued', 'running'].includes(command.status)) {
          if (command?.id === commandId && command.status === 'succeeded') {
            const result = command.result || {};
            device.profile = result.active_profile_name || 'Nenhum';
            CT.toast(result.restored ? 'Configurações anteriores restauradas.' : `${result.profile_name || profile.name} aplicado com sucesso.`);
          }
          break;
        }
      }
    } catch (error) {
      CT.toast(error.message || 'Não foi possível aplicar a otimização.', true);
      await loadOptimization(device);
    }
  }

  CT.registerPage('device', async function renderDevice() {
    const device = await CT.api(`/devices/${CT.state.selectedDevice}`);
    CT.state.selectedDevice = device.id;
    activityLastDevice = device;
    activityLastSuccessfulCommand = null;
    activityAssets.clear();
    await CT.mountPage('device');

    CT.$('#pageTitle').textContent = device.name;
    const telemetry = device.telemetry || {};

    CT.$('#deviceOnlineStatus').innerHTML = `<i class="dot ${device.online ? 'online' : 'offline'}"></i>${device.online ? 'Online' : 'Offline'}`;
    CT.$('#deviceHealthStatus').className = `health ${CT.healthClass(device.health_score)}`;
    CT.$('#deviceHealthStatus').textContent = `Saúde ${device.health_score}/100`;

    CT.$('#deviceMetrics').innerHTML = [
      CT.metric('Processador', telemetry.cpu_percent, '%', telemetry.cpu_percent),
      CT.metric('Memória RAM', telemetry.memory_percent, '%', telemetry.memory_percent),
      CT.metric('Disco principal', telemetry.disk_percent, '%', telemetry.disk_percent),
      CT.metric(
        'Temperatura',
        telemetry.temperature_c,
        ' °C',
        telemetry.temperature_c ? Math.min(100, telemetry.temperature_c) : 0,
        telemetry.temperature_c >= 85 ? 'critical' : '',
      ),
    ].join('');

    loadOptimization(device);

    renderActivityCurrent(device);
    renderActivityTimeline(device.history);
    const activityButton = CT.$('#activityRefreshBtn');
    activityButton.disabled = !device.online || !activityVersionSupported(device.agent_version);
    activityButton.title = !activityVersionSupported(device.agent_version) ? 'Atualize o CoreControl Agent para 0.6.0 ou superior.' : !device.online ? 'O computador precisa estar online.' : '';
    activityButton.onclick = () => requestActivitySnapshot(device);
    loadActivitySnapshot(device.id).then(() => {
      if (!device.online || !activityVersionSupported(device.agent_version)) return;
      startActivityAutoRefresh(device);
    });

    CT.$('#deviceHistoryCount').textContent = `Últimas ${device.history.length} amostras recebidas.`;
    CT.$('#deviceCompanyName').textContent = device.company_name;
    CT.$('#deviceIdentity').innerHTML = [
      CT.info('Fabricante', device.manufacturer),
      CT.info('Modelo', device.model),
      CT.info('Número de série', device.serial_number),
      CT.info('Windows', `${device.os_name || ''} ${device.os_version || ''}`.trim()),
      CT.info('IP local', telemetry.ip_local),
      CT.info('Último contato', CT.fmtDate(device.last_seen)),
      CT.info('Agente', device.agent_version),
      CT.info('Perfil aplicado', device.profile || 'Nenhum'),
    ].join('');

    CT.$('#deviceProtection').innerHTML = [
      CT.info('Memória instalada', telemetry.memory_total_gb == null ? '—' : `${CT.fmtNum(telemetry.memory_total_gb, 1)} GB`),
      CT.info('Memória usada', telemetry.memory_used_gb == null ? '—' : `${CT.fmtNum(telemetry.memory_used_gb, 1)} GB`),
      CT.info('Disco total', telemetry.disk_total_gb == null ? '—' : `${CT.fmtNum(telemetry.disk_total_gb, 1)} GB`),
      CT.info('Espaço livre', telemetry.disk_free_gb == null ? '—' : `${CT.fmtNum(telemetry.disk_free_gb, 1)} GB`),
      CT.info('Microsoft Defender', telemetry.defender_active == null ? 'Não informado' : telemetry.defender_active ? 'Ativo' : 'Desativado'),
      CT.info('Firewall', telemetry.firewall_active == null ? 'Não informado' : telemetry.firewall_active ? 'Ativo' : 'Desativado'),
    ].join('');

    CT.$('#deviceRemoteLabel').innerHTML = CT.remoteLabel(device);
    CT.$('#deviceRemoteText').textContent = device.remote?.running
      ? 'O agente remoto está conectado e pronto para suporte.'
      : device.remote?.installed
        ? 'O módulo está instalado, mas não está conectado.'
        : 'Instale novamente pelo CoreControl Setup autorizando o acesso remoto.';

    const remoteButton = CT.$('#remoteAccessBtn');
    remoteButton.disabled = !device.remote?.available;
    remoteButton.addEventListener('click', () => CT.openRemoteSession(device.id));
    CT.$('#backDevices').onclick = () => CT.navigate('devices');
    const reinstallDeviceBtn = CT.$('#reinstallDeviceBtn');
    if (['global_admin', 'platform_admin', 'company_admin', 'technician'].includes(CT.state.user.role)) {
      reinstallDeviceBtn.classList.remove('hidden');
      reinstallDeviceBtn.onclick = () => CT.openDeviceReinstallOptions(device);
    }

    const editDeviceBtn = CT.$('#editDeviceBtn');
    if (['global_admin', 'platform_admin', 'company_admin'].includes(CT.state.user.role)) {
      editDeviceBtn.classList.remove('hidden');
      editDeviceBtn.onclick = () => CT.openDeviceEditModal(device);
    }
    CT.drawChart(CT.$('#telemetryChart'), device.history);
  });
})();

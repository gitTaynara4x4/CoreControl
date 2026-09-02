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
    const cached = snapshot?.cached_command;
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
      const area = CT.$('#activityAppsArea');
      if (area) area.innerHTML = `<div class="activity-empty">${CT.esc(error.message || 'Falha ao carregar atividade.')}</div>`;
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
      return '<strong>Otimização em andamento</strong>O CoreControl Agent recebeu a solicitação e está aplicando o perfil com backup automático.';
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
    const title = result.restored ? 'Configurações anteriores restauradas' : `${result.profile_name || 'Perfil'} aplicado com sucesso`;
    const changedHtml = changed.length ? `<ul>${changed.map((item) => `<li>${CT.esc(item)}</li>`).join('')}</ul>` : '';
    const warningHtml = warnings.length ? `<div style="margin-top:7px"><b>Avisos:</b><ul>${warnings.map((item) => `<li>${CT.esc(item)}</li>`).join('')}</ul></div>` : '';
    return `<strong>${CT.esc(title)}</strong>${changedHtml}${warningHtml}`;
  }

  function renderOptimization(device, state) {
    const status = CT.$('#optimizationStatus');
    const profilesArea = CT.$('#optimizationProfiles');
    const feedback = CT.$('#optimizationFeedback');
    if (!status || !profilesArea || !feedback) return;

    const [label, cssClass] = optimizationStatusLabel(state);
    status.className = `optimization-status ${cssClass}`;
    status.textContent = label;

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
      const buttonLabel = active ? 'Aplicar novamente' : 'Aplicar perfil';
      return `
        <article class="optimization-profile ${active ? 'active' : ''}">
          <div class="optimization-profile-head">
            <h3>${CT.esc(profile.name)}</h3>
            ${active ? '<span class="optimization-profile-badge">ATIVO</span>' : ''}
          </div>
          <p>${CT.esc(profile.short || '')}</p>
          <ul>${(profile.actions || []).map((item) => `<li>${CT.esc(item)}</li>`).join('')}</ul>
          <button class="btn ${active ? '' : 'primary'}" type="button" data-optimization-profile="${profile.id}" ${disabled ? 'disabled' : ''}>${buttonLabel}</button>
        </article>`;
    }).join('');

    const restoreRow = restoreProfile ? `
      <div class="optimization-restore-row">
        <div>
          <strong>${CT.esc(restoreProfile.name)}</strong>
          <span>${CT.esc(restoreProfile.short || '')}</span>
        </div>
        <button class="btn" type="button" data-optimization-profile="5" ${disabled || !activeName ? 'disabled' : ''}>Restaurar original</button>
      </div>` : '';

    if (!supported) {
      profilesArea.innerHTML = `<div class="optimization-unavailable"><strong>Atualização do Agent necessária</strong><span>Reinstale/atualize este computador para o CoreControl Agent 0.9.0 ou superior. Depois disso a Rosiane poderá aplicar os perfis diretamente daqui.</span></div>`;
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

  async function loadOptimization(device) {
    try {
      const state = await CT.api(`/devices/${device.id}/optimization`);
      renderOptimization(device, state);
      return state;
    } catch (error) {
      const status = CT.$('#optimizationStatus');
      const area = CT.$('#optimizationProfiles');
      if (status) {
        status.className = 'optimization-status bad';
        status.textContent = 'Falha ao carregar';
      }
      if (area) area.innerHTML = `<div class="optimization-unavailable"><strong>Otimização indisponível</strong><span>${CT.esc(error.message || 'Não foi possível carregar os perfis.')}</span></div>`;
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

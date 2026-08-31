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
    if (key === 'chrome' || key === 'google chrome') return 'chrome';
    if (key === 'edge' || key === 'msedge' || key === 'microsoft edge') return 'msedge';
    if (key === 'opera' || key === 'opera gx' || key === 'opera_gx') return key === 'opera' ? 'opera' : 'opera_gx';
    if (key === 'brave' || key === 'brave-browser') return 'brave';
    return key || 'browser';
  }

  function activityBrowserName(browser) {
    return activityFriendlyName(activityBrowserProcess(browser));
  }

  function activityBrowserKey(value) {
    const key = activityProcessKey(value);
    if (key === 'msedge' || key === 'edge') return 'edge';
    if (key === 'opera' || key === 'opera_gx') return 'opera';
    if (key === 'brave' || key === 'brave-browser') return 'brave';
    if (key === 'chrome' || key === 'google chrome') return 'chrome';
    return key;
  }

  function activityGroupedApplications(apps, browserTabs) {
    const groups = new Map();
    (apps || []).forEach((app, index) => {
      const processKey = activityProcessKey(app?.process_name) || `pid-${app?.pid || index}`;
      if (!groups.has(processKey)) {
        groups.set(processKey, {
          key: processKey,
          process_name: app?.process_name || processKey,
          apps: [],
          tabs: [],
          order: index,
        });
      }
      groups.get(processKey).apps.push(app);
    });

    (browserTabs || []).forEach((tab, index) => {
      const browserKey = activityBrowserKey(tab?.browser);
      if (!browserKey) return;
      let group = Array.from(groups.values()).find((candidate) => activityBrowserKey(candidate.process_name) === browserKey);
      if (!group) {
        const processName = activityBrowserProcess(tab.browser);
        const processKey = activityProcessKey(processName) || `${browserKey}-browser`;
        group = { key: processKey, process_name: processName, apps: [], tabs: [], order: (apps || []).length + index };
        groups.set(processKey, group);
      }
      group.tabs.push(tab);
    });

    return Array.from(groups.values()).sort((left, right) => {
      const leftFocused = left.apps.some((app) => app?.focused) || left.tabs.some((tab) => tab?.active);
      const rightFocused = right.apps.some((app) => app?.focused) || right.tabs.some((tab) => tab?.active);
      if (leftFocused !== rightFocused) return leftFocused ? -1 : 1;
      return left.order - right.order;
    });
  }

  function activityUniqueProcessMetric(apps, field) {
    const seen = new Set();
    let total = 0;
    (apps || []).forEach((app, index) => {
      const pid = Number(app?.pid) || 0;
      const key = pid > 0 ? `pid:${pid}` : `row:${index}`;
      if (seen.has(key)) return;
      seen.add(key);
      total += Number(app?.[field]) || 0;
    });
    return total;
  }

  function activityGroupChildren(group) {
    if (group.tabs.length) {
      return group.tabs.map((tab) => ({ type: 'tab', tab }));
    }
    if (group.apps.length <= 1) return [];
    let rows = group.apps;
    if (activityProcessKey(group.process_name) === 'explorer') {
      const specific = rows.filter((app) => {
        const title = String(app?.window_title || '').trim().toLowerCase();
        return title && title !== 'explorador de arquivos' && title !== 'file explorer';
      });
      if (specific.length) rows = specific;
    }
    return rows.map((app) => ({ type: 'window', app }));
  }

  function activityRenderGroupRows(group, index) {
    const processKey = activityProcessKey(group.process_name);
    const asset = activityAssets.get(processKey);
    const leadApp = group.apps.find((app) => app?.focused) || group.apps[0] || {};
    const activeTab = group.tabs.find((tab) => tab?.active);
    const displayName = activityFriendlyName(group.process_name, leadApp.display_name || asset?.display_name);
    const focused = Boolean(leadApp.focused || activeTab);
    const children = activityGroupChildren(group);
    const expanded = children.length > 0 && activityExpandedGroups.has(group.key);
    const countText = children.length > 0 ? ` <span class="activity-group-number">(${children.length})</span>` : '';
    const toggle = children.length > 0
      ? `<button type="button" class="activity-group-toggle ${expanded ? 'expanded' : ''}" data-activity-toggle="${CT.esc(group.key)}" aria-expanded="${expanded ? 'true' : 'false'}" aria-label="${expanded ? 'Recolher' : 'Expandir'} ${CT.esc(displayName)}"><span aria-hidden="true">›</span></button>`
      : '<span class="activity-group-toggle-spacer" aria-hidden="true"></span>';
    const title = activeTab?.title || leadApp.window_title || (children.length ? `${children.length} itens abertos` : '—');
    const cpu = activityUniqueProcessMetric(group.apps, 'cpu_percent');
    const memory = activityUniqueProcessMetric(group.apps, 'memory_mb');
    const parent = `<tr class="activity-group-parent" data-activity-parent="${CT.esc(group.key)}"><td><div class="activity-app-name activity-group-app">${toggle}${activityAppIcon(group.process_name, focused)}<span class="activity-group-label" title="${CT.esc(displayName)}">${CT.esc(displayName)}${countText}</span></div></td><td><div class="activity-window-title" title="${CT.esc(title)}">${CT.esc(title)}</div></td><td>${CT.fmtNum(cpu, 1)}%</td><td>${CT.fmtNum(memory, 0)} MB</td><td>${focused ? '<span class="pill resolved">Em uso</span>' : '<span class="pill">Aberto</span>'}</td></tr>`;

    const childRows = children.map((child, childIndex) => {
      const hidden = expanded ? '' : ' hidden';
      if (child.type === 'tab') {
        const tab = child.tab || {};
        const pageTitle = String(tab.title || 'Página').trim() || 'Página';
        const site = String(tab.domain || '').trim() || (tab.url ? String(tab.url).trim() : 'Aba do navegador');
        return `<tr class="activity-tree-child" data-activity-child="${CT.esc(group.key)}"${hidden}><td><div class="activity-child-app"><span class="activity-tree-branch" aria-hidden="true"></span>${activityAppIcon(group.process_name, Boolean(tab.active), 'activity-child-icon')}<span title="${CT.esc(pageTitle)}">${CT.esc(pageTitle)}</span></div></td><td><div class="activity-child-context" title="${CT.esc(tab.url || site)}">${CT.esc(site)}</div></td><td class="activity-child-metric">—</td><td class="activity-child-metric">—</td><td>${tab.active ? '<span class="pill resolved">Em uso</span>' : '<span class="pill">Aba aberta</span>'}</td></tr>`;
      }
      const app = child.app || {};
      const windowTitle = String(app.window_title || `Janela ${childIndex + 1}`).trim();
      const childLabel = processKey === 'explorer' ? windowTitle : windowTitle;
      return `<tr class="activity-tree-child" data-activity-child="${CT.esc(group.key)}"${hidden}><td><div class="activity-child-app"><span class="activity-tree-branch" aria-hidden="true"></span>${activityAppIcon(group.process_name, Boolean(app.focused), 'activity-child-icon')}<span title="${CT.esc(childLabel)}">${CT.esc(childLabel)}</span></div></td><td><div class="activity-child-context">${processKey === 'explorer' ? 'Pasta aberta' : 'Janela aberta'}</div></td><td>${CT.fmtNum(app.cpu_percent, 1)}%</td><td>${CT.fmtNum(app.memory_mb, 0)} MB</td><td>${app.focused ? '<span class="pill resolved">Em uso</span>' : '<span class="pill">Aberto</span>'}</td></tr>`;
    }).join('');
    return parent + childRows;
  }

  function activityBindGroupToggles(area) {
    area.querySelectorAll('[data-activity-toggle]').forEach((button) => {
      button.addEventListener('click', () => {
        const key = button.dataset.activityToggle || '';
        if (!key) return;
        const expanded = !activityExpandedGroups.has(key);
        if (expanded) activityExpandedGroups.add(key);
        else activityExpandedGroups.delete(key);
        button.classList.toggle('expanded', expanded);
        button.setAttribute('aria-expanded', expanded ? 'true' : 'false');
        button.setAttribute('aria-label', `${expanded ? 'Recolher' : 'Expandir'} grupo`);
        area.querySelectorAll('[data-activity-child]').forEach((row) => {
          if (row.dataset.activityChild === key) row.hidden = !expanded;
        });
      });
    });
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

  function renderActivitySnapshot(snapshot) {
    const status = CT.$('#activitySnapshotStatus');
    const area = CT.$('#activityAppsArea');
    if (!status || !area) return;
    const command = snapshot?.command;
    if (!snapshot?.agent_supports_activity) {
      status.textContent = 'Agent antigo';
      area.innerHTML = '<div class="activity-empty">Execute novamente o CoreControl Setup para instalar o Agent 0.6.0 ou superior.</div>';
      return;
    }
    if (!command) {
      status.textContent = 'Ainda não consultado';
      area.innerHTML = '<div class="activity-empty">Clique em “Atualizar aplicativos” para consultar as janelas abertas neste computador.</div>';
      return;
    }
    if (['queued', 'running'].includes(command.status)) {
      status.textContent = command.status === 'running' ? 'Coletando...' : 'Na fila...';
      area.innerHTML = '<div class="activity-empty">O Agent vai devolver a lista na próxima comunicação com a Central.</div>';
      return;
    }
    if (command.status === 'failed') {
      status.textContent = 'Falhou';
      area.innerHTML = `<div class="activity-empty">${CT.esc(command.error || 'Não foi possível consultar os aplicativos.')}</div>`;
      return;
    }
    const apps = command.result?.apps || [];
    const browserTabs = command.result?.browser_tabs || [];
    activityRememberAssets(command.result || {});
    const assetsList = Object.values(command.result?.app_assets || {});
    const iconCount = assetsList.filter((asset) => Boolean(activityIconData(asset?.icon_data))).length;
    const appRowsWithIcon = apps.filter((app) => Boolean(activityIconData(activityAssets.get(activityProcessKey(app?.process_name))?.icon_data))).length;
    const realIcons = iconCount > 0;
    const tabSuffix = browserTabs.length ? ` · ${browserTabs.length} aba${browserTabs.length === 1 ? '' : 's'}` : '';
    status.textContent = !realIcons && (apps.length || browserTabs.length)
      ? `Atualizado · 0 ícones recebidos${tabSuffix}`
      : realIcons && apps.length ? `Atualizado · ${appRowsWithIcon}/${apps.length} janelas com ícone${tabSuffix}`
      : realIcons ? `Atualizado · ${iconCount} ícone(s) real(is)${tabSuffix}`
      : command.finished_at ? `Atualizado ${CT.fmtDate(command.finished_at)}${tabSuffix}` : `Atualizado${tabSuffix}`;
    const groups = activityGroupedApplications(apps, browserTabs);
    const appsHtml = groups.length ? `<div class="activity-browser-title activity-grouped-title">Aplicativos com janela aberta <span class="activity-count">${groups.length}</span><small>Clique na seta para ver todas as abas e janelas agrupadas.</small></div><div class="table-wrap activity-table-scroll"><table class="activity-table activity-grouped-table"><thead><tr><th>Aplicativo</th><th>Janela</th><th>CPU</th><th>Memória</th><th>Status</th></tr></thead><tbody>${groups.map((group, index) => activityRenderGroupRows(group, index)).join('')}</tbody></table></div>` : '';
    area.innerHTML = appsHtml || '<div class="activity-empty">Nenhuma atividade visível foi encontrada.</div>';
    activityBindGroupToggles(area);
    if (activityLastDevice) {
      renderActivityCurrent(activityLastDevice);
      renderActivityTimeline(activityLastDevice.history);
    }
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

  async function requestActivitySnapshot(device) {
    const button = CT.$('#activityRefreshBtn');
    if (!button || button.disabled) return;
    button.disabled = true;
    button.textContent = 'Solicitando...';
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
      CT.toast(error.message || 'Não foi possível consultar a atividade.', 'error');
    } finally {
      button.disabled = !device.online || !activityVersionSupported(device.agent_version);
      button.textContent = 'Atualizar aplicativos';
    }
  }

  CT.registerPage('device', async function renderDevice() {
    const device = await CT.api(`/devices/${CT.state.selectedDevice}`);
    CT.state.selectedDevice = device.id;
    activityLastDevice = device;
    activityAssets.clear();
    activityExpandedGroups.clear();
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

    renderActivityCurrent(device);
    renderActivityTimeline(device.history);
    const activityButton = CT.$('#activityRefreshBtn');
    activityButton.disabled = !device.online || !activityVersionSupported(device.agent_version);
    activityButton.title = !activityVersionSupported(device.agent_version) ? 'Atualize o CoreControl Agent para 0.6.0 ou superior.' : !device.online ? 'O computador precisa estar online.' : '';
    activityButton.onclick = () => requestActivitySnapshot(device);
    loadActivitySnapshot(device.id).then((snapshot) => {
      const result = snapshot?.command?.result || {};
      const apps = result.apps || [];
      const browserTabs = result.browser_tabs || [];
      const hasApps = apps.length > 0 || browserTabs.length > 0;
      const missingIcons = hasApps && !activityResultHasRealIcons(result);
      const hasBrowserWindow = apps.some((app) => ['chrome', 'msedge', 'opera', 'opera_gx', 'brave'].includes(activityProcessKey(app?.process_name)));
      const missingBrowserTabs = activityBrowserTabsVersionSupported(device.agent_version) && hasBrowserWindow && browserTabs.length === 0;
      // Depois de atualizar o Agent, um snapshot antigo pode continuar salvo no
      // servidor sem ícones/abas. Faz uma nova coleta automaticamente uma vez.
      if (device.online && snapshot?.command?.status === 'succeeded' && ((activityIconVersionSupported(device.agent_version) && missingIcons) || missingBrowserTabs)) {
        requestActivitySnapshot(device);
      }
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

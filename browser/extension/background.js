const HOST = 'com.corecontrol.browser';
const SNAPSHOT_ALARM = 'corecontrol-browser-snapshot';
let sendTimer = null;

function browserName() {
  const ua = navigator.userAgent || '';
  if (/OPR\//i.test(ua)) return 'opera';
  if (/Edg\//i.test(ua)) return 'edge';
  return 'chrome';
}

function safeUrl(raw) {
  try {
    const url = new URL(raw || '');
    if (!['http:', 'https:'].includes(url.protocol)) return null;
    return {
      url: `${url.origin}${url.pathname}`,
      domain: url.hostname.replace(/^www\./i, '')
    };
  } catch (_) {
    return null;
  }
}

async function collectTabs() {
  const tabs = await chrome.tabs.query({});
  let focusedWindowId = null;
  try {
    const focused = await chrome.windows.getLastFocused({populate: false});
    if (focused?.focused) focusedWindowId = focused.id;
  } catch (_) {}
  return tabs
    .filter(tab => !tab.incognito)
    .map(tab => {
      const parsed = safeUrl(tab.url);
      if (!parsed) return null;
      return {
        tab_id: tab.id,
        window_id: tab.windowId,
        title: (tab.title || parsed.domain || 'Página').trim(),
        url: parsed.url,
        domain: parsed.domain,
        fav_icon_url: (tab.favIconUrl || '').trim(),
        active: Boolean(tab.active && tab.windowId === focusedWindowId),
        audible: Boolean(tab.audible),
        pinned: Boolean(tab.pinned),
        discarded: Boolean(tab.discarded)
      };
    })
    .filter(Boolean);
}

async function sendSnapshot() {
  try {
    const tabs = await collectTabs();
    const message = {
      type: 'tabs.snapshot',
      version: 1,
      browser: browserName(),
      captured_at: new Date().toISOString(),
      tabs
    };
    const response = await chrome.runtime.sendNativeMessage(HOST, message);
    await chrome.storage.local.set({
      corecontrol_status: response?.ok ? 'connected' : 'error',
      corecontrol_last_sync: new Date().toISOString(),
      corecontrol_last_error: response?.error || ''
    });
    return response;
  } catch (error) {
    await chrome.storage.local.set({
      corecontrol_status: 'error',
      corecontrol_last_sync: new Date().toISOString(),
      corecontrol_last_error: String(error?.message || error)
    });
    return {ok: false, error: String(error?.message || error)};
  }
}

function scheduleSnapshot(delay = 350) {
  clearTimeout(sendTimer);
  sendTimer = setTimeout(() => sendSnapshot(), delay);
}

chrome.runtime.onInstalled.addListener(() => {
  chrome.alarms.create(SNAPSHOT_ALARM, {periodInMinutes: 0.5});
  scheduleSnapshot(100);
});
chrome.runtime.onStartup.addListener(() => {
  chrome.alarms.create(SNAPSHOT_ALARM, {periodInMinutes: 0.5});
  scheduleSnapshot(100);
});
chrome.alarms.onAlarm.addListener(alarm => {
  if (alarm.name === SNAPSHOT_ALARM) sendSnapshot();
});
chrome.tabs.onActivated.addListener(() => scheduleSnapshot(100));
chrome.tabs.onCreated.addListener(() => scheduleSnapshot());
chrome.tabs.onRemoved.addListener(() => scheduleSnapshot());
chrome.tabs.onUpdated.addListener((_tabId, changeInfo) => {
  if (changeInfo.title !== undefined || changeInfo.url !== undefined || changeInfo.status === 'complete') {
    scheduleSnapshot();
  }
});
chrome.windows.onFocusChanged.addListener(() => scheduleSnapshot(100));
chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (message?.type === 'sync-now') {
    sendSnapshot().then(sendResponse);
    return true;
  }
});

// Garante uma primeira tentativa mesmo quando a extensao e carregada por --load-extension.
scheduleSnapshot(750);

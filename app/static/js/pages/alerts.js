(function () {
  'use strict';

  const CT = window.CoreTuner;

  CT.registerPage('alerts', async function renderAlerts() {
    const alerts = await CT.api('/alerts?status_filter=active');
    await CT.mountPage('alerts');
    CT.setAlertBadge(alerts.length);

    CT.$('#alertsArea').innerHTML = alerts.length
      ? `<div class="table-wrap"><table><thead><tr><th>Gravidade</th><th>Computador</th><th>Alerta</th><th>Detectado</th><th>Status</th><th></th></tr></thead><tbody>${alerts.map((alert) => `<tr><td><span class="pill ${CT.esc(alert.severity)}">${alert.severity === 'critical' ? 'Crítico' : 'Atenção'}</span></td><td><strong>${CT.esc(alert.device_name)}</strong></td><td><strong>${CT.esc(alert.title)}</strong><small style="display:block;color:var(--muted);margin-top:4px">${CT.esc(alert.message)}</small></td><td>${CT.fmtDate(alert.opened_at)}</td><td><span class="pill ${CT.esc(alert.status)}">${alert.status === 'acknowledged' ? 'Reconhecido' : 'Aberto'}</span></td><td>${alert.status === 'open' ? `<button class="btn small" data-ack="${alert.id}">Reconhecer</button>` : '—'}</td></tr>`).join('')}</tbody></table></div>`
      : '<div class="empty">Nenhum alerta ativo.</div>';

    CT.$$('[data-ack]').forEach((button) => {
      button.onclick = async () => {
        try {
          await CT.api(`/alerts/${button.dataset.ack}/ack`, { method: 'POST' });
          CT.toast('Alerta reconhecido.');
          CT.renderCurrent(true);
        } catch (error) {
          CT.toast(error.message, true);
        }
      };
    });
  });
})();

(function () {
  'use strict';
  const CT = window.CoreTuner;

  const groups = [
    ['Computadores', 'Inventário, hardware, software e saúde', ['Inventário', 'Hardware', 'Software', 'Saúde']],
    ['Monitoramento', 'Disponibilidade, alertas e eventos', ['Disponibilidade', 'Alertas', 'Eventos']],
    ['Administração', 'Empresas, usuários e acessos', ['Empresas', 'Usuários', 'Acessos']],
    ['Operações', 'Acesso remoto, scripts e atualizações', ['Acesso remoto', 'Scripts', 'Atualizações', 'Otimizações']],
  ];

  function center() {
    return `<div class="report-grid">${groups.map(([name, description, items]) => `<article class="report-card"><div class="report-card-head"><div><strong>${name}</strong><p>${description}</p></div></div><div class="report-links">${items.map((item) => `<button type="button" data-report="${item}"><span>${item}</span><b>→</b></button>`).join('')}</div></article>`).join('')}</div>`;
  }

  function audit() {
    const allowed = CT.state.user?.role === 'global_admin';
    return `<div class="card module-section"><div class="card-header"><div><h2>Auditoria</h2><p>Registro administrativo de ações críticas no CoreControl.</p></div>${allowed ? '<button class="btn" data-report="Exportar auditoria">Exportar</button>' : ''}</div>${allowed ? '<div class="table-wrap"><table><thead><tr><th>Data</th><th>Usuário</th><th>Ação</th><th>Empresa</th><th>Alvo</th><th>Origem</th></tr></thead><tbody><tr><td colspan="6" class="table-empty-cell">A visualização consolidada dos logs será conectada ao backend nesta etapa do módulo.</td></tr></tbody></table></div>' : '<div class="module-empty compact"><strong>Acesso restrito</strong><span>A auditoria global é exclusiva do Administrador Global.</span></div>'}</div>`;
  }

  function exports() {
    return `<div class="module-layout-2"><div class="card module-section"><div class="card-header"><div><h2>Gerar relatório</h2><p>Escolha o tipo, empresa, período e formato.</p></div></div><div class="module-form"><label>Relatório<select><option>Computadores</option><option>Alertas</option><option>Acesso remoto</option><option>Scripts</option><option>Atualizações</option></select></label><label>Formato<select><option>PDF</option><option>Excel</option><option>CSV</option></select></label><button class="btn primary" data-report="Gerar relatório">Gerar relatório</button></div></div><div class="card module-section"><div class="card-header"><div><h2>Exportações recentes</h2><p>Arquivos gerados pela sua conta.</p></div></div><div class="module-empty compact"><strong>Nenhuma exportação</strong><span>Os relatórios gerados aparecerão aqui.</span></div></div></div>`;
  }

  CT.registerPage('reports', async function renderReports() {
    await CT.mountPage('reports');
    const page = CT.$('.page-reports');
    const view = CT.$('#reportsView');
    const render = (tab) => {
      CT.$$('[data-module-tab]', page).forEach((button) => button.classList.toggle('active', button.dataset.moduleTab === tab));
      view.innerHTML = tab === 'audit' ? audit() : tab === 'exports' ? exports() : center();
    };
    page.addEventListener('click', (event) => {
      const tab = event.target.closest('[data-module-tab]');
      if (tab) return render(tab.dataset.moduleTab);
      const report = event.target.closest('[data-report]');
      if (report) CT.toast(`${report.dataset.report}: estrutura pronta para ligar os dados e a exportação.`);
    });
    render('center');
  });
})();

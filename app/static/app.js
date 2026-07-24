const state = { user:null, page:'overview', refreshTimer:null, selectedCompany:null, selectedDevice:null };
const $ = (s, root=document) => root.querySelector(s);
const $$ = (s, root=document) => [...root.querySelectorAll(s)];

function esc(value=''){ return String(value ?? '').replace(/[&<>'"]/g, c=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c])); }
function fmtDate(value){ if(!value) return '—'; const d=new Date(value); return Number.isNaN(d.getTime())?'—':d.toLocaleString('pt-BR'); }
function fmtNum(value, digits=0){ return value==null?'—':Number(value).toLocaleString('pt-BR',{maximumFractionDigits:digits,minimumFractionDigits:digits}); }
function roleName(role){ return ({platform_admin:'Administrador da plataforma',company_admin:'Administrador da empresa',technician:'Técnico',viewer:'Visualização'})[role]||role; }
function healthClass(score){ return score>=80?'good':score>=55?'warn':'bad'; }
function toast(msg, error=false){ const el=$('#toast'); el.textContent=msg; el.className='toast show'+(error?' error':''); clearTimeout(el._t); el._t=setTimeout(()=>el.className='toast',3200); }

async function api(path, options={}){
  const opts={credentials:'same-origin',headers:{...(options.body?{'Content-Type':'application/json'}:{}),...(options.headers||{})},...options};
  const res=await fetch('/api'+path,opts);
  let data=null; try{ data=await res.json(); }catch{}
  if(res.status===401 && !['/auth/login','/auth/register-company'].includes(path)){ showLogin(); throw new Error('Sessão expirada'); }
  if(!res.ok){
    const detail=Array.isArray(data?.detail)?data.detail.map(x=>x.msg).join(' '):data?.detail;
    throw new Error(detail||'Não foi possível concluir a operação');
  }
  return data;
}

function setCentralAuthMode(mode){
  const login=mode==='login';
  $('#centralLoginTab').classList.toggle('active',login);
  $('#centralRegisterTab').classList.toggle('active',!login);
  $('#loginForm').classList.toggle('hidden',!login);
  $('#registerCompanyForm').classList.toggle('hidden',login);
}
function showLogin(){
  clearInterval(state.refreshTimer); state.user=null;
  $('#appView').classList.add('hidden'); $('#loginView').classList.remove('hidden');
  setCentralAuthMode('login');
}
function showApp(){ $('#loginView').classList.add('hidden'); $('#appView').classList.remove('hidden'); }

async function boot(){
  bindBaseEvents();
  try{ state.user=await api('/auth/me'); showApp(); setupUser(); await navigate('overview'); startRefresh(); }
  catch{ showLogin(); }
}

function bindBaseEvents(){
  $('#centralLoginTab').addEventListener('click',()=>setCentralAuthMode('login'));
  $('#centralRegisterTab').addEventListener('click',()=>setCentralAuthMode('register'));
  $('#centralForgotPassword').addEventListener('click',()=>{
    const email=$('#loginEmail').value.trim();
    const target=new URL('/',window.location.origin);
    target.searchParams.set('forgot','1');
    if(email) target.searchParams.set('email',email);
    window.location.assign(target.toString());
  });
  $('#loginForm').addEventListener('submit',async e=>{
    e.preventDefault();
    const button=e.submitter; button.disabled=true; button.textContent='Entrando...';
    try{
      await api('/auth/login',{method:'POST',body:JSON.stringify({email:$('#loginEmail').value,password:$('#loginPassword').value})});
      state.user=await api('/auth/me'); showApp(); setupUser(); await navigate('overview'); startRefresh();
    }catch(err){ toast(err.message,true); }
    finally{ button.disabled=false; button.textContent='Entrar'; }
  });
  $('#registerCompanyForm').addEventListener('submit',async e=>{
    e.preventDefault();
    const button=e.submitter; button.disabled=true; button.textContent='Criando empresa...';
    try{
      await api('/auth/register-company',{method:'POST',body:JSON.stringify({
        company_name:$('#centralCompanyName').value,
        responsible_name:$('#centralResponsibleName').value,
        email:$('#centralRegisterEmail').value,
        password:$('#centralRegisterPassword').value,
        password_confirmation:$('#centralRegisterPasswordConfirmation').value
      })});
      state.user=await api('/auth/me'); showApp(); setupUser(); await navigate('overview'); startRefresh();
      toast('Empresa criada com sucesso.');
    }catch(err){ toast(err.message,true); }
    finally{ button.disabled=false; button.textContent='Criar empresa'; }
  });
  $('#logoutBtn').addEventListener('click',async()=>{ try{await api('/auth/logout',{method:'POST'});}catch{} showLogin(); });
  $('#refreshBtn').addEventListener('click',()=>renderCurrent(true));
  $('#mainNav').addEventListener('click',e=>{ const btn=e.target.closest('[data-page]'); if(btn) navigate(btn.dataset.page); });
  $('#modalBackdrop').addEventListener('click',e=>{ if(e.target.id==='modalBackdrop') closeModal(); });
}
function setupUser(){
  $('#userName').textContent=state.user.name; $('#userRole').textContent=roleName(state.user.role);
  $('#userInitial').textContent=(state.user.name||'A').slice(0,1).toUpperCase();
  const usersBtn=$('[data-page="users"]'); usersBtn.classList.toggle('hidden',!['platform_admin','company_admin'].includes(state.user.role));
}
function startRefresh(){ clearInterval(state.refreshTimer); state.refreshTimer=setInterval(()=>{ if(['overview','devices','alerts','remote'].includes(state.page)) renderCurrent(false); },15000); }

const pageMeta={
  overview:['Visão geral','Acompanhe as empresas e computadores em tempo quase real.'],
  companies:['Empresas','Organize cada cliente e os computadores vinculados.'],
  devices:['Computadores','Veja saúde, uso e alertas de todas as máquinas autorizadas.'],
  alerts:['Alertas','Priorize o que exige atenção técnica.'],
  remote:['Acesso remoto','Acesse computadores autorizados com registro da solicitação.'],
  users:['Usuários e permissões','Controle quem pode visualizar ou administrar cada empresa.'],
  company:['Detalhes da empresa','Computadores, situação e instalação de novos agentes.'],
  device:['Detalhes do computador','Diagnóstico técnico e histórico de telemetria.']
};
async function navigate(page, context=null){
  state.page=page; if(page==='company') state.selectedCompany=context; if(page==='device') state.selectedDevice=context;
  $$('.nav-item[data-page]').forEach(b=>b.classList.toggle('active',b.dataset.page===page));
  const [title,subtitle]=pageMeta[page]||['CoreTuner Central','']; $('#pageTitle').textContent=title; $('#pageSubtitle').textContent=subtitle;
  await renderCurrent(true);
}
async function renderCurrent(showBusy=true){
  const content=$('#content'); if(showBusy) content.innerHTML='<div class="card empty">Atualizando informações...</div>';
  try{
    if(state.page==='overview') await renderOverview();
    else if(state.page==='companies') await renderCompanies();
    else if(state.page==='devices') await renderDevices();
    else if(state.page==='alerts') await renderAlerts();
    else if(state.page==='remote') await renderRemote();
    else if(state.page==='users') await renderUsers();
    else if(state.page==='company') await renderCompany(state.selectedCompany);
    else if(state.page==='device') await renderDevice(state.selectedDevice);
    $('#lastRefresh').textContent='Atualizado '+new Date().toLocaleTimeString('pt-BR',{hour:'2-digit',minute:'2-digit',second:'2-digit'});
  }catch(err){ content.innerHTML=`<div class="card empty">${esc(err.message)}</div>`; if(showBusy) toast(err.message,true); }
}

async function renderOverview(){
  const [summary,companies,devices,alerts]=await Promise.all([api('/dashboard/summary'),api('/companies'),api('/devices'),api('/alerts?status_filter=active')]);
  setAlertBadge(summary.alerts_open);
  const attention=devices.filter(d=>!d.online||d.health_score<70).slice(0,7);
  $('#content').innerHTML=`
    <div class="grid stats-grid">
      ${stat('Empresas',summary.companies,'Clientes cadastrados')}
      ${stat('Computadores',summary.devices,'Agentes vinculados')}
      ${stat('Online',summary.online,'Comunicando agora','var(--green)')}
      ${stat('Offline',summary.offline,'Sem comunicação','var(--red)')}
      ${stat('Alertas ativos',summary.alerts_open,'Precisam de avaliação','var(--amber)')}
    </div>
    <div class="grid split">
      <div class="card">
        <div class="card-header"><div><h2>Computadores que exigem atenção</h2><p>Offline, nota baixa ou alerta ativo.</p></div><button class="btn small" data-go="devices">Ver todos</button></div>
        ${attention.length?deviceTable(attention):'<div class="empty">Nenhum computador exige atenção agora.</div>'}
      </div>
      <div class="card">
        <div class="card-header"><div><h2>Alertas recentes</h2><p>Eventos técnicos ativos.</p></div><button class="btn small" data-go="alerts">Ver alertas</button></div>
        ${alerts.length?alerts.slice(0,6).map(alertRow).join(''):'<div class="empty">Nenhum alerta ativo.</div>'}
      </div>
    </div>
    <div class="card" style="margin-top:18px">
      <div class="card-header"><div><h2>Empresas</h2><p>Resumo por cliente.</p></div>${state.user.role==='platform_admin'?'<button id="newCompanyBtn" class="btn primary">Cadastrar empresa</button>':''}</div>
      <div class="grid company-cards">${companies.length?companies.map(companyCard).join(''):'<div class="empty">Cadastre a primeira empresa para começar.</div>'}</div>
    </div>`;
  bindCommonActions();
}
function stat(label,value,hint,color='var(--ink)'){ return `<div class="stat-card"><div class="label">${esc(label)}</div><div class="value" style="color:${color}">${esc(value)}</div><div class="hint">${esc(hint)}</div></div>`; }
function companyCard(c){ return `<article class="card company-card" data-company="${c.id}"><div class="company-title"><h3>${esc(c.name)}</h3><span class="pill">${c.devices_total} PCs</span></div><div class="company-metrics"><div class="mini"><strong>${c.devices_online}</strong><span>Online</span></div><div class="mini"><strong>${c.devices_total-c.devices_online}</strong><span>Offline</span></div><div class="mini"><strong>${c.alerts_open}</strong><span>Alertas</span></div></div></article>`; }
function alertRow(a){ return `<div class="info-row"><div><span class="severity-marker ${esc(a.severity)}"></span><strong>${esc(a.title)}</strong><small style="display:block;color:var(--muted);margin:5px 0 0 18px">${esc(a.device_name)}</small></div><span class="pill ${esc(a.severity)}">${a.severity==='critical'?'Crítico':'Atenção'}</span></div>`; }

async function renderCompanies(){
  const companies=await api('/companies');
  $('#content').innerHTML=`<div class="card"><div class="card-header"><div><h2>Empresas cadastradas</h2><p>Cada empresa mantém seus computadores e dados separados.</p></div>${state.user.role==='platform_admin'?'<button id="newCompanyBtn" class="btn primary">Cadastrar empresa</button>':''}</div><div class="grid company-cards">${companies.length?companies.map(companyCard).join(''):'<div class="empty">Nenhuma empresa cadastrada.</div>'}</div></div>`;
  bindCommonActions();
}

async function renderCompany(companyId){
  const c=await api('/companies/'+companyId); state.selectedCompany=companyId;
  $('#pageTitle').textContent=c.name;
  $('#content').innerHTML=`
    <div class="toolbar" style="margin-bottom:16px"><button class="btn" id="backCompanies">← Empresas</button><button class="btn primary" id="enrollBtn">Adicionar computador</button></div>
    <div class="grid stats-grid" style="grid-template-columns:repeat(4,1fr)">${stat('Computadores',c.devices.length,'Total vinculado')}${stat('Online',c.devices.filter(d=>d.online).length,'Comunicando','var(--green)')}${stat('Offline',c.devices.filter(d=>!d.online).length,'Sem comunicação','var(--red)')}${stat('Alertas',c.devices.reduce((s,d)=>s+d.alerts_open,0),'Ativos','var(--amber)')}</div>
    <div class="card"><div class="card-header"><div><h2>Computadores da empresa</h2><p>O agente somente envia informações técnicas autorizadas.</p></div></div>${c.devices.length?deviceTable(c.devices):'<div class="empty">Nenhum computador instalado nesta empresa.</div>'}</div>`;
  $('#backCompanies').onclick=()=>navigate('companies'); $('#enrollBtn').onclick=()=>createEnrollmentToken(c.id,c.name); bindDeviceRows();
}

function remoteLabel(d){
  if(!d.remote?.enabled) return '<span class="pill">Não configurado</span>';
  if(d.remote?.running) return '<span class="pill resolved">Disponível</span>';
  if(d.remote?.installed) return '<span class="pill warning">Parado</span>';
  return '<span class="pill critical">Não instalado</span>';
}
function deviceTable(devices){
  return `<div class="table-wrap"><table><thead><tr><th>Computador</th><th>Empresa/Setor</th><th>Status</th><th>Saúde</th><th>CPU</th><th>Memória</th><th>Disco</th><th>Remoto</th><th>Alertas</th></tr></thead><tbody>${devices.map(d=>`<tr data-device="${d.id}" style="cursor:pointer"><td><strong>${esc(d.name)}</strong><small style="display:block;color:var(--muted);margin-top:3px">${esc(d.hostname)}</small></td><td>${esc(d.sector||'Não informado')}</td><td><span class="status"><i class="dot ${d.online?'online':'offline'}"></i>${d.online?'Online':'Offline'}</span></td><td><span class="health ${healthClass(d.health_score)}">${d.health_score}/100</span></td><td>${fmtNum(d.telemetry?.cpu_percent)}%</td><td>${fmtNum(d.telemetry?.memory_percent)}%</td><td>${fmtNum(d.telemetry?.disk_percent)}%</td><td>${remoteLabel(d)}</td><td>${d.alerts_open?`<span class="pill critical">${d.alerts_open}</span>`:'—'}</td></tr>`).join('')}</tbody></table></div>`;
}
async function renderDevices(){
  const devices=await api('/devices');
  setAlertBadge(devices.reduce((s,d)=>s+d.alerts_open,0));
  $('#content').innerHTML=`<div class="card"><div class="card-header"><div><h2>Todos os computadores</h2><p>${devices.length} equipamento(s) autorizado(s).</p></div><input id="deviceSearch" class="search" placeholder="Pesquisar computador..."></div><div id="deviceTableArea">${devices.length?deviceTable(devices):'<div class="empty">Nenhum computador vinculado.</div>'}</div></div>`;
  bindDeviceRows(); $('#deviceSearch').oninput=e=>{const q=e.target.value.toLowerCase(); const filtered=devices.filter(d=>[d.name,d.hostname,d.sector,d.manufacturer,d.model].some(v=>String(v||'').toLowerCase().includes(q))); $('#deviceTableArea').innerHTML=filtered.length?deviceTable(filtered):'<div class="empty">Nenhum resultado.</div>'; bindDeviceRows();};
}
function bindDeviceRows(){ $$('[data-device]').forEach(row=>row.onclick=()=>navigate('device',Number(row.dataset.device))); }

async function renderDevice(deviceId){
  const d=await api('/devices/'+deviceId); state.selectedDevice=deviceId; $('#pageTitle').textContent=d.name;
  const t=d.telemetry||{};
  $('#content').innerHTML=`
    <div class="toolbar" style="margin-bottom:16px"><button class="btn" id="backDevices">← Computadores</button><span class="status"><i class="dot ${d.online?'online':'offline'}"></i>${d.online?'Online':'Offline'}</span><span class="health ${healthClass(d.health_score)}">Saúde ${d.health_score}/100</span></div>
    <div class="grid detail-grid">
      ${metric('Processador',t.cpu_percent,'%',t.cpu_percent)}${metric('Memória RAM',t.memory_percent,'%',t.memory_percent)}${metric('Disco principal',t.disk_percent,'%',t.disk_percent)}${metric('Temperatura',t.temperature_c,' °C',t.temperature_c?Math.min(100,t.temperature_c):0,t.temperature_c>=85?'critical':'')}
    </div>
    <div class="grid split" style="margin-top:18px">
      <div class="card"><div class="card-header"><div><h2>Histórico recente</h2><p>Últimas ${d.history.length} amostras recebidas.</p></div></div><canvas id="telemetryChart" class="chart"></canvas></div>
      <div class="card"><div class="card-header"><div><h2>Identificação</h2><p>${esc(d.company_name)}</p></div></div><div class="info-list">
        ${info('Fabricante',d.manufacturer)}${info('Modelo',d.model)}${info('Número de série',d.serial_number)}${info('Windows',`${d.os_name||''} ${d.os_version||''}`.trim())}${info('IP local',t.ip_local)}${info('Último contato',fmtDate(d.last_seen))}${info('Agente',d.agent_version)}${info('Perfil aplicado',d.profile||'Nenhum')}
      </div></div>
    </div>
    <div class="grid split" style="margin-top:18px">
      <div class="card"><div class="card-header"><div><h2>Proteções e capacidade</h2><p>Coleta técnica, sem acesso a arquivos do cliente.</p></div></div><div class="info-list">${info('Memória instalada',t.memory_total_gb==null?'—':fmtNum(t.memory_total_gb,1)+' GB')}${info('Memória usada',t.memory_used_gb==null?'—':fmtNum(t.memory_used_gb,1)+' GB')}${info('Disco total',t.disk_total_gb==null?'—':fmtNum(t.disk_total_gb,1)+' GB')}${info('Espaço livre',t.disk_free_gb==null?'—':fmtNum(t.disk_free_gb,1)+' GB')}${info('Microsoft Defender',t.defender_active==null?'Não informado':t.defender_active?'Ativo':'Desativado')}${info('Firewall',t.firewall_active==null?'Não informado':t.firewall_active?'Ativo':'Desativado')}</div></div>
      <div class="card"><div class="card-header"><div><h2>Acesso remoto</h2><p>A solicitação fica registrada no histórico administrativo.</p></div></div><div class="remote-panel"><div>${remoteLabel(d)}<p>${d.remote?.running?'O agente remoto está conectado e pronto para suporte.':d.remote?.installed?'O módulo está instalado, mas não está conectado.':'Instale novamente pelo CoreTuner Setup autorizando o acesso remoto.'}</p></div><button id="remoteAccessBtn" class="btn primary" ${d.remote?.available?'':'disabled'}>Acessar computador</button></div></div>
    </div>`;
  $('#backDevices').onclick=()=>navigate('devices');
  $('#remoteAccessBtn')?.addEventListener('click',()=>openRemoteSession(d.id));
  drawChart($('#telemetryChart'),d.history);
}

function ensureRemoteViewer(){
  let viewer=$('#remoteViewer');
  if(viewer) return viewer;
  viewer=document.createElement('section');
  viewer.id='remoteViewer';
  viewer.className='remote-viewer hidden';
  viewer.innerHTML=`
    <header class="remote-viewer-header">
      <div><strong id="remoteViewerTitle">Acesso remoto</strong><small id="remoteViewerStatus">Preparando conexão segura...</small></div>
      <div class="remote-viewer-actions">
        <button id="remoteViewerNewTab" type="button" class="btn small">Abrir em nova guia</button>
        <button id="remoteViewerClose" type="button" class="btn danger small">Encerrar</button>
      </div>
    </header>
    <div class="remote-viewer-body">
      <div id="remoteViewerLoading" class="remote-viewer-loading"><span></span><p>Conectando ao computador...</p></div>
      <iframe id="remoteViewerFrame" title="Área de trabalho remota" allow="clipboard-read; clipboard-write" referrerpolicy="no-referrer"></iframe>
    </div>`;
  document.body.appendChild(viewer);
  $('#remoteViewerClose').onclick=closeRemoteViewer;
  document.addEventListener('keydown',event=>{ if(event.key==='Escape'&&!viewer.classList.contains('hidden')) closeRemoteViewer(); });
  return viewer;
}
function closeRemoteViewer(){
  const viewer=$('#remoteViewer');
  const frame=$('#remoteViewerFrame');
  if(frame) frame.src='about:blank';
  if(viewer) viewer.classList.add('hidden');
  document.body.classList.remove('remote-viewer-open');
}
async function requestRemoteUrl(deviceId){
  return api(`/devices/${deviceId}/remote-session`,{method:'POST'});
}
async function openRemoteSession(deviceId){
  const viewer=ensureRemoteViewer();
  const frame=$('#remoteViewerFrame');
  const loading=$('#remoteViewerLoading');
  viewer.classList.remove('hidden');
  document.body.classList.add('remote-viewer-open');
  loading.classList.remove('hidden');
  frame.classList.remove('ready');
  frame.src='about:blank';
  $('#remoteViewerTitle').textContent='Acesso remoto';
  $('#remoteViewerStatus').textContent='Gerando acesso temporário...';
  try{
    const data=await requestRemoteUrl(deviceId);
    $('#remoteViewerTitle').textContent=`Acesso remoto — ${data.device_name}`;
    $('#remoteViewerStatus').textContent='Sessão autorizada pelo CoreTuner';
    frame.onload=()=>{ loading.classList.add('hidden'); frame.classList.add('ready'); };
    frame.src=data.url;
    $('#remoteViewerNewTab').onclick=async()=>{
      const tab=window.open('about:blank','_blank');
      try{
        const fresh=await requestRemoteUrl(deviceId);
        if(tab){ tab.opener=null; tab.location=fresh.url; }
      }catch(err){ if(tab)tab.close(); toast(err.message,true); }
    };
    toast('Conexão remota autorizada.');
  }catch(err){
    closeRemoteViewer();
    toast(err.message,true);
  }
}

async function renderRemote(){
  const devices=await api('/devices');
  const ready=devices.filter(d=>d.remote?.available);
  const unavailable=devices.filter(d=>!d.remote?.available);
  $('#content').innerHTML=`<div class="grid stats-grid" style="grid-template-columns:repeat(3,1fr)">${stat('Remoto disponível',ready.length,'Prontos para conexão','var(--green)')}${stat('Indisponíveis',unavailable.length,'Sem agente conectado','var(--amber)')}${stat('Total',devices.length,'Computadores autorizados')}</div><div class="card" style="margin-top:18px"><div class="card-header"><div><h2>Computadores com acesso remoto</h2><p>Somente administradores e técnicos autorizados podem iniciar uma sessão.</p></div></div>${devices.length?`<div class="remote-list">${devices.map(d=>`<div class="remote-row"><div><strong>${esc(d.name)}</strong><small>${esc(d.hostname)} · ${esc(d.sector||'Sem setor')}</small></div>${remoteLabel(d)}<button class="btn small ${d.remote?.available?'primary':''}" data-remote="${d.id}" ${d.remote?.available?'':'disabled'}>Acessar</button></div>`).join('')}</div>`:'<div class="empty">Nenhum computador vinculado.</div>'}</div>`;
  $$('[data-remote]').forEach(button=>button.onclick=()=>openRemoteSession(Number(button.dataset.remote)));
}
function metric(label,value,suffix,percent,extra=''){ const n=value==null?'—':fmtNum(value); const level=extra||(Number(percent)>=90?'critical':Number(percent)>=75?'warning':''); return `<div class="metric ${level}"><small>${esc(label)}</small><div class="big">${n}${value==null?'':suffix}</div><div class="bar"><span style="width:${Math.max(0,Math.min(100,Number(percent)||0))}%"></span></div></div>`; }
function info(label,value){ return `<div class="info-row"><span>${esc(label)}</span><strong>${esc(value||'—')}</strong></div>`; }
function drawChart(canvas,history){
  const ctx=canvas.getContext('2d'); const rect=canvas.getBoundingClientRect(); const ratio=window.devicePixelRatio||1; canvas.width=rect.width*ratio; canvas.height=rect.height*ratio; ctx.scale(ratio,ratio); const w=rect.width,h=rect.height,p=24;
  ctx.clearRect(0,0,w,h); ctx.strokeStyle='#e5ebf3'; ctx.lineWidth=1; for(let i=0;i<=4;i++){const y=p+(h-p*2)*i/4;ctx.beginPath();ctx.moveTo(p,y);ctx.lineTo(w-p,y);ctx.stroke();}
  const series=[['cpu_percent','#1769ff'],['memory_percent','#1ca650'],['disk_percent','#7c3aed']];
  series.forEach(([key,color])=>{ const vals=history.map(x=>x?.[key]).filter(v=>v!=null); if(vals.length<2)return; ctx.strokeStyle=color;ctx.lineWidth=2;ctx.beginPath();let started=false;history.forEach((x,i)=>{const v=x?.[key];if(v==null)return;const px=p+(w-p*2)*(i/Math.max(1,history.length-1));const py=h-p-(h-p*2)*(v/100); if(!started){ctx.moveTo(px,py);started=true;}else ctx.lineTo(px,py);});ctx.stroke();});
  ctx.font='12px Segoe UI';ctx.fillStyle='#66758d';ctx.fillText('CPU',p,15);ctx.fillStyle='#1ca650';ctx.fillText('RAM',p+36,15);ctx.fillStyle='#7c3aed';ctx.fillText('Disco',p+76,15);
}

async function renderAlerts(){
  const alerts=await api('/alerts?status_filter=active'); setAlertBadge(alerts.length);
  $('#content').innerHTML=`<div class="card"><div class="card-header"><div><h2>Alertas ativos</h2><p>Reconhecer um alerta não executa nenhuma ação no computador.</p></div></div>${alerts.length?`<div class="table-wrap"><table><thead><tr><th>Gravidade</th><th>Computador</th><th>Alerta</th><th>Detectado</th><th>Status</th><th></th></tr></thead><tbody>${alerts.map(a=>`<tr><td><span class="pill ${esc(a.severity)}">${a.severity==='critical'?'Crítico':'Atenção'}</span></td><td><strong>${esc(a.device_name)}</strong></td><td><strong>${esc(a.title)}</strong><small style="display:block;color:var(--muted);margin-top:4px">${esc(a.message)}</small></td><td>${fmtDate(a.opened_at)}</td><td><span class="pill ${esc(a.status)}">${a.status==='acknowledged'?'Reconhecido':'Aberto'}</span></td><td>${a.status==='open'?`<button class="btn small" data-ack="${a.id}">Reconhecer</button>`:'—'}</td></tr>`).join('')}</tbody></table></div>`:'<div class="empty">Nenhum alerta ativo.</div>'}</div>`;
  $$('[data-ack]').forEach(b=>b.onclick=async()=>{try{await api('/alerts/'+b.dataset.ack+'/ack',{method:'POST'});toast('Alerta reconhecido.');renderAlerts();}catch(e){toast(e.message,true);}});
}

async function renderUsers(){
  const users=await api('/users'); const companies=state.user.role==='platform_admin'?await api('/companies'):[];
  $('#content').innerHTML=`<div class="card"><div class="card-header"><div><h2>Usuários autorizados</h2><p>Permissões respeitam a separação entre empresas.</p></div><button id="newUserBtn" class="btn primary">Cadastrar usuário</button></div><div class="table-wrap"><table><thead><tr><th>Nome</th><th>E-mail</th><th>Perfil</th><th>Empresa</th><th>Status</th></tr></thead><tbody>${users.map(u=>`<tr><td><strong>${esc(u.name)}</strong></td><td>${esc(u.email)}</td><td>${esc(roleName(u.role))}</td><td>${esc(companies.find(c=>c.id===u.company_id)?.name||'Todas as empresas')}</td><td><span class="pill ${u.active?'resolved':'critical'}">${u.active?'Ativo':'Bloqueado'}</span></td></tr>`).join('')}</tbody></table></div></div>`;
  $('#newUserBtn').onclick=()=>openUserModal(companies);
}

function bindCommonActions(){
  $$('[data-go]').forEach(b=>b.onclick=()=>navigate(b.dataset.go));
  $$('[data-company]').forEach(c=>c.onclick=()=>navigate('company',Number(c.dataset.company)));
  $('#newCompanyBtn')?.addEventListener('click',openCompanyModal); bindDeviceRows();
}
function setAlertBadge(n){ const b=$('#navAlertCount'); b.textContent=n; b.classList.toggle('hidden',!n); }
function openModal(html){ $('#modal').innerHTML=html; $('#modalBackdrop').classList.remove('hidden'); }
function closeModal(){ $('#modalBackdrop').classList.add('hidden'); $('#modal').innerHTML=''; }
function openCompanyModal(){
  openModal(`<h2>Cadastrar empresa</h2><p>Os computadores e usuários desta empresa ficarão isolados dos demais clientes.</p><form id="companyForm" class="stack"><label>Nome da empresa<input id="companyName" required minlength="2" maxlength="160"></label><div class="modal-actions"><button type="button" class="btn" id="cancelModal">Cancelar</button><button type="submit" class="btn primary">Cadastrar</button></div></form>`);
  $('#cancelModal').onclick=closeModal; $('#companyForm').onsubmit=async e=>{e.preventDefault();try{const c=await api('/companies',{method:'POST',body:JSON.stringify({name:$('#companyName').value})});closeModal();toast('Empresa cadastrada.');navigate('company',c.id);}catch(err){toast(err.message,true);}};
}
async function createEnrollmentToken(companyId,companyName){
  try{ const data=await api(`/companies/${companyId}/enrollment-token`,{method:'POST'}); openModal(`<h2>Adicionar computador</h2><p>Use este token uma única vez no instalador do agente da empresa <strong>${esc(companyName)}</strong>.</p><div class="token-box" id="tokenText">${esc(data.token)}</div><div class="callout" style="margin-top:14px">Validade: ${fmtDate(data.expires_at)}. O token deixa de funcionar assim que um computador for vinculado.</div><div class="modal-actions"><button class="btn" id="closeToken">Fechar</button><button class="btn primary" id="copyToken">Copiar token</button></div>`); $('#closeToken').onclick=closeModal; $('#copyToken').onclick=async()=>{await navigator.clipboard.writeText(data.token);toast('Token copiado.');}; }
  catch(err){toast(err.message,true);}
}
function openUserModal(companies){
  const companyField=state.user.role==='platform_admin'?`<label>Empresa<select id="userCompany"><option value="">Todas (somente administrador da plataforma)</option>${companies.map(c=>`<option value="${c.id}">${esc(c.name)}</option>`).join('')}</select></label>`:'';
  openModal(`<h2>Cadastrar usuário</h2><p>Conceda somente o nível necessário para o trabalho da pessoa.</p><form id="userForm" class="stack"><label>Nome<input id="newUserName" required minlength="2"></label><label>E-mail<input id="newUserEmail" type="email" required></label><label>Senha inicial<input id="newUserPassword" type="password" required minlength="10"></label><label>Perfil<select id="newUserRole"><option value="viewer">Visualização</option><option value="technician">Técnico</option><option value="company_admin">Administrador da empresa</option>${state.user.role==='platform_admin'?'<option value="platform_admin">Administrador da plataforma</option>':''}</select></label>${companyField}<div class="modal-actions"><button type="button" class="btn" id="cancelModal">Cancelar</button><button class="btn primary" type="submit">Cadastrar</button></div></form>`);
  $('#cancelModal').onclick=closeModal; $('#userForm').onsubmit=async e=>{e.preventDefault(); const companyVal=$('#userCompany')?.value; try{await api('/users',{method:'POST',body:JSON.stringify({name:$('#newUserName').value,email:$('#newUserEmail').value,password:$('#newUserPassword').value,role:$('#newUserRole').value,company_id:companyVal?Number(companyVal):null})});closeModal();toast('Usuário cadastrado.');renderUsers();}catch(err){toast(err.message,true);}};
}

document.addEventListener('DOMContentLoaded',boot);

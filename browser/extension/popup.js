const status = document.getElementById('status');
const text = document.getElementById('statusText');
const button = document.getElementById('sync');

async function load() {
  const data = await chrome.storage.local.get(['corecontrol_status','corecontrol_last_sync','corecontrol_last_error']);
  const ok = data.corecontrol_status === 'connected';
  status.classList.toggle('ok', ok);
  status.classList.toggle('bad', data.corecontrol_status === 'error');
  if (ok) {
    text.textContent = 'Conectado ao CoreControl';
  } else if (data.corecontrol_status === 'error') {
    text.textContent = 'Integração local não encontrada';
    status.title = data.corecontrol_last_error || '';
  } else {
    text.textContent = 'Aguardando primeira sincronização';
  }
}
button.addEventListener('click', async () => {
  button.disabled = true;
  button.textContent = 'Atualizando…';
  await chrome.runtime.sendMessage({type:'sync-now'});
  await load();
  button.disabled = false;
  button.textContent = 'Atualizar agora';
});
load();

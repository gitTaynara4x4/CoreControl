from pathlib import Path


def test_backend_returns_last_successful_snapshot_while_refreshing():
    api = Path('app/update_api.py').read_text(encoding='utf-8')
    assert 'def _latest_successful_activity_command' in api
    assert 'AgentCommand.status == "succeeded"' in api
    assert '"cached_command": _activity_command_public(_latest_successful_activity_command(db, device.id))' in api


def test_frontend_never_discards_last_successful_list_during_polling():
    js = Path('app/static/js/pages/devices.js').read_text(encoding='utf-8')
    assert 'let activityLastSuccessfulCommand = null;' in js
    assert "activityLastSuccessfulCommand = serverCached;" in js
    assert "const cached = serverCached?.status === 'succeeded' ? serverCached : activityLastSuccessfulCommand;" in js
    assert "Falha ao atualizar · exibindo última lista" in js
    assert "requestActivitySnapshot(device, { silent: true })" in js


def test_v107_optimization_is_preserved():
    js = Path('app/static/js/pages/devices.js').read_text(encoding='utf-8')
    assert 'function renderOptimization(device, state)' in js
    assert '/optimization' in js


def test_cache_bust_is_current_and_css_fix_is_preserved():
    html = Path('app/static/index.html').read_text(encoding='utf-8')
    assert 'styles.css?v=20260902-css-activity-fix-v10-7-1' in html
    assert 'devices.js?v=20260902-activity-persist-v10-7-2' in html

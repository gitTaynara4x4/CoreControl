from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def test_update_modal_explains_available_items_by_source():
    js = read("app/static/js/pages/updates.js")
    assert '<small>Disponíveis</small>' in js
    assert '<small>Windows</small>' in js
    assert '<small>Drivers</small>' in js
    assert '<small>Aplicativos</small>' in js
    assert '1. Escolha o que deseja atualizar' in js
    assert 'Nada é instalado apenas por marcar uma caixa.' in js


def test_offline_state_explains_queue_without_ambiguous_copy():
    js = read("app/static/js/pages/updates.js")
    assert 'Computador offline' in js
    assert 'o CoreControl guarda o pedido' in js
    assert 'Colocar ${count} na fila' in js
    assert 'aguardando o computador ficar online' in js
    assert 'Guardar ${itemKeys.length} atualização' in js


def test_online_state_uses_install_now_copy():
    js = read("app/static/js/pages/updates.js")
    assert 'Computador pronto para atualizar' in js
    assert 'Instalar ${count} agora' in js
    assert 'enviada${itemKeys.length === 1 ? \'\' : \'s\'} para instalação' in js


def test_operation_notice_distinguishes_scan_and_install_queue():
    js = read("app/static/js/pages/updates.js")
    assert "commandType === 'updates.install'" in js
    assert "commandType === 'updates.scan'" in js
    assert 'Instalação aguardando o computador ficar online' in js
    assert 'Verificação aguardando o computador ficar online' in js


def test_update_clarity_styles_and_cache_busting():
    css = read("app/static/styles.css")
    html = read("app/static/index.html")
    assert '.update-operation-notice' in css
    assert '.update-choice-help' in css
    assert '.update-selection-count' in css
    assert '/static/styles.css?v=20260910-updates-clarity-v10-52' in html
    assert '/static/js/pages/updates.js?v=20260910-updates-clarity-v10-52' in html

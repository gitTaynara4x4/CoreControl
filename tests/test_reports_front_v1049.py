from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
REPORTS_HTML = (ROOT / "app/static/pages/reports.html").read_text(encoding="utf-8")
REPORTS_JS = (ROOT / "app/static/js/pages/reports.js").read_text(encoding="utf-8")
ROUTER_JS = (ROOT / "app/static/js/router.js").read_text(encoding="utf-8")
INDEX_HTML = (ROOT / "app/static/index.html").read_text(encoding="utf-8")
STYLES = (ROOT / "app/static/styles.css").read_text(encoding="utf-8")


def test_reports_uses_custom_tabs_and_view():
    assert 'class="reports-tabs"' in REPORTS_HTML
    assert 'data-reports-tab="center"' in REPORTS_HTML
    assert 'data-reports-tab="audit"' in REPORTS_HTML
    assert 'data-reports-tab="exports"' in REPORTS_HTML
    assert 'id="reportsView"' in REPORTS_HTML


def test_reports_keeps_tab_on_refresh():
    assert "REPORT_TABS" in REPORTS_JS
    assert "searchParams.get('tab')" in REPORTS_JS
    assert "searchParams.set('tab', tab)" in REPORTS_JS
    assert "'reports'" in ROUTER_JS
    assert "['updates', 'scripts', 'network', 'reports', 'settings']" in ROUTER_JS


def test_reports_center_has_search_and_categories():
    assert 'id="reportsSearch"' in REPORTS_JS
    assert 'data-report-group=' in REPORTS_JS
    assert 'data-report-select=' in REPORTS_JS
    assert 'reports-category-grid' in REPORTS_JS
    assert 'reportsSearchEmpty' in REPORTS_JS


def test_reports_export_does_not_fake_backend():
    assert 'A geração real ainda depende do backend de exportações.' in REPORTS_JS
    assert 'falta conectar a geração de arquivos ao backend' in REPORTS_JS.lower()
    assert 'Auditoria aguardando integração' in REPORTS_JS


def test_reports_styles_and_cache_busting_are_present():
    assert '/* ===== Relatórios v10.49 ===== */' in STYLES
    assert '.reports-category-grid' in STYLES
    assert '.reports-export-layout' in STYLES
    assert '/static/js/pages/reports.js?v=20260909-reports-ui-v10-49' in INDEX_HTML

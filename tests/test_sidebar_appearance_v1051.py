from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CSS = (ROOT / 'app/static/styles.css').read_text(encoding='utf-8')
INDEX = (ROOT / 'app/static/index.html').read_text(encoding='utf-8')


def test_appearance_menu_is_overlay_not_document_flow():
    assert '.settings-wrap{' in CSS
    assert 'position:relative' in CSS
    assert '.appearance-menu{' in CSS
    assert 'position:absolute' in CSS
    assert 'bottom:calc(100% + 8px)' in CSS
    assert 'z-index:40' in CSS


def test_theme_options_are_compact_and_icons_are_bounded():
    assert '.theme-option{' in CSS
    assert 'grid-template-columns:30px minmax(0,1fr) 18px' in CSS
    assert 'min-height:48px' in CSS
    assert '.theme-option-icon svg{' in CSS
    assert 'width:16px!important' in CSS
    assert 'height:16px!important' in CSS


def test_active_theme_has_clear_selection_state():
    assert '.theme-option.active{' in CSS
    assert '.theme-option.active .theme-check{' in CSS
    assert 'opacity:1' in CSS


def test_dark_theme_has_sidebar_appearance_styles():
    assert 'html[data-theme="dark"] .appearance-menu{' in CSS
    assert 'html[data-theme="dark"] .theme-option.active{' in CSS


def test_index_busts_sidebar_appearance_cache():
    assert 'styles.css?v=20260910-updates-clarity-v10-52' in INDEX

from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

def test_agent_version_bumped():
    main = (ROOT / "agent/src/main.go").read_text(encoding="utf-8")
    assert 'const agentVersion = "0.8.7"' in main

def test_uia_filters_webpage_tab_widgets():
    source = (ROOT / "agent/src/browser_tabs_windows.go").read_text(encoding="utf-8")
    assert 'ControlType]::Document' in source
    assert 'Chrome_RenderWidgetHostHWND' in source
    assert 'if($insidePage){ continue }' in source
    assert '$chromeBand=' in source
    assert 'if($tabRect.Top -gt ($windowRect.Top + $chromeBand)){ continue }' in source

def test_no_brittle_disney_title_blacklist():
    source = (ROOT / "agent/src/browser_tabs_windows.go").read_text(encoding="utf-8").lower()
    # The fix is structural: it must not depend on hard-coded Disney page labels.
    for label in ("sugestões", "detalhes", "extras", "versões"):
        assert label not in source

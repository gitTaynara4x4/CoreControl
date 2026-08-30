from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding='utf-8')


def test_uia_does_not_overwrite_powershell_pid_automatic_variable():
    source = read('agent/src/browser_tabs_windows.go')
    assert '$processId=[int]$w.Current.ProcessId' in source
    assert 'Get-Process -Id $processId' in source
    assert '$pid=[int]$w.Current.ProcessId' not in source


def test_uia_outputs_utf8_for_titles_with_accents():
    source = read('agent/src/browser_tabs_windows.go')
    assert '[Console]::OutputEncoding=[System.Text.UTF8Encoding]::new($false);' in source


def test_agent_version_bumped_for_browser_tab_pid_fix():
    main = read('agent/src/main.go')
    assert 'const agentVersion = "0.8.6"' in main

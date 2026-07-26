import base64
from types import SimpleNamespace

from app.meshcentral import MeshCentralClient, MeshDevice, _mesh_id_to_hex, build_remote_desktop_url


def test_mesh_id_to_hex_accepts_meshcentral_base64_identifier():
    raw = bytes(range(32))
    encoded = base64.b64encode(raw).decode("ascii").rstrip("=")
    mesh_id = f"mesh//{encoded}"
    assert _mesh_id_to_hex(mesh_id) == raw.hex()




def test_mesh_id_to_hex_accepts_bare_hex_identifier_from_meshctrl_hex_mode():
    raw_hex = bytes(range(32)).hex()
    assert _mesh_id_to_hex(raw_hex) == raw_hex


def test_mesh_id_to_hex_accepts_prefixed_hex_identifier():
    raw_hex = bytes(reversed(range(32))).hex()
    assert _mesh_id_to_hex(f"mesh//{raw_hex}") == raw_hex


def test_remote_url_targets_exact_node():
    url = build_remote_desktop_url(
        base_url="https://remote.example.com",
        login_token="TOKEN",
        node_id="node//NODE123",
    )
    assert "gotonode=NODE123" in url
    assert "viewmode=11" in url
    assert "hide=63" in url


def test_match_device_uses_saved_node_then_unique_hostname():
    client = MeshCentralClient()
    devices = [
        MeshDevice(
            node_id="node//A",
            mesh_id="mesh//G",
            name="PC SALA",
            real_name="",
            hostname="DESKTOP-A",
            connected=True,
            raw={},
        ),
        MeshDevice(
            node_id="node//B",
            mesh_id="mesh//G",
            name="OUTRO",
            real_name="",
            hostname="DESKTOP-B",
            connected=False,
            raw={},
        ),
    ]
    assert client.match_device(SimpleNamespace(mesh_node_id="node//B", hostname="x", name="x"), devices).node_id == "node//B"
    assert client.match_device(SimpleNamespace(mesh_node_id=None, hostname="desktop-a", name="irrelevante"), devices).node_id == "node//A"

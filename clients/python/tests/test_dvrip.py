"""DVRIP codec + Sofia hash testleri (Go testleriyle aynı vektörler)."""
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from xmeye_wrapper.dvrip import (  # noqa: E402
    Packet, build_login, sofia_hash, MSG_LOGIN_REQ, MSG_OPMONITOR_DATA,
)


def test_sofia_hash_known_vector():
    # boş parola → tlJwpbo6 (Go ile aynı bilinen referans)
    assert sofia_hash("") == "tlJwpbo6"


def test_sofia_hash_shape():
    for pw in ["", "admin123.", "koy1", "password"]:
        assert len(sofia_hash(pw)) == 8


def test_packet_roundtrip():
    p = Packet(msg_id=MSG_OPMONITOR_DATA, body=b'{"Name":"OPMonitor"}',
               session_id=0x084E, sequence=2043)
    wire = p.encode()
    got, consumed = Packet.decode(wire)
    assert consumed == len(wire)
    assert got.session_id == 0x084E
    assert got.sequence == 2043
    assert got.msg_id == MSG_OPMONITOR_DATA
    assert got.body == p.body


def test_decode_real_header():
    # login2.pcap başlığı: sess=0x084e seq=2043 msg=1412 len=168
    hdr = bytes([
        0xFF, 0x01, 0x00, 0x00,
        0x4E, 0x08, 0x00, 0x00,
        0xFB, 0x07, 0x00, 0x00,
        0x01, 0x00,
        0x84, 0x05,
        0xA8, 0x00, 0x00, 0x00,
    ])
    p, consumed = Packet.decode(hdr + b"\x00" * 168)
    assert p.session_id == 0x084E
    assert p.sequence == 2043
    assert p.msg_id == 1412
    assert len(p.body) == 168
    assert consumed == 188


def test_build_login():
    p = build_login("koy1", "admin123.")
    assert p.msg_id == MSG_LOGIN_REQ
    assert p.body.endswith(b"\x0a\x00")
    assert sofia_hash("admin123.").encode() in p.body


def test_short_buffer():
    try:
        Packet.decode(b"\xff\x01")
        assert False, "short bekleniyordu"
    except ValueError as e:
        assert "short" in str(e)

"""XMEye / Xiongmai DVRIP (Sofia) protokol — Python referans.

Go çekirdeğinin (core-go/dvrip) Python portu. Paket codec + Sofia parola hash.
Protokol: 20 byte başlık (FF 01 ...) + gövde, little-endian. Ayrıntı: ../../core-go/dvrip/packet.go
"""
from __future__ import annotations

import hashlib
import struct
from dataclasses import dataclass, field

HEADER_LEN = 20
MAGIC_HEAD = 0xFF
MAGIC_VERSION = 0x01

# DVRIP mesaj ID'leri (Go commands.go ile aynı)
MSG_LOGIN_REQ = 1000
MSG_LOGIN_RESP = 1001
MSG_KEEPALIVE = 1006
MSG_KEEP_RESP = 1007
MSG_OPMONITOR_DATA = 1412  # video frame (pcap'te doğrulandı)
MSG_OPMONITOR_CLAIM = 1413
MSG_OPMONITOR_REQ = 1410
MSG_OPPLAYBACK = 1420


def sofia_hash(password: str) -> str:
    """Xiongmai/Sofia parola özeti ('compressed MD5').

    MD5 → ardışık ikişer byte toplanır → [0-9A-Za-z] alfabesinde 8 karaktere eşlenir.
    Boş parola → 'tlJwpbo6' (bilinen referans).
    """
    alphabet = (
        "0123456789"
        "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
        "abcdefghijklmnopqrstuvwxyz"
    )
    digest = hashlib.md5(password.encode()).digest()  # 16 byte
    out = []
    for i in range(8):
        n = digest[2 * i] + digest[2 * i + 1]
        out.append(alphabet[n % len(alphabet)])
    return "".join(out)


@dataclass
class Packet:
    msg_id: int
    body: bytes = b""
    session_id: int = 0
    sequence: int = 0
    total_packet: int = 0
    cur_packet: int = 0

    def encode(self) -> bytes:
        head = struct.pack(
            "<BBBBIIBBHI",
            MAGIC_HEAD, MAGIC_VERSION, 0, 0,
            self.session_id, self.sequence,
            self.total_packet, self.cur_packet,
            self.msg_id, len(self.body),
        )
        return head + self.body

    @classmethod
    def decode(cls, buf: bytes) -> tuple["Packet", int]:
        """Baştaki tek paketi çöz; (paket, tüketilen_byte) döner.

        Gövde eksikse ValueError('short') atar (akış reassembly).
        """
        if len(buf) < HEADER_LEN:
            raise ValueError("short")
        if buf[0] != MAGIC_HEAD or buf[1] != MAGIC_VERSION:
            raise ValueError("bad magic")
        (_h, _v, _r1, _r2, session, seq, total, cur, msg, dlen) = struct.unpack(
            "<BBBBIIBBHI", buf[:HEADER_LEN]
        )
        if dlen > (8 << 20):
            raise ValueError("insane length")
        total_len = HEADER_LEN + dlen
        if len(buf) < total_len:
            raise ValueError("short")
        return (
            cls(
                msg_id=msg, body=buf[HEADER_LEN:total_len],
                session_id=session, sequence=seq,
                total_packet=total, cur_packet=cur,
            ),
            total_len,
        )


def build_login(user: str, password: str) -> Packet:
    """DVRIP login paketi (Sofia JSON gövdesi, \\n\\x00 sonlandırıcı)."""
    import json
    req = {
        "EncryptType": "MD5",
        "LoginType": "DVRIP-Web",
        "PassWord": sofia_hash(password),
        "UserName": user,
    }
    body = json.dumps(req, separators=(",", ":")).encode() + b"\x0a\x00"
    return Packet(msg_id=MSG_LOGIN_REQ, body=body)

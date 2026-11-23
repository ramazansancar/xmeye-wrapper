// XMEye / Xiongmai DVRIP (Sofia) protokol — Node.js portu.
// Go çekirdeğiyle (core-go/dvrip) birebir: 20 byte başlık (FF 01 ...) + gövde, little-endian.
import { createHash } from 'node:crypto';

export const HEADER_LEN = 20;
const MAGIC_HEAD = 0xff;
const MAGIC_VERSION = 0x01;

export const MSG = {
  LOGIN_REQ: 1000,
  LOGIN_RESP: 1001,
  KEEPALIVE: 1006,
  KEEP_RESP: 1007,
  OPMONITOR_DATA: 1412, // video frame (pcap doğrulandı)
  OPMONITOR_CLAIM: 1413,
  OPMONITOR_REQ: 1410,
  OPPLAYBACK: 1420,
};

const ALPHABET =
  '0123456789' +
  'ABCDEFGHIJKLMNOPQRSTUVWXYZ' +
  'abcdefghijklmnopqrstuvwxyz';

/** Xiongmai/Sofia parola özeti ('compressed MD5'). Boş parola → 'tlJwpbo6'. */
export function sofiaHash(password) {
  const d = createHash('md5').update(password, 'utf8').digest(); // 16 byte
  let out = '';
  for (let i = 0; i < 8; i++) {
    const n = d[2 * i] + d[2 * i + 1];
    out += ALPHABET[n % ALPHABET.length];
  }
  return out;
}

/** DVRIP paketi encode → Buffer. */
export function encode({ msgId, body = Buffer.alloc(0), sessionId = 0, sequence = 0, totalPacket = 0, curPacket = 0 }) {
  const buf = Buffer.alloc(HEADER_LEN + body.length);
  buf[0] = MAGIC_HEAD;
  buf[1] = MAGIC_VERSION;
  buf.writeUInt32LE(sessionId, 4);
  buf.writeUInt32LE(sequence, 8);
  buf[12] = totalPacket;
  buf[13] = curPacket;
  buf.writeUInt16LE(msgId, 14);
  buf.writeUInt32LE(body.length, 16);
  body.copy(buf, HEADER_LEN);
  return buf;
}

/** Baştaki tek paketi çöz → { packet, consumed }. Gövde eksikse Error('short'). */
export function decode(buf) {
  if (buf.length < HEADER_LEN) throw new Error('short');
  if (buf[0] !== MAGIC_HEAD || buf[1] !== MAGIC_VERSION) throw new Error('bad magic');
  const dataLen = buf.readUInt32LE(16);
  if (dataLen > 8 << 20) throw new Error('insane length');
  const total = HEADER_LEN + dataLen;
  if (buf.length < total) throw new Error('short');
  return {
    packet: {
      sessionId: buf.readUInt32LE(4),
      sequence: buf.readUInt32LE(8),
      totalPacket: buf[12],
      curPacket: buf[13],
      msgId: buf.readUInt16LE(14),
      body: buf.subarray(HEADER_LEN, total),
    },
    consumed: total,
  };
}

/** DVRIP login paketi (Sofia JSON + \n\x00 sonlandırıcı). */
export function buildLogin(user, password) {
  const req = { EncryptType: 'MD5', LoginType: 'DVRIP-Web', PassWord: sofiaHash(password), UserName: user };
  const body = Buffer.concat([Buffer.from(JSON.stringify(req)), Buffer.from([0x0a, 0x00])]);
  return encode({ msgId: MSG.LOGIN_REQ, body });
}

// DVRIP codec + Sofia hash testleri (Go/Python ile aynı vektörler). node:test, bağımlılık yok.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { encode, decode, sofiaHash, buildLogin, MSG } from '../src/dvrip.js';

test('sofia hash bilinen vektör (boş → tlJwpbo6)', () => {
  assert.equal(sofiaHash(''), 'tlJwpbo6');
});

test('sofia hash uzunluk 8', () => {
  for (const pw of ['', 'admin123.', 'koy1', 'password']) {
    assert.equal(sofiaHash(pw).length, 8);
  }
});

test('packet round-trip', () => {
  const body = Buffer.from('{"Name":"OPMonitor"}');
  const wire = encode({ msgId: MSG.OPMONITOR_DATA, body, sessionId: 0x084e, sequence: 2043 });
  const { packet, consumed } = decode(wire);
  assert.equal(consumed, wire.length);
  assert.equal(packet.sessionId, 0x084e);
  assert.equal(packet.sequence, 2043);
  assert.equal(packet.msgId, MSG.OPMONITOR_DATA);
  assert.deepEqual(packet.body, body);
});

test('decode gerçek header (login2.pcap)', () => {
  const hdr = Buffer.from([
    0xff, 0x01, 0x00, 0x00,
    0x4e, 0x08, 0x00, 0x00,
    0xfb, 0x07, 0x00, 0x00,
    0x01, 0x00,
    0x84, 0x05,
    0xa8, 0x00, 0x00, 0x00,
  ]);
  const { packet, consumed } = decode(Buffer.concat([hdr, Buffer.alloc(168)]));
  assert.equal(packet.sessionId, 0x084e);
  assert.equal(packet.sequence, 2043);
  assert.equal(packet.msgId, 1412);
  assert.equal(packet.body.length, 168);
  assert.equal(consumed, 188);
});

test('build login', () => {
  const wire = buildLogin('koy1', 'admin123.');
  const { packet } = decode(wire);
  assert.equal(packet.msgId, MSG.LOGIN_REQ);
  const s = packet.body.toString('latin1');
  assert.ok(s.endsWith('\n\x00'));
  assert.ok(s.includes(sofiaHash('admin123.')));
});

test('short buffer', () => {
  assert.throws(() => decode(Buffer.from([0xff, 0x01])), /short/);
});

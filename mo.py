"""Minimal GNU gettext .mo reader/writer (no external deps)."""
import struct


def read_mo(path):
    data = open(path, 'rb').read()
    magic = struct.unpack('<I', data[:4])[0]
    end = '<' if magic == 0x950412de else '>'
    _, n, oo, to = struct.unpack(end + '4I', data[4:20])
    out = []
    for i in range(n):
        ol, op = struct.unpack(end + '2I', data[oo + i * 8: oo + i * 8 + 8])
        tl, tp = struct.unpack(end + '2I', data[to + i * 8: to + i * 8 + 8])
        out.append((data[op:op + ol], data[tp:tp + tl]))
    return out  # list of (msgid_bytes, msgstr_bytes); msgid may contain \x04 ctx, \x00 plural


def _hashpjw(b):
    h = 0
    for c in b:
        h = ((h << 4) + c) & 0xffffffff
        g = h & 0xf0000000
        if g:
            h ^= g >> 24
            h ^= g
    return h


def _next_prime(n):
    def isp(x):
        if x < 2: return False
        i = 2
        while i * i <= x:
            if x % i == 0: return False
            i += 1
        return True
    n = max(n, 3)
    while not isp(n): n += 1
    return n


def write_mo(path, entries):
    """entries: list of (msgid_bytes, msgstr_bytes). Writes a valid .mo (no hash table)."""
    entries = sorted(entries, key=lambda e: e[0])
    n = len(entries)
    oo = 28
    to = oo + n * 8
    so = to + n * 8
    ids = b''
    strs = b''
    otab = []
    ttab = []
    for mid, mstr in entries:
        otab.append((len(mid), so + len(ids)))
        ids += mid + b'\0'
    base = so + len(ids)
    for mid, mstr in entries:
        ttab.append((len(mstr), base + len(strs)))
        strs += mstr + b'\0'
    hsize = _next_prime((n * 4) // 3)
    ho = so
    hb = 4 * hsize
    # shift string offsets by hash table size
    otab = [(l, o + hb) for l, o in otab]
    ttab = [(l, o + hb) for l, o in ttab]
    table = [0] * hsize
    for idx, (mid, _) in enumerate(entries):
        hv = _hashpjw(mid)
        i = hv % hsize
        inc = 1 + (hv % (hsize - 2))
        while table[i]:
            i = (i + inc) % hsize
        table[i] = idx + 1
    hdr = struct.pack('<7I', 0x950412de, 0, n, oo, to, hsize, ho)
    body = b''.join(struct.pack('<2I', *x) for x in otab) + b''.join(struct.pack('<2I', *x) for x in ttab)
    open(path, 'wb').write(hdr + body + struct.pack('<%dI' % hsize, *table) + ids + strs)

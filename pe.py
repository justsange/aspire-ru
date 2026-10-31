"""Minimal PE resource reader for comparing localisation DLLs."""
import struct

RT = {1: 'CURSOR', 2: 'BITMAP', 3: 'ICON', 4: 'MENU', 5: 'DIALOG', 6: 'STRING', 9: 'ACCEL',
      10: 'RCDATA', 12: 'GROUP_CURSOR', 14: 'GROUP_ICON', 16: 'VERSION', 23: 'HTML', 24: 'MANIFEST', 240: 'DLGINIT', 241: 'TOOLBAR'}


class PE:
    def __init__(self, path):
        self.d = open(path, 'rb').read()
        d = self.d
        pe = struct.unpack_from('<I', d, 0x3c)[0]
        nsec = struct.unpack_from('<H', d, pe + 6)[0]
        optsz = struct.unpack_from('<H', d, pe + 20)[0]
        opt = pe + 24
        magic = struct.unpack_from('<H', d, opt)[0]
        ddir = opt + (112 if magic == 0x20b else 96)
        self.rsrc_rva, self.rsrc_size = struct.unpack_from('<II', d, ddir + 2 * 8)
        self.secs = []
        so = opt + optsz
        for i in range(nsec):
            name = d[so:so + 8].rstrip(b'\0')
            vsize, va, rsize, raw = struct.unpack_from('<IIII', d, so + 8)
            self.secs.append((name, va, vsize, raw, rsize))
            so += 40

    def off(self, rva):
        for n, va, vs, raw, rs in self.secs:
            if va <= rva < va + max(vs, rs):
                return rva - va + raw
        raise ValueError(hex(rva))

    def _dir(self, o):
        base = self.off(self.rsrc_rva)
        d = self.d
        nn, ni = struct.unpack_from('<HH', d, base + o + 12)
        res = []
        for i in range(nn + ni):
            name, off = struct.unpack_from('<II', d, base + o + 16 + i * 8)
            if name & 0x80000000:
                p = base + (name & 0x7fffffff)
                l = struct.unpack_from('<H', d, p)[0]
                key = d[p + 2:p + 2 + l * 2].decode('utf-16le')
            else:
                key = name
            res.append((key, off))
        return res

    def resources(self):
        """yield (type, name, lang, bytes)"""
        base = self.off(self.rsrc_rva)
        for t, o1 in self._dir(0):
            for n, o2 in self._dir(o1 & 0x7fffffff):
                for lg, o3 in self._dir(o2 & 0x7fffffff):
                    rva, size = struct.unpack_from('<II', self.d, base + o3)
                    p = self.off(rva)
                    yield t, n, lg, self.d[p:p + size]


def string_table(block_id, data):
    out = {}
    p = 0
    for i in range(16):
        l = struct.unpack_from('<H', data, p)[0]
        p += 2
        s = data[p:p + l * 2].decode('utf-16le', 'replace')
        p += l * 2
        if l:
            out[(block_id - 1) * 16 + i] = s
    return out


def _sz_or_ord(d, p):
    w = struct.unpack_from('<H', d, p)[0]
    if w == 0xffff:
        return struct.unpack_from('<H', d, p + 2)[0], p + 4
    if w == 0:
        return '', p + 2
    e = p
    while struct.unpack_from('<H', d, e)[0]:
        e += 2
    return d[p:e].decode('utf-16le', 'replace'), e + 2


def dialog_texts(d):
    """Returns (title, [(ctrl_id, text)]) for DIALOGEX or DIALOG templates."""
    texts = []
    ex = struct.unpack_from('<HH', d, 0) == (1, 0xffff)
    if ex:
        p = 26
        style = struct.unpack_from('<I', d, 12)[0]
        n = struct.unpack_from('<H', d, 16)[0]
    else:
        style = struct.unpack_from('<I', d, 0)[0]
        n = struct.unpack_from('<H', d, 8)[0]
        p = 18
    menu, p = _sz_or_ord(d, p)
    cls, p = _sz_or_ord(d, p)
    title, p = _sz_or_ord(d, p)
    if style & 0x40 or style & 0x48:  # DS_SETFONT
        p += 6 if ex else 2
        _, p = _sz_or_ord(d, p)
    for i in range(n):
        p = (p + 3) & ~3
        if ex:
            cx = struct.unpack_from('<h', d, p + 16)[0]
            cid = struct.unpack_from('<I', d, p + 20)[0]
            p += 24
        else:
            cx = struct.unpack_from('<h', d, p + 12)[0]
            cid = struct.unpack_from('<H', d, p + 16)[0]
            p += 18
        c, p = _sz_or_ord(d, p)
        t, p = _sz_or_ord(d, p)
        extra = struct.unpack_from('<H', d, p)[0]
        p += 2 + extra
        if isinstance(t, str) and t:
            texts.append((cid, t, cx))
    return title, texts


def menu_texts(d):
    out = []
    ver = struct.unpack_from('<H', d, 0)[0]
    if ver == 1:  # MENUEX
        p = 4 + struct.unpack_from('<H', d, 2)[0]
        while p < len(d) - 14:
            p = (p + 3) & ~3
            typ, state, mid, flags = struct.unpack_from('<IIIH', d, p)
            p += 14
            t, p = _sz_or_ord(d, p)
            if flags & 1:
                p = (p + 3) & ~3
                p += 4
            if t:
                out.append((mid, t))
        return out
    p = 4
    while p < len(d) - 2:
        flags = struct.unpack_from('<H', d, p)[0]
        p += 2
        mid = None
        if not flags & 0x10:
            mid = struct.unpack_from('<H', d, p)[0]
            p += 2
        t, p = _sz_or_ord(d, p)
        if t:
            out.append((mid, t))
    return out

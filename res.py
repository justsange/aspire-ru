"""Resource tree read / rebuild + DIALOG/MENU/STRING (de)serialisers."""
import struct
from pe import PE


# ---------------- tree ----------------
def read_tree(pe):
    """returns list of [type, name, lang, codepage, bytes] preserving order"""
    base = pe.off(pe.rsrc_rva)
    d = pe.d
    out = []
    for t, o1 in pe._dir(0):
        for n, o2 in pe._dir(o1 & 0x7fffffff):
            for lg, o3 in pe._dir(o2 & 0x7fffffff):
                rva, size, cp, _ = struct.unpack_from('<IIII', d, base + o3)
                p = pe.off(rva)
                out.append([t, n, lg, cp, d[p:p + size]])
    return out


def _key(k):
    # named entries first (sorted case-insensitively by name), then ids ascending
    return (0, k.upper()) if isinstance(k, str) else (1, k)


def build_rsrc(entries, section_rva):
    tree = {}
    for t, n, lg, cp, data in entries:
        tree.setdefault(t, {}).setdefault(n, {})[lg] = (cp, data)
    # layout: all directory tables first, then name strings, then data entries, then data
    dirs = []  # (offset placeholder) built breadth-first

    def dir_size(m):
        return 16 + 8 * len(m)

    # level offsets
    order1 = sorted(tree, key=_key)
    off = dir_size(tree)
    lvl2 = {}
    for t in order1:
        lvl2[t] = off
        off += dir_size(tree[t])
    lvl3 = {}
    for t in order1:
        for n in sorted(tree[t], key=_key):
            lvl3[(t, n)] = off
            off += dir_size(tree[t][n])
    # name strings
    names = {}
    strblob = b''
    str_base = off
    for t in order1:
        for k in [t] + list(tree[t].keys()):
            if isinstance(k, str) and k not in names:
                names[k] = str_base + len(strblob)
                enc = k.encode('utf-16le')
                strblob += struct.pack('<H', len(k)) + enc
    off = str_base + len(strblob)
    off = (off + 3) & ~3
    data_entry_base = off
    leaves = []
    for t in order1:
        for n in sorted(tree[t], key=_key):
            for lg in sorted(tree[t][n], key=_key):
                leaves.append((t, n, lg))
    off = data_entry_base + 16 * len(leaves)
    data_offs = {}
    datablob = b''
    data_base = (off + 7) & ~7
    for lf in leaves:
        cp, data = tree[lf[0]][lf[1]][lf[2]]
        data_offs[lf] = data_base + len(datablob)
        datablob += data
        while len(datablob) % 8:
            datablob += b'\0'

    def dir_hdr(m):
        nn = sum(1 for k in m if isinstance(k, str))
        ni = len(m) - nn
        return struct.pack('<IIHHHH', 0, 0, 4, 0, nn, ni)

    def ent(k, target, is_dir):
        nm = (names[k] | 0x80000000) if isinstance(k, str) else k
        return struct.pack('<II', nm, target | (0x80000000 if is_dir else 0))

    out = bytearray()
    out += dir_hdr(tree)
    for t in order1:
        out += ent(t, lvl2[t], True)
    for t in order1:
        out += dir_hdr(tree[t])
        for n in sorted(tree[t], key=_key):
            out += ent(n, lvl3[(t, n)], True)
    li = 0
    leaf_index = {lf: i for i, lf in enumerate(leaves)}
    for t in order1:
        for n in sorted(tree[t], key=_key):
            m = tree[t][n]
            out += dir_hdr(m)
            for lg in sorted(m, key=_key):
                out += ent(lg, data_entry_base + 16 * leaf_index[(t, n, lg)], False)
    assert len(out) == str_base, (len(out), str_base)
    out += strblob
    while len(out) < data_entry_base:
        out += b'\0'
    for lf in leaves:
        cp, data = tree[lf[0]][lf[1]][lf[2]]
        out += struct.pack('<IIII', section_rva + data_offs[lf], len(data), cp, 0)
    while len(out) < data_base:
        out += b'\0'
    out += datablob
    return bytes(out)


def rebuild_pe(src_path, entries, dst_path):
    pe = PE(src_path)
    d = bytearray(pe.d)
    peo = struct.unpack_from('<I', d, 0x3c)[0]
    nsec = struct.unpack_from('<H', d, peo + 6)[0]
    optsz = struct.unpack_from('<H', d, peo + 20)[0]
    opt = peo + 24
    salign, falign = struct.unpack_from('<II', d, opt + 32)
    ddir = opt + 112
    sec0 = opt + optsz
    secs = []
    for i in range(nsec):
        so = sec0 + i * 40
        name = bytes(d[so:so + 8]).rstrip(b'\0')
        vsize, va, rsize, raw = struct.unpack_from('<IIII', d, so + 8)
        secs.append(dict(so=so, name=name, vsize=vsize, va=va, rsize=rsize, raw=raw))
    ri = [i for i, s in enumerate(secs) if s['name'] == b'.rsrc'][0]
    after = secs[ri + 1:]
    assert all(s['name'] == b'.reloc' for s in after), 'only .reloc may follow .rsrc'
    rs = secs[ri]
    blob = build_rsrc(entries, rs['va'])
    up = lambda x, a: (x + a - 1) // a * a
    new_raw_size = up(len(blob), falign)
    head = bytes(d[:rs['raw']])
    body = blob + b'\0' * (new_raw_size - len(blob))
    tail = b''
    cur_raw = rs['raw'] + new_raw_size
    cur_va = up(rs['va'] + len(blob), salign)
    newsec = []
    for s in after:
        data = bytes(pe.d[s['raw']:s['raw'] + s['rsize']])
        newsec.append((s, cur_va, cur_raw))
        tail += data
        cur_raw += s['rsize']
        cur_va = up(cur_va + max(s['vsize'], s['rsize']), salign)
    out = bytearray(head + body + tail)
    # patch headers
    struct.pack_into('<IIII', out, rs['so'] + 8, len(blob), rs['va'], new_raw_size, rs['raw'])
    for s, va, raw in newsec:
        struct.pack_into('<I', out, s['so'] + 12, va)
        struct.pack_into('<I', out, s['so'] + 20, raw)
        if s['name'] == b'.reloc':
            struct.pack_into('<I', out, ddir + 5 * 8, va)
    struct.pack_into('<II', out, ddir + 2 * 8, rs['va'], len(blob))
    struct.pack_into('<I', out, opt + 56, cur_va)  # SizeOfImage
    # SizeOfInitializedData: sum of initialised-data sections' raw sizes
    init = 0
    for i in range(nsec):
        so = sec0 + i * 40
        ch = struct.unpack_from('<I', out, so + 36)[0]
        if ch & 0x40:
            init += struct.unpack_from('<I', out, so + 16)[0]
    struct.pack_into('<I', out, opt + 8, init)
    struct.pack_into('<I', out, opt + 64, 0)  # checksum
    open(dst_path, 'wb').write(out)


# ---------------- sz_or_ord raw helpers ----------------
def _rd_szord(d, p):
    w = struct.unpack_from('<H', d, p)[0]
    if w == 0xffff:
        return ('ord', d[p:p + 4]), p + 4
    e = p
    while struct.unpack_from('<H', d, e)[0]:
        e += 2
    return ('str', d[p:e].decode('utf-16le')), e + 2


def _wr_szord(v):
    if v[0] == 'ord':
        return v[1]
    return v[1].encode('utf-16le') + b'\0\0'


# ---------------- DIALOG ----------------
def parse_dialog(d):
    ex = struct.unpack_from('<HH', d, 0) == (1, 0xffff)
    if ex:
        style = struct.unpack_from('<I', d, 12)[0]
        n = struct.unpack_from('<H', d, 16)[0]
        p = 26
    else:
        style = struct.unpack_from('<I', d, 0)[0]
        n = struct.unpack_from('<H', d, 8)[0]
        p = 18
    fixed = d[:p]
    menu, p = _rd_szord(d, p)
    cls, p = _rd_szord(d, p)
    title, p = _rd_szord(d, p)
    font = b''
    if style & 0x40:
        fs = p
        p += 6 if ex else 2
        _, p = _rd_szord(d, p)
        font = d[fs:p]
    items = []
    for i in range(n):
        p = (p + 3) & ~3
        fl = 24 if ex else 18
        ifixed = d[p:p + fl]
        p += fl
        icls, p = _rd_szord(d, p)
        ititle, p = _rd_szord(d, p)
        extra = struct.unpack_from('<H', d, p)[0]
        iextra = d[p:p + 2 + extra]
        p += 2 + extra
        items.append([ifixed, icls, ititle, iextra])
    return dict(ex=ex, fixed=fixed, menu=menu, cls=cls, title=title, font=font, items=items)


def build_dialog(t):
    out = bytearray(t['fixed'])
    out += _wr_szord(t['menu']) + _wr_szord(t['cls']) + _wr_szord(t['title']) + t['font']
    for ifixed, icls, ititle, iextra in t['items']:
        while len(out) % 4:
            out += b'\0'
        out += ifixed + _wr_szord(icls) + _wr_szord(ititle) + iextra
    return bytes(out)


# ---------------- MENU ----------------
def parse_menu(d):
    ver = struct.unpack_from('<H', d, 0)[0]
    items = []
    if ver == 1:
        hoff = struct.unpack_from('<H', d, 2)[0]
        hdr = d[:4 + hoff]
        p = 4 + hoff
        while p < len(d):
            p0 = p
            typ, state, mid, flags = struct.unpack_from('<IIIH', d, p)
            pre = d[p:p + 14]
            p += 14
            txt, p = _rd_szord(d, p)
            p = (p + 3) & ~3
            post = b''
            if flags & 1:
                post = d[p:p + 4]
                p += 4
            items.append([pre, txt, post])
            if p >= len(d) - 3:
                break
        return dict(ver=1, hdr=hdr, items=items)
    hdr = d[:4]
    p = 4
    while p < len(d) - 1:
        flags = struct.unpack_from('<H', d, p)[0]
        pl = 2 if flags & 0x10 else 4
        pre = d[p:p + pl]
        p += pl
        txt, p = _rd_szord(d, p)
        items.append([pre, txt, b''])
    return dict(ver=0, hdr=hdr, items=items)


def build_menu(m):
    out = bytearray(m['hdr'])
    for pre, txt, post in m['items']:
        if m['ver'] == 1:
            while len(out) % 4:
                out += b'\0'
        out += pre + _wr_szord(txt)
        if m['ver'] == 1:
            while len(out) % 4:
                out += b'\0'
            out += post
    return bytes(out)


# ---------------- STRING ----------------
def parse_strings(d):
    out = []
    p = 0
    for i in range(16):
        l = struct.unpack_from('<H', d, p)[0]
        p += 2
        out.append(d[p:p + l * 2].decode('utf-16le'))
        p += l * 2
    return out


def build_strings(lst):
    out = b''
    for s in lst:
        e = s.encode('utf-16le')
        out += struct.pack('<H', len(e) // 2) + e
    return out

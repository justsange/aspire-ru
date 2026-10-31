import struct, math
from PIL import ImageFont
F = ImageFont.truetype('/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf', 11)  # bold metrics: Aspire draws many labels bold
PXDLU = 1.48

def geom(t, it):
    if t['ex']:
        st = struct.unpack_from('<I', it[0], 8)[0]; x, y, cx, cy = struct.unpack_from('<hhhh', it[0], 12)
    else:
        st = struct.unpack_from('<I', it[0], 0)[0]; x, y, cx, cy = struct.unpack_from('<hhhh', it[0], 8)
    c = it[1][1] if it[1][0] == 'str' else struct.unpack_from('<H', it[1][1], 2)[0]
    return st, x, y, cx, cy, c

def setg(t, it, x, cx):
    f = bytearray(it[0]); io = 12 if t['ex'] else 8
    _, y, _, cy = struct.unpack_from('<hhhh', f, io)
    struct.pack_into('<hhhh', f, io, x, y, cx, cy); it[0] = bytes(f)

def autofit(t, dlg_cx, log=None):
    G = [geom(t, it) for it in t['items']]
    changed = 0
    for i, it in enumerate(t['items']):
        st, x, y, cx, cy, c = G[i]
        txt = it[2][1] if it[2][0] == 'str' else ''
        if not txt or '\n' in txt or '\r' in txt or c not in (0x80, 0x82) or cy > 14 or cx <= 0:
            continue
        pad = 0
        if c == 0x80:
            bs = st & 0xf
            if bs in (2, 3, 4, 5, 6, 9): pad = 16
            elif bs == 7: continue
            else: pad = 8
        need = math.ceil((F.getlength(txt.replace('&', '')) + pad + 2) / PXDLU)
        if need <= cx:
            continue
        align = (st & 3) if c == 0x82 else 0
        right_lim = dlg_cx - 3
        left_lim = 2
        for j, (st2, x2, y2, cx2, cy2, c2) in enumerate(G):
            if j == i or cx2 <= 0 or cy2 <= 0:
                continue
            if y2 + cy2 <= y or y2 >= y + cy:
                continue
            encl = x2 <= x and x2 + cx2 >= x + cx and y2 <= y and y2 + cy2 >= y + cy
            if encl:
                right_lim = min(right_lim, x2 + cx2 - 3)
                left_lim = max(left_lim, x2 + 3)
                continue
            if x2 > x:
                right_lim = min(right_lim, x2 - 2)
            elif x2 + cx2 <= x + 1:
                left_lim = max(left_lim, x2 + cx2 + 2)
        if align == 2:
            nx = max(left_lim, x + cx - need)
            if nx < x:
                setg(t, it, nx, x + cx - nx); changed += 1
                if log is not None: log.append((txt, cx, x + cx - nx, need))
        else:
            ncx = min(need, right_lim - x)
            if ncx > cx:
                setg(t, it, x, ncx); changed += 1
                if log is not None: log.append((txt, cx, ncx, need))
        G[i] = geom(t, it)
    return changed

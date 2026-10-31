import sys, json, struct, re, glob, os, collections
sys.path.insert(0, '/home/claude/aspire')
from pe import PE, RT
from res import *
from mo import read_mo, write_mo

SRC = '/home/claude/aspire/src/Language'
OUT = '/home/claude/aspire/build'
os.makedirs(OUT + '/RUS/x64', exist_ok=True)
os.makedirs(OUT + '/Lang/ru_RU/LC_MESSAGES', exist_ok=True)

tr = json.load(open('/home/claude/aspire/tr/merged.json'))
dll = json.load(open('/home/claude/aspire/tr/dll_all.json'))
mo_items = json.load(open('/home/claude/aspire/tr/mo_all.json'))
en2id = {x['en']: x['id'] for x in dll}
en2ru_any = {x['en']: x['ru'] for x in dll if x['ru']}

LAT2CYR = str.maketrans('AaBCcEeHKkMOoPpTXxyu', 'АаВСсЕеНКкМОоРрТХхуи')
LAT2CYR = str.maketrans({'A': 'А', 'a': 'а', 'B': 'В', 'C': 'С', 'c': 'с', 'E': 'Е', 'e': 'е', 'H': 'Н', 'K': 'К', 'k': 'к',
                         'M': 'М', 'O': 'О', 'o': 'о', 'P': 'Р', 'p': 'р', 'T': 'Т', 'X': 'Х', 'x': 'х', 'y': 'у'})


POST = json.load(open('/home/claude/aspire/tr/postfix.json'))


def fix_homoglyphs(s):
    def f(m):
        w = m.group(0)
        if re.search('[А-Яа-яЁё]', w) and re.search('[A-Za-z]', w):
            w2 = w.translate(LAT2CYR)
            if not re.search('[A-Za-z]', w2):
                return w2
        return w
    s = re.sub(r'[A-Za-zА-Яа-яЁё]+', f, s)
    return POST.get(s, s)


def isru(s):
    return bool(s) and bool(re.search('[А-Яа-яЁё]', s))


stats = collections.Counter()


FIT = json.load(open('/home/claude/aspire/tr/fit_min.json')) if os.environ.get('NOFIT') else json.load(open('/home/claude/aspire/tr/fit_fixes.json'))
DLGI = json.load(open('/home/claude/aspire/tr/dlginit.json'))


def pick(en, cur, dlg=False):
    """en: english text at this position, cur: current RUS text at this position"""
    if dlg and isinstance(en, str) and en in FIT:
        stats['fit'] += 1
        return FIT[en]
    if not isinstance(en, str) or not re.search('[A-Za-z]{2}', en):
        return cur
    i = en2id.get(en)
    if i and i in tr:
        stats['new'] += 1
        return fix_homoglyphs(tr[i])
    if isru(cur):
        stats['kept'] += 1
        return fix_homoglyphs(cur)
    if en in en2ru_any:
        stats['borrowed'] += 1
        return fix_homoglyphs(en2ru_any[en])
    stats['english'] += 1
    return cur


# ---------- DLL ----------
ENU = {(t, n): b for t, n, lg, cp, b in read_tree(PE(SRC + '/ENU/x64/Resources_ENU.dll'))}
rus = read_tree(PE(SRC + '/RUS/x64/Resources_RUS.dll'))
mismatch = collections.Counter()
for e in rus:
    t, n, lg, cp, b = e
    tn = RT.get(t)
    eb = ENU.get((t, n))
    if t == 240 and eb is not None:
        def di(bb):
            p = 0; o = []
            while p + 2 <= len(bb):
                cid = struct.unpack_from('<H', bb, p)[0]
                if cid == 0: break
                msg, ln = struct.unpack_from('<HI', bb, p + 2); p += 8
                o.append([cid, msg, bb[p:p + ln]]); p += ln
            return o, bb[p:]
        (ri, rt), (ei, et) = di(b), di(eb)
        if len(ri) == len(ei):
            ENC = os.environ.get('DLGENC', 'cp1251')
            for r, x in zip(ri, ei):
                en_ = x[2].rstrip(b'\0').decode('latin1')
                ru_ = DLGI.get(en_) or r[2].rstrip(b'\0').decode('cp1251')
                if ENC == 'en':
                    r[2] = x[2]
                else:
                    r[2] = ru_.encode(ENC) + b'\0'
                stats['dlginit'] += 1
            e[4] = b''.join(struct.pack('<HHI', c, m, len(d)) + d for c, m, d in ri) + rt
        continue
    if eb is None or tn not in ('DIALOG', 'MENU', 'STRING'):
        continue
    if tn == 'STRING':
        R, Eg = parse_strings(b), parse_strings(eb)
        e[4] = build_strings([pick(Eg[i], R[i]) for i in range(16)])
    elif tn == 'DIALOG':
        R, Eg = parse_dialog(b), parse_dialog(eb)
        if R['title'][0] == 'str' and Eg['title'][0] == 'str':
            R['title'] = ('str', pick(Eg['title'][1], R['title'][1]))
        if len(R['items']) != len(Eg['items']):
            mismatch['dlg'] += 1
        for ri, ei in zip(R['items'], Eg['items']):
            if ri[2][0] == 'str' and ei[2][0] == 'str':
                ri[2] = ('str', pick(ei[2][1], ri[2][1], dlg=True))
        e[4] = build_dialog(R)
    elif tn == 'MENU':
        R, Eg = parse_menu(b), parse_menu(eb)
        if len(R['items']) != len(Eg['items']):
            mismatch['menu'] += 1
            print('menu mismatch', n, len(R['items']), len(Eg['items']))
            continue
        for ri, ei in zip(R['items'], Eg['items']):
            if ri[1][0] == 'str' and ei[1][0] == 'str':
                ri[1] = ('str', pick(ei[1][1], ri[1][1]))
        e[4] = build_menu(R)
WIDEN = float(os.environ.get('WIDEN', '1.0'))
if WIDEN != 1.0:
    def sc(v):
        return int(round(v * WIDEN))
    for e in rus:
        if e[0] != 5:
            continue
        t = parse_dialog(e[4])
        fx = bytearray(t['fixed'])
        ho = 18 if t['ex'] else 10
        x, y, cx, cy = struct.unpack_from('<hhhh', fx, ho)
        struct.pack_into('<hh', fx, ho, sc(x) if x > 0 else x, y)
        struct.pack_into('<h', fx, ho + 4, sc(cx))
        t['fixed'] = bytes(fx)
        for it in t['items']:
            f = bytearray(it[0])
            io = 12 if t['ex'] else 8
            x, y, cx, cy = struct.unpack_from('<hhhh', f, io)
            struct.pack_into('<hhhh', f, io, sc(x), y, max(1, sc(cx)) if cx > 0 else cx, cy)
            it[0] = bytes(f)
        e[4] = build_dialog(t)
    print('widened dialogs by', WIDEN)
# точечные правки раскладки (в единицах диалога уже после расширения)
GEOM = {185: {1025: (32, None, 48, None), 1028: (82, None, 86, None)}, 531: {1066: (None, None, 159, None), 1325: (12, None, 66, None), 4177: (80, None, 50, None), 1335: (50, None, 31, None), 2843: (50, None, 31, None), 1253: (83, None, 84, None), 2844: (83, None, 84, None)}}
from autofit import autofit
AFLOG = []
if os.environ.get('AUTOFIT'):
    for e in rus:
        if e[0] != 5:
            continue
        t = parse_dialog(e[4])
        hx = struct.unpack_from('<hhhh', t['fixed'], 18 if t['ex'] else 10)
        if autofit(t, hx[2], AFLOG):
            e[4] = build_dialog(t)
    print('autofit grew', len(AFLOG), 'controls')
for e in rus:
    if e[0] != 5 or e[1] not in GEOM:
        continue
    t = parse_dialog(e[4])
    for it in t['items']:
        f = bytearray(it[0])
        io = 12 if t['ex'] else 8
        cid = struct.unpack_from('<I' if t['ex'] else '<H', f, 20 if t['ex'] else 16)[0] & 0xffff
        g = GEOM[e[1]].get(cid)
        if g:
            cur = list(struct.unpack_from('<hhhh', f, io))
            new = [g[i] if g[i] is not None else cur[i] for i in range(4)]
            struct.pack_into('<hhhh', f, io, *new)
            it[0] = bytes(f)
    e[4] = build_dialog(t)
rebuild_pe(SRC + '/RUS/x64/Resources_RUS.dll', rus, os.environ.get('DLLOUT', OUT + '/RUS/x64/Resources_RUS.dll'))
print('DLL', dict(stats), dict(mismatch))

# ---------- MO ----------
mid2id = {}
for x in mo_items:
    key = ((x['ctx'] + '\x04') if x.get('ctx') else '') + x['en']
    if 'en_plural' in x:
        continue
    mid2id[key.encode()] = x['id']
ms = collections.Counter()
for f in sorted(glob.glob(SRC + '/Lang/ru_RU/LC_MESSAGES/*.mo')):
    ents = read_mo(f)
    new = []
    # строки, которые Aspire выводит через ANSI-преобразование: перевод превращается в «Р›РѕРєР°...»
    # подписи, которые Aspire вставляет в окна во время работы: короткие варианты, чтобы влезали
    MO_FIT = {k.encode(): v for k, v in json.load(open('/home/claude/aspire/tr/mo_form_fit.json')).items()}
    MO_KEEP_EN = set()  # 'Local' теперь ставится вариантами AppCmd.mo под кодировку системы (см. установщик)
    for a, b in ents:
        i = mid2id.get(a)
        if (os.path.basename(f), a) in MO_KEEP_EN:
            b = a
        elif a in MO_FIT:
            b = MO_FIT[a].encode()
            ms['fit'] += 1
        elif a and i in tr:
            b = fix_homoglyphs(tr[i]).encode()
            ms['new'] += 1
        elif a and b:
            b2 = fix_homoglyphs(b.decode()).encode()
            if b2 != b:
                ms['homoglyph'] += 1
            b = b2
        new.append((a, b))
    write_mo(OUT + '/Lang/ru_RU/LC_MESSAGES/' + os.path.basename(f), new)
print('MO', dict(ms))

import sys, struct
sys.path.insert(0, '/home/claude/aspire')
from res import build_rsrc

def version_res():
    def s(key, val, typ=1):
        k = (key + '\0').encode('utf-16le')
        v = (val + '\0').encode('utf-16le') if typ == 1 else val
        hdr_len = 6 + len(k)
        pad = (4 - (hdr_len % 4)) % 4
        body = k + b'\0' * pad + v
        vl = len(val) + 1 if typ == 1 else len(val)
        blk = struct.pack('<HHH', 6 + len(body), vl, typ) + body
        return blk + b'\0' * ((4 - len(blk) % 4) % 4)
    def node(key, children, value=b'', typ=0):
        k = (key + '\0').encode('utf-16le')
        hdr_len = 6 + len(k); pad = (4 - (hdr_len % 4)) % 4
        body = k + b'\0' * pad + value
        body += b'\0' * ((4 - len(body) % 4) % 4)
        body += b''.join(children)
        return struct.pack('<HHH', 6 + len(body), len(value), typ) + body
    strings = [s('CompanyName', 'A&A — мастерская форм'), s('FileDescription', 'Русификатор Vectric Aspire 12.5'),
               s('FileVersion', '2.6.5.0'), s('ProductName', 'Русификатор Aspire от A&A'), s('ProductVersion', '2.6.5'),
               s('LegalCopyright', 'Бесплатно. aa-forma.ru'), s('OriginalFilename', 'Aspire_RU_Setup.exe')]
    st = node('041904B0', strings, typ=1)
    sfi = node('StringFileInfo', [st], typ=1)
    var = node('Translation', [], value=struct.pack('<HH', 0x0419, 1200), typ=0)
    vfi = node('VarFileInfo', [var], typ=1)
    ffi = struct.pack('<13I', 0xFEEF04BD, 0x00010000, 0x00020000, 0x00060005, 0x00020000, 0x00060005, 0x3f, 0, 0x40004, 1, 0, 0, 0)
    return node('VS_VERSION_INFO', [sfi, vfi], value=ffi, typ=0)

sizes = [16, 32, 48, 64, 256]
entries = []
grp = struct.pack('<HHH', 0, 1, len(sizes))
for i, s in enumerate(sizes, 1):
    d = open(f'/home/claude/brand/ico_{s}.png', 'rb').read()
    entries.append([3, i, 1033, 0, d])
    grp += struct.pack('<BBBBHHIH', s % 256, s % 256, 0, 0, 1, 32, len(d), i)
entries.append([14, 1, 1033, 0, grp])
entries.append([24, 1, 1033, 0, open('/home/claude/brand/manifest.xml', 'rb').read()])
entries.append([16, 1, 1049, 0, version_res()])
blob = build_rsrc(entries, 0)

# find data entry offsets: walk directory
def walk(off, depth, out):
    nn, ni = struct.unpack_from('<HH', blob, off + 12)
    for i in range(nn + ni):
        name, o = struct.unpack_from('<II', blob, off + 16 + i * 8)
        if o & 0x80000000:
            walk(o & 0x7fffffff, depth + 1, out)
        else:
            out.append(o)
leaves = []
walk(0, 0, leaves)
# COFF object
nrel = len(leaves)
hdr_size = 20; sec_size = 40
raw_ptr = hdr_size + sec_size
rel_ptr = raw_ptr + len(blob)
sym_ptr = rel_ptr + nrel * 10
coff = struct.pack('<HHIIIHH', 0x8664, 1, 0, sym_ptr, 1, 0, 0)
sec = struct.pack('<8sIIIIIIHHI', b'.rsrc', 0, 0, len(blob), raw_ptr, rel_ptr, 0, nrel, 0, 0x40000040)
rels = b''.join(struct.pack('<IIH', o, 0, 3) for o in leaves)
sym = struct.pack('<8sIhHBB', b'.rsrc', 0, 1, 0, 3, 0)
strtab = struct.pack('<I', 4)
open(sys.argv[1], 'wb').write(coff + sec + blob + rels + sym + strtab)
print('syso ok', len(blob), nrel)

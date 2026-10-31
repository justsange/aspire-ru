import sys,struct,re,json;sys.path.insert(0,'/home/claude/aspire')
from pe import PE
from res import read_tree,parse_dialog
from PIL import ImageFont
F=ImageFont.truetype('/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf',11)
FB=ImageFont.truetype('/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf',11)
def W(s,font=F): return font.getlength(s.replace('&',''))
def items(path):
    out={}
    for t,n,lg,cp,b in read_tree(PE(path)):
        if t!=5: continue
        dl=parse_dialog(b)
        for i,(fx,cls,title,extra) in enumerate(dl['items']):
            if title[0]!='str' or not title[1]: continue
            if dl['ex']:
                style=struct.unpack_from('<I',fx,8)[0]; x,y,cx,cy=struct.unpack_from('<hhhh',fx,12)
            else:
                style=struct.unpack_from('<I',fx,0)[0]; x,y,cx,cy=struct.unpack_from('<hhhh',fx,8)
            c=cls[1] if cls[0]=='str' else struct.unpack_from('<H',cls[1],2)[0]
            out[(n,i)]=dict(text=title[1],style=style,cx=cx,cy=cy,cls=c)
    return out
def avail(it):
    px=it['cx']*1.55
    kind='static'
    if it['cls']==0x80:
        bs=it['style']&0xf
        if bs in (2,3,4,5,6,9): px-=16; kind='check'
        elif bs==7: px-=12; kind='group'
        else: px-=6; kind='button'
    lines=max(1,int(it['cy']*13/8/13)) if it['cy']>=16 else 1
    if kind in('button',) : lines=1
    return px,lines,kind
if __name__=='__main__':
    E=items('src/Language/ENU/x64/Resources_ENU.dll'); R=items(sys.argv[1])
    bad=[]
    for k,it in R.items():
        e=E.get(k)
        if not e or e['text']==it['text'] or it['cls'] not in (0x80,0x82): continue
        if '\n' in it['text'] or '\r' in it['text']: continue
        px,lines,kind=avail(it)
        if px<=10: continue
        we=W(e['text']); wr=W(it['text'])
        cap=px*lines*(0.85 if lines>1 else 1.0)
        if wr>cap and wr>we*1.05:
            bad.append(dict(dlg=k[0],idx=k[1],kind=kind,en=e['text'],ru=it['text'],px=round(px),lines=lines,ru_px=round(wr),en_px=round(we),maxchars=int(len(it['text'])*cap/wr)))
    json.dump(bad,open('tr/overflow.json','w'),ensure_ascii=False,indent=1)
    print(len(bad))
    import collections
    print(collections.Counter(b['kind'] for b in bad))
    # calibration: how many english overflow by same metric
    ce=sum(1 for k,e in E.items() if e['cls'] in(0x80,0x82) and '\n' not in e['text'] and avail(e)[0]>10 and W(e['text'])>avail(e)[0]*avail(e)[1]*(0.85 if avail(e)[1]>1 else 1))
    print('english overflow by metric',ce,'of',len(E))

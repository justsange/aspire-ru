import sys,re,json;sys.path.insert(0,'/home/claude/aspire')
from pe import *
W={}
def collect(path):
    p=PE(path); out={}
    for t,n,lg,b in p.resources():
        tn=RT.get(t,t)
        try:
            if tn=='STRING':
                for k,v in string_table(n,b).items(): out[('STR',k)]=v
            elif tn=='DIALOG':
                title,ts=dialog_texts(b)
                if isinstance(title,str) and title: out[('DLG',n,'title')]=title
                for i,(cid,tx,cx) in enumerate(ts): cid&=0xffff; out[('DLG',n,cid,i)]=tx; W[('DLG',n,cid,i)]=cx
            elif tn=='MENU':
                for i,(mid,tx) in enumerate(menu_texts(b)): out[('MENU',n,i)]=tx
            elif tn=='RT_XAML':
                out[('XAML',n)]=b.decode('utf-8','replace')
            elif tn=='HTML':
                out[('HTML',n)]=b.decode('utf-8','replace')
        except Exception as e:
            print('ERR',tn,n,e,file=sys.stderr)
    return out

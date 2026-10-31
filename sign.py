import hashlib,sys
SALT='53705D50AECA4A7C815A3980B670D816'
PRE='DE4F3A99A4F245B9A6D38A62D8EBF351'
def sign(path):
    d=bytearray(open(path,'rb').read())
    i=d.find(PRE.encode('utf-16le'))
    assert i>0 and d.find(PRE.encode('utf-16le'),i+1)<0
    j=i+64
    d[j:j+64]=SALT.encode('utf-16le')
    h=hashlib.md5(bytes(d)).hexdigest().upper()
    d[j:j+64]=h.encode('utf-16le')
    open(path,'wb').write(d)
    return h
def verify(path):
    d=bytearray(open(path,'rb').read()); i=d.find(PRE.encode('utf-16le'))+64
    cur=d[i:i+64].decode('utf-16le'); d[i:i+64]=SALT.encode('utf-16le')
    return cur==hashlib.md5(bytes(d)).hexdigest().upper()
if __name__=='__main__':
    for p in sys.argv[1:]: print(p, sign(p), verify(p))

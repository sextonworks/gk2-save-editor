import sys, re, dnfile
from dncil.cil.body import CilMethodBody
from dncil.cil.body.reader import CilMethodBodyReaderBase
from dncil.clr.token import Token, StringToken
S=lambda x:str(x) if x is not None else ""
pe=dnfile.dnPE(sys.argv[1]); md=pe.net.mdtables
class R(CilMethodBodyReaderBase):
    def __init__(s,pe,rva): s.pe=pe; s.off=pe.get_offset_from_rva(rva)
    def read(s,n): d=s.pe.get_data(s.pe.get_rva_from_offset(s.off),n); s.off+=n; return d
    def tell(s): return s.off
    def seek(s,o): s.off=o; return o
owner={}
for t in md.TypeDef:
    for m in t.MethodList: owner[m.row_index]=S(t.TypeName)
pat=re.compile(sys.argv[2],re.I)
for i,m in enumerate(md.MethodDef,1):
    if not m.Rva: continue
    try: body=CilMethodBody(R(pe,m.Rva))
    except Exception: continue
    for ins in body.instructions:
        if ins.opcode.name=='ldstr':
            v=ins.operand
            try: s=pe.net.user_strings.get(v.value & 0xFFFFFF).value
            except Exception: continue
            if s and pat.search(s): print(owner.get(i)+'.'+S(m.Name), repr(s))

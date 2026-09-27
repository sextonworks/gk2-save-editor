import sys, dnfile
from dncil.cil.body import CilMethodBody
from dncil.cil.body.reader import CilMethodBodyReaderBase
from dncil.clr.token import Token
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
    if S(t.TypeName) in ('ZombieTalentData','ZombieWgoData'):
        print('==',S(t.TypeName),'fields:',[S(f.row.Name) for f in t.FieldList][:60])
fields=set(sys.argv[2].split(','))
ids={i for i,f in enumerate(md.Field,1) if S(f.Name) in fields}
hits={}
for i,m in enumerate(md.MethodDef,1):
    if not m.Rva: continue
    try: body=CilMethodBody(R(pe,m.Rva))
    except Exception: continue
    for ins in body.instructions:
        if isinstance(ins.operand,Token) and (ins.operand.value>>24)==4 and (ins.operand.value&0xFFFFFF) in ids:
            hits.setdefault(owner.get(i)+'.'+S(m.Name),set()).add(ins.opcode.name+':'+S(md.Field[(ins.operand.value&0xFFFFFF)-1].Name))
for k,v in sorted(hits.items()): print(k, sorted(v))

import sys, dnfile
from dncil.cil.body import CilMethodBody
from dncil.cil.body.reader import CilMethodBodyReaderBase
from dncil.clr.token import Token, StringToken, InvalidToken
from dncil.cil.opcode import OpCodes
S=lambda x:str(x) if x is not None else ""
pe=dnfile.dnPE(sys.argv[1]); md=pe.net.mdtables
class R(CilMethodBodyReaderBase):
    def __init__(s,pe,rva): s.pe=pe; s.off=pe.get_offset_from_rva(rva)
    def read(s,n): d=s.pe.get_data(s.pe.get_rva_from_offset(s.off),n); s.off+=n; return d
    def tell(s): return s.off
    def seek(s,o): s.off=o; return o
def tok(t):
    tab={0x04:'Field',0x06:'MethodDef',0x0A:'MemberRef',0x02:'TypeDef',0x01:'TypeRef',0x2B:'MethodSpec'}.get(t>>24)
    rid=t&0xFFFFFF
    try:
        if tab=='Field': return 'F:'+S(md.Field[rid-1].Name)
        if tab=='MethodDef': return 'M:'+owner.get(rid,'?')+'.'+S(md.MethodDef[rid-1].Name)
        if tab=='MemberRef':
            r=md.MemberRef[rid-1]; c=r.Class.row; return 'MR:'+S(getattr(c,'TypeName','spec'))+'.'+S(r.Name)
        if tab=='TypeDef': return 'T:'+S(md.TypeDef[rid-1].TypeName)
        if tab=='TypeRef': return 'T:'+S(md.TypeRef[rid-1].TypeName)
        if tab=='MethodSpec':
            m=md.MethodSpec[rid-1].Method.row; return 'MS:'+S(m.Name)
    except Exception as e: return hex(t)
    if (t>>24)==0x70: return repr(pe.net.user_strings.get(rid).value)
    return hex(t)
owner={}
for t in md.TypeDef:
    for m in t.MethodList: owner[m.row_index]=S(t.TypeName)
want=[(a,b) for a in sys.argv[2].split(',') for b in [None]]
for i,m in enumerate(md.MethodDef,1):
    full=owner.get(i,'?')+'.'+S(m.Name)
    if full in sys.argv[2].split(',') and m.Rva:
        print('=====',full)
        body=CilMethodBody(R(pe,m.Rva))
        for ins in body.instructions:
            op=ins.operand
            if isinstance(op,Token): op=tok(op.value)
            print('  %04x %-12s %s'%(ins.offset-body.offset if False else ins.offset, ins.opcode.name, op if op is not None else ''))

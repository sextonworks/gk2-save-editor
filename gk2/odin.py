import struct
import sys
from pathlib import Path

PRIM = {
    15: ("b", 1),
    17: ("B", 1),
    19: ("h", 2),
    21: ("H", 2),
    23: ("i", 4),
    25: ("I", 4),
    27: ("q", 8),
    29: ("Q", 8),
    31: ("f", 4),
    33: ("d", 8),
}
PRIM.update({k + 1: v for k, v in list(PRIM.items())})


class Node:
    __slots__ = ("children", "end", "kind", "name", "off", "parent", "refid", "refoff", "type", "value", "voff")

    def __init__(self, name, kind, off, parent):
        self.name, self.kind, self.off, self.parent = name, kind, off, parent
        self.type = self.value = self.voff = self.refid = self.refoff = self.end = None
        self.children = []

    def path(self):
        p, n = [], self
        while n is not None and n.parent is not None:
            p.append(n.name if n.name is not None else "#")
            n = n.parent
        return "/".join(reversed(p))


class Reader:
    def __init__(self, data):
        self.d, self.p, self.types = data, 0, {}

    def i32(self):
        v = struct.unpack_from("<i", self.d, self.p)[0]
        self.p += 4
        return v

    def string(self):
        flag = self.d[self.p]
        self.p += 1
        n = self.i32()
        if flag == 0:
            s = self.d[self.p : self.p + n].decode("latin-1")
            self.p += n
        else:
            s = self.d[self.p : self.p + 2 * n].decode("utf-16-le", "replace")
            self.p += 2 * n
        return s

    def type_entry(self):
        t = self.d[self.p]
        self.p += 1
        if t == 47:
            i = self.i32()
            s = self.string()
            self.types[i] = s
            return s
        if t == 48:
            return self.types.get(self.i32())
        if t == 46:
            return None
        raise ValueError(f"bad type entry {t} at {self.p - 1}")

    def parse(self):
        root = Node("<root>", "root", 0, None)
        stack = [root]
        d = self.d
        while self.p < len(d):
            off = self.p
            e = d[self.p]
            self.p += 1
            if e == 49:
                break
            named = e in (1, 3, 9, 11, 13, 15, 17, 19, 21, 23, 25, 27, 29, 31, 33, 35, 37, 39, 41, 43, 45, 50)
            name = self.string() if named else None
            cur = stack[-1]
            if e in (1, 2, 3, 4):
                n = Node(name, "ref" if e < 3 else "struct", off, cur)
                n.type = self.type_entry()
                if e < 3:
                    n.refoff = self.p
                    n.refid = self.i32()
                cur.children.append(n)
                stack.append(n)
            elif e == 5:
                stack.pop().end = self.p
            elif e == 6:
                n = Node(name, "array", off, cur)
                n.value = struct.unpack_from("<q", d, self.p)[0]
                self.p += 8
                cur.children.append(n)
                stack.append(n)
            elif e == 7:
                stack.pop().end = self.p
            elif e == 8:
                cnt, sz = struct.unpack_from("<ii", d, self.p)
                self.p += 8
                n = Node(name, "primarray", off, cur)
                n.voff, n.value = self.p, (cnt, sz)
                self.p += cnt * sz
                cur.children.append(n)
            else:
                n = Node(name, "val", off, cur)
                n.voff = self.p
                if e in PRIM:
                    f, s = PRIM[e]
                    n.value = struct.unpack_from("<" + f, d, self.p)[0]
                    n.type = f
                    self.p += s
                elif e in (9, 10, 11, 12):
                    n.value = ("ref", self.i32())
                elif e in (13, 14, 41, 42, 35, 36):
                    n.value = d[self.p : self.p + 16].hex()
                    self.p += 16
                elif e in (37, 38):
                    n.value = d[self.p : self.p + 2].decode("utf-16-le")
                    self.p += 2
                elif e in (39, 40, 50, 51):
                    n.value = self.string()
                    n.type = "str"
                elif e in (43, 44):
                    n.value = bool(d[self.p])
                    n.type = "bool"
                    self.p += 1
                elif e in (45, 46):
                    n.value = None
                else:
                    raise ValueError(f"unknown entry {e} at {off}")
                cur.children.append(n)
        return root


def walk(n):
    yield n
    for c in n.children:
        yield from walk(c)


def dump(n, depth=0, maxdepth=99, out=sys.stdout):
    t = f" <{n.type}>" if n.type and n.kind != "val" else ""
    v = "" if n.kind in ("ref", "struct", "root") else f" = {n.value!r}"
    out.write(f"{'  ' * depth}{n.name if n.name is not None else '#'} [{n.kind}@{n.voff or n.off}]{t}{v}\n")
    if depth < maxdepth:
        for c in n.children:
            dump(c, depth + 1, maxdepth, out)


def load(path):
    return Reader(Path(path).read_bytes()).parse()

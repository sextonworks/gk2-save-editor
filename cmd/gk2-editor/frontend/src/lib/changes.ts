import { m } from "./paraglide/messages.js";
import { branchName } from "./branches";
import type { session } from "./wailsjs/go/models";

export function describe(c: session.Change): string {
  const p = { subject: c.subject, value: c.value };
  switch (c.kind) {
    case "money":
      return m.ch_money(p);
    case "resource":
      return m.ch_resource(p);
    case "count":
      return m.ch_count(p);
    case "add":
      return m.ch_add(p);
    case "remove":
      return m.ch_remove(p);
    case "replace":
      return m.ch_replace(p);
    case "talent":
      return m.ch_talent({ subject: branchName(c.subject), value: c.value });
    case "zombie":
      return m.ch_zombie(p);
    case "equip":
      return m.ch_equip();
    case "inspire":
      return m.ch_inspire(p);
    default:
      return m.ch_other(p);
  }
}

export function place(where: string): string {
  if (where === "bag") return m.backpack();
  if (where === "belt") return m.belt();
  return where;
}

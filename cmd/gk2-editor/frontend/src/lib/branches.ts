import { m } from "./paraglide/messages.js";

const names: Record<string, () => string> = {
  talent_orange: m.talent_orange,
  talent_red: m.talent_red,
  talent_green: m.talent_green,
  talent_yellow: m.talent_yellow,
  talent_blue: m.talent_blue,
};

export function branchName(id: string): string {
  return names[id]?.() ?? id;
}

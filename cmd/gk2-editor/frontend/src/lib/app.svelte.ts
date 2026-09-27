import * as api from "./wailsjs/go/main/App.js";
import { session } from "./wailsjs/go/models";
import { EventsOn } from "./wailsjs/runtime/runtime.js";
import { overwriteGetLocale } from "./paraglide/runtime.js";

export type Page =
  | "saves"
  | "character"
  | "inventory"
  | "storage"
  | "zombies"
  | "inspirations"
  | "technologies"
  | "inspector"
  | "backups";

export type Locale = "en" | "ru" | "zh-CN";

const localeKey = "gk2-locale";

function initialLocale(): Locale {
  try {
    const saved = localStorage.getItem(localeKey);
    if (saved === "en" || saved === "ru" || saved === "zh-CN") return saved;
  } catch {
    return "en";
  }
  const nav = navigator.language.toLowerCase();
  if (nav.startsWith("ru")) return "ru";
  if (nav.startsWith("zh")) return "zh-CN";
  return "en";
}

const gameLang: Record<Locale, string> = { en: "en", ru: "ru", "zh-CN": "zh_cn" };

export const app = $state({
  page: "saves" as Page,
  locale: initialLocale(),
  env: null as session.Environment | null,
  state: null as session.State | null,
  revision: 0,
  busy: false,
  error: "",
  notice: "",
});

export function lang(): string {
  return gameLang[app.locale];
}

overwriteGetLocale(() => app.locale);

export function changeLocale(l: Locale) {
  app.locale = l;
  void api.SetLang(gameLang[l]).then((st) => (app.state = st));
  document.documentElement.lang = l;
  try {
    localStorage.setItem(localeKey, l);
  } catch {
    return;
  }
}

function message(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

export async function run<T>(task: () => Promise<T>): Promise<T | undefined> {
  app.busy = true;
  app.error = "";
  try {
    return await task();
  } catch (err) {
    app.error = message(err);
    return undefined;
  } finally {
    app.busy = false;
  }
}

export async function edit(task: () => Promise<session.State>) {
  const st = await run(task);
  if (st) {
    app.state = st;
    app.notice = "";
    app.revision++;
  }
}

export async function refresh() {
  await run(() => api.SetLang(lang()));
  app.env = (await run(() => api.Environment())) ?? app.env;
  app.state = (await run(() => api.State())) ?? app.state;
  app.revision++;
}

export async function syncState() {
  const st = await run(() => api.State());
  if (st) {
    app.state = st;
  }
}

export function listen() {
  EventsOn("state", () => {
    void syncState();
  });
}

export { api };

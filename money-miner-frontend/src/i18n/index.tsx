// i18n — minimal hook per dossier 05: flat dotted keys, {var} interpolation,
// browser-language default (de* → de, else en), missing key renders the key.
import React, { createContext, useCallback, useContext, useMemo, useState } from "react";
import en from "./en.json";
import de from "./de.json";

export type Lang = "en" | "de";

const dicts: Record<Lang, Record<string, string>> = {
  en: en as Record<string, string>,
  de: de as Record<string, string>,
};

function browserLang(): Lang {
  const nav = typeof navigator !== "undefined" ? navigator.language : "en";
  return nav.toLowerCase().startsWith("de") ? "de" : "en";
}

interface I18nCtx {
  lang: Lang;
  setLang: (l: Lang) => void;
  t: (key: string, vars?: Record<string, string | number>) => string;
}

const Ctx = createContext<I18nCtx>({
  lang: "en",
  setLang: () => {},
  t: (k) => k,
});

export function I18nProvider({ children }: { children: React.ReactNode }) {
  const [lang, setLangState] = useState<Lang>(
    () => (localStorage.getItem("mm.lang") as Lang) || browserLang(),
  );
  const setLang = useCallback((l: Lang) => {
    localStorage.setItem("mm.lang", l);
    setLangState(l);
    document.documentElement.lang = l;
  }, []);
  const t = useCallback(
    (key: string, vars?: Record<string, string | number>) => {
      let s = dicts[lang][key] ?? dicts.en[key] ?? key; // missing key renders the key (dev-visible)
      if (vars) {
        for (const [k, v] of Object.entries(vars)) {
          s = s.replaceAll(`{${k}}`, String(v));
        }
      }
      return s;
    },
    [lang],
  );
  const value = useMemo(() => ({ lang, setLang, t }), [lang, setLang, t]);
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useT() {
  return useContext(Ctx);
}

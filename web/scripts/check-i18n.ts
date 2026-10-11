// Mechanical i18n check: the zh and en dictionaries must have exactly the same
// sections, the same keys per section, no key defined in two sections, and the
// same {placeholders} per message. Run: pnpm check:i18n (exit 1 on mismatch).
import { sections as en } from '../src/i18n/en/index.ts';
import { sections as zh } from '../src/i18n/zh/index.ts';

type Sections = Record<string, Record<string, string>>;
const problems: string[] = [];

const diff = (a: Iterable<string>, b: Set<string>) => [...a].filter((x) => !b.has(x));
const placeholders = (s: string) => new Set([...s.matchAll(/\{(\w+)\}/g)].map((m) => m[1]));

function check(a: Sections, aName: string, b: Sections, bName: string) {
  for (const s of diff(Object.keys(a), new Set(Object.keys(b)))) problems.push(`section "${s}" exists in ${aName} but not in ${bName}`);
}
check(en, 'en', zh, 'zh');
check(zh, 'zh', en, 'en');

for (const [locale, secs] of [['en', en], ['zh', zh]] as const) {
  const seen = new Map<string, string>();
  for (const [sec, dict] of Object.entries(secs as Sections)) {
    for (const k of Object.keys(dict)) {
      if (seen.has(k)) problems.push(`${locale}: key "${k}" defined in both "${seen.get(k)}" and "${sec}"`);
      seen.set(k, sec);
    }
  }
}

for (const sec of Object.keys(en)) {
  const e = (en as Sections)[sec];
  const z = (zh as Sections)[sec];
  if (!z) continue;
  for (const k of diff(Object.keys(e), new Set(Object.keys(z)))) problems.push(`${sec}: "${k}" missing in zh`);
  for (const k of diff(Object.keys(z), new Set(Object.keys(e)))) problems.push(`${sec}: "${k}" missing in en`);
  for (const k of Object.keys(e)) {
    if (!(k in z)) continue;
    const pe = placeholders(e[k]);
    const pz = placeholders(z[k]);
    if (pe.size !== pz.size || diff(pe, pz).length) problems.push(`${sec}: "${k}" placeholders differ: en {${[...pe]}} zh {${[...pz]}}`);
    if (!z[k].trim() || !e[k].trim()) problems.push(`${sec}: "${k}" is empty`);
  }
}

if (problems.length) {
  console.error(`i18n check failed (${problems.length}):\n  ` + problems.join('\n  '));
  process.exit(1);
}
const total = Object.values(en as Sections).reduce((n, d) => n + Object.keys(d).length, 0);
console.log(`i18n ok: ${total} keys in en and zh`);

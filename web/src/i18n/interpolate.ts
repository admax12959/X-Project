// Pure {name} interpolation, kept apart from the runtime in index.ts so it can be unit tested.
export type Params = Record<string, string | number>;

const PLACEHOLDER = /\{(\w+)\}/g;

/** Fills {name} placeholders from params; a placeholder without a value stays as written. */
export function interpolate(template: string, params?: Params): string {
  return template.replace(PLACEHOLDER, (m, name: string) => (params && name in params ? String(params[name]) : m));
}

/** Names of the placeholders in template that params has no value for. */
export function missingParams(template: string, params?: Params): string[] {
  return [...template.matchAll(PLACEHOLDER)].map((m) => m[1]).filter((name) => !(params && name in params));
}

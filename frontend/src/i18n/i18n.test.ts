import { describe, it, expect } from "vitest";
import en from "./en.json";
import de from "./de.json";

function flattenKeys(obj: unknown, prefix = ""): string[] {
  if (typeof obj !== "object" || obj === null) return [prefix];
  return Object.entries(obj as Record<string, unknown>).flatMap(([k, v]) =>
    flattenKeys(v, prefix ? `${prefix}.${k}` : k),
  );
}

function flattenValues(obj: unknown, prefix = ""): Record<string, unknown> {
  if (typeof obj !== "object" || obj === null) return { [prefix]: obj };
  return Object.entries(obj as Record<string, unknown>).reduce(
    (acc, [k, v]) => ({ ...acc, ...flattenValues(v, prefix ? `${prefix}.${k}` : k) }),
    {} as Record<string, unknown>,
  );
}

describe("i18n resources", () => {
  it("en and de have identical key sets", () => {
    const enKeys = flattenKeys(en).sort();
    const deKeys = flattenKeys(de).sort();
    expect(deKeys).toEqual(enKeys);
  });

  it("no value is an empty string", () => {
    for (const [lang, resource] of [
      ["en", en],
      ["de", de],
    ] as const) {
      const values = flattenValues(resource);
      for (const [key, value] of Object.entries(values)) {
        expect(value, `${lang}.${key} should not be empty`).not.toBe("");
      }
    }
  });
});

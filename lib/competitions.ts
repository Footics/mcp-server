import type { CompetitionKind } from "@/lib/types";

export const WC_SLUG = "wc-2026";
export const FRIENDLIES_SLUG = "friendlies-2026";

const SLUG_BY_KIND: Record<CompetitionKind, string> = {
  wc: WC_SLUG,
  friendlies: FRIENDLIES_SLUG,
};

const KIND_BY_SLUG: Record<string, CompetitionKind> = {
  [WC_SLUG]: "wc",
  [FRIENDLIES_SLUG]: "friendlies",
};

export function slugForKind(kind: CompetitionKind): string {
  return SLUG_BY_KIND[kind];
}

export function kindForSlug(slug: string): CompetitionKind {
  return KIND_BY_SLUG[slug] ?? "wc";
}

export function isFriendlySlug(slug: string): boolean {
  return slug === FRIENDLIES_SLUG;
}

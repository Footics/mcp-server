import { isFriendlySlug } from "@/lib/competitions";
import type { Phase } from "@/lib/types";

export type JokerBucket = "group" | "round_of_32" | "round_of_16" | "quarter_final" | "semi_final" | "final";

export function jokerBucket(phase: Phase): JokerBucket {
  return phase === "third_place" ? "final" : phase;
}

export const JOKER_QUOTA: Record<JokerBucket, number> = {
  group: 12,
  round_of_32: 4,
  round_of_16: 2,
  quarter_final: 1,
  semi_final: 1,
  final: 1,
};

export function jokerQuota(phase: Phase): number {
  return JOKER_QUOTA[jokerBucket(phase)];
}

const BUCKET_LABEL: Record<JokerBucket, string> = {
  group: "la phase de poules",
  round_of_32: "les 16es de finale",
  round_of_16: "les 8es de finale",
  quarter_final: "les quarts",
  semi_final: "les demies",
  final: "la finale",
};

export function jokerBucketLabel(phase: Phase): string {
  return BUCKET_LABEL[jokerBucket(phase)];
}

export const FRIENDLY_BUCKET = "friendly";
export const FRIENDLY_JOKER_QUOTA = 3;

export function jokerBucketFor(competitionSlug: string, phase: Phase): string {
  return isFriendlySlug(competitionSlug) ? FRIENDLY_BUCKET : jokerBucket(phase);
}

export function jokerQuotaFor(competitionSlug: string, phase: Phase): number {
  return isFriendlySlug(competitionSlug) ? FRIENDLY_JOKER_QUOTA : jokerQuota(phase);
}

export function jokerBucketLabelFor(competitionSlug: string, phase: Phase): string {
  return isFriendlySlug(competitionSlug) ? "les amicaux" : jokerBucketLabel(phase);
}

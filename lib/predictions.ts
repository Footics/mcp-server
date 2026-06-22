import "server-only";

import { and, eq } from "drizzle-orm";
import { db } from "@/db";
import { competitions, matches, predictions } from "@/db/schema";
import { jokerBucketFor, jokerBucketLabelFor, jokerQuotaFor } from "@/lib/jokers";
import type { Phase } from "@/lib/types";

export type SubmitResult =
  | { ok: true; saved: { matchId: string; home: number; away: number; joker: boolean }; locked: false }
  | { ok: false; error: string };

export async function submitPredictionFor(
  userId: string,
  input: { matchId: string; homeScore: number; awayScore: number; joker: boolean },
): Promise<SubmitResult> {
  const h = Math.trunc(input.homeScore);
  const a = Math.trunc(input.awayScore);
  if (!(h >= 0 && h <= 20 && a >= 0 && a <= 20)) return { ok: false, error: "Score invalide (0 à 20)." };

  const [m] = await db
    .select({ match: matches, slug: competitions.slug })
    .from(matches)
    .innerJoin(competitions, eq(matches.competitionId, competitions.id))
    .where(eq(matches.id, input.matchId))
    .limit(1);
  if (!m) return { ok: false, error: "Match introuvable." };

  if (m.match.status !== "scheduled" || m.match.kickoffAt.getTime() <= Date.now()) {
    return { ok: false, error: "Match verrouillé : le coup d'envoi est passé." };
  }

  const bucket = jokerBucketFor(m.slug, m.match.phase as Phase);
  if (input.joker) {
    const held = await db
      .select({ matchId: predictions.matchId, slug: competitions.slug, phase: matches.phase })
      .from(predictions)
      .innerJoin(matches, eq(predictions.matchId, matches.id))
      .innerJoin(competitions, eq(matches.competitionId, competitions.id))
      .where(and(eq(predictions.userId, userId), eq(predictions.jokerApplied, true)));
    const usedElsewhere = held.filter((j) => jokerBucketFor(j.slug, j.phase as Phase) === bucket && j.matchId !== m.match.id).length;
    if (usedElsewhere >= jokerQuotaFor(m.slug, m.match.phase as Phase)) {
      return { ok: false, error: `Plus de joker disponible pour ${jokerBucketLabelFor(m.slug, m.match.phase as Phase)} : retires-en un d'abord.` };
    }
  }

  await db
    .insert(predictions)
    .values({ userId, matchId: m.match.id, homeScore: h, awayScore: a, jokerApplied: input.joker })
    .onConflictDoUpdate({
      target: [predictions.userId, predictions.matchId],
      set: { homeScore: h, awayScore: a, jokerApplied: input.joker, updatedAt: new Date() },
    });

  return { ok: true, saved: { matchId: m.match.id, home: h, away: a, joker: input.joker }, locked: false };
}

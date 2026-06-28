import "server-only";

import { and, eq } from "drizzle-orm";
import { db } from "@/db";
import { competitions, matches, predictions } from "@/db/schema";
import { jokerBucketFor, jokerBucketLabelFor, jokerQuotaFor } from "@/lib/jokers";
import type { Phase } from "@/lib/types";

export type SubmitResult =
  | { ok: true; saved: { matchId: string; home: number; away: number; joker: boolean; winnerTeamCode: string | null; koDrawNeedsQualifier: boolean }; locked: false }
  | { ok: false; error: string };

export async function submitPredictionFor(
  userId: string,
  input: { matchId: string; homeScore: number; awayScore: number; joker: boolean; winnerTeamCode?: string | null },
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

  // Qualifié (nouvelle règle KO) : n'a de sens que sur un NUL prédit en match à
  // élimination. On NORMALISE pour ne jamais violer le CHECK predictions_winner_implies_draw :
  //  - hors nul KO → winner_team_code = null. C'ÉTAIT LE BUG : passer un nul KO déjà
  //    qualifié (winner posé via le web) à un score décisif laissait le qualifié périmé →
  //    winner ≠ null + home ≠ away → violation du CHECK. On l'efface désormais.
  //  - nul KO → on pose le qualifié fourni (validé ∈ {dom, ext}), sinon on PRÉSERVE celui
  //    déjà choisi (ex. via le web), pour ne pas le perdre en éditant juste le score.
  const isKoDraw = m.match.scoringRule === "knockout" && h === a;
  const validWinner =
    isKoDraw && input.winnerTeamCode && (input.winnerTeamCode === m.match.homeTeamCode || input.winnerTeamCode === m.match.awayTeamCode)
      ? input.winnerTeamCode
      : null;
  let winnerTeamCode: string | null = null;
  if (isKoDraw) {
    if (input.winnerTeamCode !== undefined) {
      winnerTeamCode = validWinner;
    } else {
      const [existing] = await db
        .select({ w: predictions.winnerTeamCode })
        .from(predictions)
        .where(and(eq(predictions.userId, userId), eq(predictions.matchId, m.match.id)))
        .limit(1);
      winnerTeamCode = existing?.w ?? null;
    }
  }

  await db
    .insert(predictions)
    .values({ userId, matchId: m.match.id, homeScore: h, awayScore: a, jokerApplied: input.joker, winnerTeamCode })
    .onConflictDoUpdate({
      target: [predictions.userId, predictions.matchId],
      set: { homeScore: h, awayScore: a, jokerApplied: input.joker, winnerTeamCode, updatedAt: new Date() },
    });

  return {
    ok: true,
    saved: { matchId: m.match.id, home: h, away: a, joker: input.joker, winnerTeamCode, koDrawNeedsQualifier: isKoDraw && winnerTeamCode == null },
    locked: false,
  };
}

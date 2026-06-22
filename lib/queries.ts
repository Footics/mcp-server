import "server-only";

import { aliasedTable, and, asc, desc, eq, ilike, inArray, or, sql } from "drizzle-orm";
import { db } from "@/db";
import {
  competitions,
  groupMembers,
  groups,
  matchEvents,
  matchLive,
  matches,
  predictions,
  teams,
  usernameMatches,
  users,
  userCompetitionStandings,
} from "@/db/schema";
import { slugForKind } from "@/lib/competitions";
import { jokerBucketFor, jokerBucketLabelFor, jokerQuotaFor, type JokerBucket } from "@/lib/jokers";
import type { CompetitionKind, MatchStatus, Phase } from "@/lib/types";

const competitionIdCache = new Map<string, string>();

async function competitionIdForKind(comp: CompetitionKind): Promise<string> {
  const slug = slugForKind(comp);
  const cached = competitionIdCache.get(slug);
  if (cached) return cached;
  const [c] = await db.select({ id: competitions.id }).from(competitions).where(eq(competitions.slug, slug)).limit(1);
  if (!c) throw new Error(`[mcp/queries] no competition found for slug "${slug}" (missing seed?)`);
  competitionIdCache.set(slug, c.id);
  return c.id;
}

export type MatchJson = {
  id: string;
  competition: CompetitionKind;
  phase: Phase;
  group: string | null;
  status: MatchStatus;
  kickoffAt: string;
  venue: string | null;
  home: { code: string; name: string };
  away: { code: string; name: string };
  score: { home: number; away: number } | null;
  live: { period: string; minute: number | null; running: boolean } | null;
  myPrediction: { home: number; away: number; joker: boolean; points: number | null } | null;
};

function kindForSlug(slug: string): CompetitionKind {
  return slug === slugForKind("friendlies") ? "friendlies" : "wc";
}

interface MatchRow {
  id: string;
  phase: string;
  groupLabel: string | null;
  status: string;
  kickoffAt: Date;
  venue: string | null;
  homeCode: string;
  awayCode: string;
  homeScore: number | null;
  awayScore: number | null;
  homeName: string;
  awayName: string;
  slug: string;
  pId: string | null;
  pHome: number | null;
  pAway: number | null;
  pJoker: boolean | null;
  pPoints: number | null;
  livePeriod: string | null;
  liveMinute: number | null;
  liveRunning: boolean | null;
}

function toMatchJson(r: MatchRow): MatchJson {
  const hasScore = r.homeScore != null && r.awayScore != null;
  const isLive = r.status === "live";
  const isFinished = r.status === "finished";
  return {
    id: r.id,
    competition: kindForSlug(r.slug),
    phase: r.phase as Phase,
    group: r.groupLabel ?? null,
    status: r.status as MatchStatus,
    kickoffAt: r.kickoffAt.toISOString(),
    venue: r.venue ?? null,
    home: { code: r.homeCode, name: r.homeName },
    away: { code: r.awayCode, name: r.awayName },
    score: (isLive || isFinished) && hasScore ? { home: r.homeScore as number, away: r.awayScore as number } : null,
    live: isLive && r.livePeriod ? { period: r.livePeriod, minute: r.liveMinute ?? null, running: r.liveRunning ?? false } : null,
    myPrediction: r.pId != null ? { home: r.pHome as number, away: r.pAway as number, joker: r.pJoker as boolean, points: r.pPoints ?? null } : null,
  };
}

function matchSelect(userId: string) {
  const homeT = aliasedTable(teams, "home_t");
  const awayT = aliasedTable(teams, "away_t");
  return db
    .select({
      id: matches.id,
      phase: matches.phase,
      groupLabel: matches.groupLabel,
      status: matches.status,
      kickoffAt: matches.kickoffAt,
      venue: matches.venue,
      homeCode: matches.homeTeamCode,
      awayCode: matches.awayTeamCode,
      homeScore: matches.homeScore,
      awayScore: matches.awayScore,
      homeName: homeT.name,
      awayName: awayT.name,
      slug: competitions.slug,
      pId: predictions.id,
      pHome: predictions.homeScore,
      pAway: predictions.awayScore,
      pJoker: predictions.jokerApplied,
      pPoints: predictions.pointsAwarded,
      livePeriod: matchLive.period,
      liveMinute: matchLive.displayMinute,
      liveRunning: matchLive.running,
    })
    .from(matches)
    .innerJoin(competitions, eq(matches.competitionId, competitions.id))
    .innerJoin(homeT, eq(matches.homeTeamCode, homeT.code))
    .innerJoin(awayT, eq(matches.awayTeamCode, awayT.code))
    .leftJoin(predictions, and(eq(predictions.matchId, matches.id), eq(predictions.userId, userId)))
    .leftJoin(matchLive, eq(matchLive.matchId, matches.id));
}

export async function listMatches(
  userId: string,
  comp: CompetitionKind,
  opts: { status?: MatchStatus; limit?: number } = {},
): Promise<MatchJson[]> {
  const competitionId = await competitionIdForKind(comp);
  const rows = await matchSelect(userId)
    .where(and(eq(matches.competitionId, competitionId), opts.status ? eq(matches.status, opts.status) : undefined))
    .orderBy(asc(matches.kickoffAt))
    .limit(Math.min(Math.max(opts.limit ?? 200, 1), 200));
  return rows.map((r) => toMatchJson(r));
}

export type MatchEventJson = {
  type: string;
  period: string;
  minute: number;
  minutePlus: number | null;
  teamCode: string | null;
  player: string | null;
  detail: string | null;
};

export async function getMatch(
  userId: string,
  matchId: string,
): Promise<(MatchJson & { events: MatchEventJson[] }) | null> {
  const [row] = await matchSelect(userId).where(eq(matches.id, matchId)).limit(1);
  if (!row) return null;
  const base = toMatchJson(row);
  const events =
    base.status === "scheduled"
      ? []
      : (
          await db
            .select()
            .from(matchEvents)
            .where(eq(matchEvents.matchId, matchId))
            .orderBy(asc(matchEvents.minute), asc(matchEvents.sortOrder), asc(matchEvents.createdAt))
        ).map((e) => ({
          type: e.type,
          period: e.period,
          minute: e.minute,
          minutePlus: e.minutePlus ?? null,
          teamCode: e.teamCode ?? null,
          player: e.player ?? null,
          detail: e.detail ?? null,
        }));
  return { ...base, events };
}

export type StandingJson = {
  userId: string;
  username: string;
  competition: CompetitionKind;
  points: number;
  exact: number;
  rank: number;
  rankOf: number;
};

export async function getMyStanding(userId: string, comp: CompetitionKind): Promise<StandingJson | null> {
  const competitionId = await competitionIdForKind(comp);
  const [u] = await db.select().from(users).where(eq(users.id, userId)).limit(1);
  if (!u) return null;

  const [mine] = await db
    .select({ points: userCompetitionStandings.pointsTotal, exact: userCompetitionStandings.exactCount })
    .from(userCompetitionStandings)
    .where(and(eq(userCompetitionStandings.userId, userId), eq(userCompetitionStandings.competitionId, competitionId)))
    .limit(1);
  const points = mine?.points ?? 0;
  const exact = mine?.exact ?? 0;

  const [{ ahead }] = await db
    .select({ ahead: sql<number>`count(*)::int` })
    .from(userCompetitionStandings)
    .where(and(eq(userCompetitionStandings.competitionId, competitionId), sql`${userCompetitionStandings.pointsTotal} > ${points}`));
  const [{ total }] = await db.select({ total: sql<number>`count(*)::int` }).from(users);

  return { userId: u.id, username: u.username, competition: comp, points, exact, rank: ahead + 1, rankOf: total };
}

export type PredictionJson = {
  matchId: string;
  fixture: string;
  kickoffAt: string;
  status: MatchStatus;
  prediction: { home: number; away: number; joker: boolean };
  result: { home: number; away: number } | null;
  points: number | null;
};

export async function getMyPredictions(
  userId: string,
  comp: CompetitionKind,
  opts: { when?: "upcoming" | "past" | "all"; limit?: number } = {},
): Promise<PredictionJson[]> {
  const competitionId = await competitionIdForKind(comp);
  const when = opts.when ?? "all";
  const statusCond =
    when === "upcoming" ? eq(matches.status, "scheduled") : when === "past" ? eq(matches.status, "finished") : undefined;
  const rows = await db
    .select({
      matchId: matches.id,
      home: matches.homeTeamCode,
      away: matches.awayTeamCode,
      kickoffAt: matches.kickoffAt,
      status: matches.status,
      pHome: predictions.homeScore,
      pAway: predictions.awayScore,
      joker: predictions.jokerApplied,
      points: predictions.pointsAwarded,
      mHome: matches.homeScore,
      mAway: matches.awayScore,
    })
    .from(predictions)
    .innerJoin(matches, eq(predictions.matchId, matches.id))
    .where(and(eq(predictions.userId, userId), eq(matches.competitionId, competitionId), statusCond))
    .orderBy(when === "upcoming" ? asc(matches.kickoffAt) : desc(matches.kickoffAt))
    .limit(Math.min(Math.max(opts.limit ?? 100, 1), 200));
  return rows.map((r) => ({
    matchId: r.matchId,
    fixture: `${r.home}-${r.away}`,
    kickoffAt: r.kickoffAt.toISOString(),
    status: r.status as MatchStatus,
    prediction: { home: r.pHome, away: r.pAway, joker: r.joker },
    result: r.status === "finished" && r.mHome != null && r.mAway != null ? { home: r.mHome, away: r.mAway } : null,
    points: r.points ?? null,
  }));
}

export type LeaderRowJson = { rank: number; username: string; points: number; exact: number; me: boolean };

const boardOrder = [
  desc(sql`coalesce(${userCompetitionStandings.pointsTotal}, 0)`),
  desc(sql`coalesce(${userCompetitionStandings.exactCount}, 0)`),
  asc(users.createdAt),
];

export async function getLeaderboard(
  userId: string,
  comp: CompetitionKind,
  opts: { groupId?: string; limit?: number } = {},
): Promise<LeaderRowJson[]> {
  const competitionId = await competitionIdForKind(comp);
  const base = db
    .select({
      id: users.id,
      name: users.username,
      pts: sql<number>`coalesce(${userCompetitionStandings.pointsTotal}, 0)::int`,
      exact: sql<number>`coalesce(${userCompetitionStandings.exactCount}, 0)::int`,
    })
    .from(users)
    .leftJoin(
      userCompetitionStandings,
      and(eq(userCompetitionStandings.userId, users.id), eq(userCompetitionStandings.competitionId, competitionId)),
    );

  const rows = opts.groupId
    ? await base
        .innerJoin(groupMembers, eq(groupMembers.userId, users.id))
        .where(eq(groupMembers.groupId, opts.groupId))
        .orderBy(...boardOrder)
        .limit(Math.min(Math.max(opts.limit ?? 50, 1), 200))
    : await base.orderBy(...boardOrder).limit(Math.min(Math.max(opts.limit ?? 50, 1), 200));

  return rows.map((r, i) => ({ rank: i + 1, username: r.name, points: r.pts, exact: r.exact, me: r.id === userId }));
}

export type GroupJson = { id: string; name: string; code: string; members: number; myRank: number; myPoints: number; owner: boolean };

export async function listMyGroups(userId: string, comp: CompetitionKind): Promise<GroupJson[]> {
  const competitionId = await competitionIdForKind(comp);
  const mine = await db.select({ g: groups }).from(groupMembers).innerJoin(groups, eq(groupMembers.groupId, groups.id)).where(eq(groupMembers.userId, userId));
  const out: GroupJson[] = [];
  for (const { g } of mine) {
    const board = await db
      .select({ id: users.id, pts: sql<number>`coalesce(${userCompetitionStandings.pointsTotal}, 0)::int` })
      .from(groupMembers)
      .innerJoin(users, eq(groupMembers.userId, users.id))
      .leftJoin(
        userCompetitionStandings,
        and(eq(userCompetitionStandings.userId, users.id), eq(userCompetitionStandings.competitionId, competitionId)),
      )
      .where(eq(groupMembers.groupId, g.id))
      .orderBy(...boardOrder);
    const idx = board.findIndex((r) => r.id === userId);
    out.push({
      id: g.id,
      name: g.name,
      code: g.inviteCode,
      members: board.length,
      myRank: idx >= 0 ? idx + 1 : board.length,
      myPoints: idx >= 0 ? board[idx].pts : 0,
      owner: g.ownerId === userId,
    });
  }
  return out;
}

export type JokerStatusJson = { bucket: string; label: string; used: number; quota: number; remaining: number };

export async function getJokerStatus(userId: string, comp: CompetitionKind): Promise<JokerStatusJson[]> {
  const competitionId = await competitionIdForKind(comp);
  const held = await db
    .select({ slug: competitions.slug, phase: matches.phase })
    .from(predictions)
    .innerJoin(matches, eq(predictions.matchId, matches.id))
    .innerJoin(competitions, eq(matches.competitionId, competitions.id))
    .where(and(eq(predictions.userId, userId), eq(predictions.jokerApplied, true), eq(matches.competitionId, competitionId)));

  const phases = await db
    .select({ slug: competitions.slug, phase: matches.phase })
    .from(matches)
    .innerJoin(competitions, eq(matches.competitionId, competitions.id))
    .where(eq(matches.competitionId, competitionId))
    .groupBy(competitions.slug, matches.phase);

  const used = new Map<string, number>();
  for (const h of held) {
    const b = jokerBucketFor(h.slug, h.phase as Phase);
    used.set(b, (used.get(b) ?? 0) + 1);
  }

  const seen = new Map<string, { label: string; quota: number }>();
  for (const p of phases) {
    const bucket = jokerBucketFor(p.slug, p.phase as Phase);
    if (!seen.has(bucket)) {
      seen.set(bucket, { label: jokerBucketLabelFor(p.slug, p.phase as Phase), quota: jokerQuotaFor(p.slug, p.phase as Phase) });
    }
  }

  return [...seen.entries()].map(([bucket, info]) => {
    const u = used.get(bucket) ?? 0;
    return { bucket, label: info.label, used: u, quota: info.quota, remaining: Math.max(info.quota - u, 0) };
  });
}

export type SearchJson = {
  matches: { id: string; fixture: string; kickoffAt: string }[];
  groups: { id: string; name: string }[];
  users: { id: string; username: string; me: boolean }[];
};

export async function search(userId: string, query: string): Promise<SearchJson> {
  const q = query.trim();
  if (q.length < 1) return { matches: [], groups: [], users: [] };
  const like = `%${q.replace(/[%_]/g, (c) => `\\${c}`)}%`;
  const PER = 8;
  const homeT = aliasedTable(teams, "home_t");
  const awayT = aliasedTable(teams, "away_t");

  const [matchRows, groupRows, userRows] = await Promise.all([
    db
      .select({ id: matches.id, home: matches.homeTeamCode, away: matches.awayTeamCode, kickoffAt: matches.kickoffAt })
      .from(matches)
      .innerJoin(homeT, eq(matches.homeTeamCode, homeT.code))
      .innerJoin(awayT, eq(matches.awayTeamCode, awayT.code))
      .where(or(ilike(homeT.name, like), ilike(awayT.name, like), ilike(matches.homeTeamCode, like), ilike(matches.awayTeamCode, like), ilike(matches.groupLabel, like)))
      .orderBy(asc(matches.kickoffAt))
      .limit(PER),
    db
      .select({ g: groups })
      .from(groupMembers)
      .innerJoin(groups, eq(groupMembers.groupId, groups.id))
      .where(and(eq(groupMembers.userId, userId), ilike(groups.name, like)))
      .limit(PER),
    db.select().from(users).where(ilike(users.username, like)).orderBy(asc(users.username)).limit(PER),
  ]);

  return {
    matches: matchRows.map((m) => ({ id: m.id, fixture: `${m.home}-${m.away}`, kickoffAt: m.kickoffAt.toISOString() })),
    groups: groupRows.map(({ g }) => ({ id: g.id, name: g.name })),
    users: userRows.map((u) => ({ id: u.id, username: u.username, me: u.id === userId })),
  };
}

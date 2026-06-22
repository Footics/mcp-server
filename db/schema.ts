import { relations, sql } from "drizzle-orm";
import {
  boolean,
  check,
  index,
  integer,
  pgEnum,
  pgTable,
  primaryKey,
  smallint,
  text,
  timestamp,
  uniqueIndex,
  uuid,
  varchar,
} from "drizzle-orm/pg-core";

export const phaseEnum = pgEnum("phase", [
  "group",
  "round_of_32",
  "round_of_16",
  "quarter_final",
  "semi_final",
  "third_place",
  "final",
]);

export const matchStatusEnum = pgEnum("match_status", ["scheduled", "live", "finished", "postponed", "cancelled"]);

export const scoringRuleEnum = pgEnum("scoring_rule", ["simple", "knockout"]);

export const matchPeriodEnum = pgEnum("match_period", [
  "pre",
  "first_half",
  "half_time",
  "second_half",
  "et_break",
  "et_first",
  "et_second",
  "penalties",
  "full_time",
]);

export const matchEventTypeEnum = pgEnum("match_event_type", [
  "goal",
  "own_goal",
  "penalty_goal",
  "penalty_missed",
  "yellow_card",
  "second_yellow",
  "red_card",
  "substitution",
  "var",
  "shootout_goal",
  "shootout_miss",
]);

export const userRoleEnum = pgEnum("user_role", ["member", "admin"]);

export const competitions = pgTable(
  "competitions",
  {
    id: uuid("id").defaultRandom().primaryKey(),
    slug: varchar("slug", { length: 24 }).notNull().unique(),
    label: text("label").notNull(),
    short: varchar("short", { length: 16 }).notNull(),
    emoji: varchar("emoji", { length: 16 }),
    hasPhases: boolean("has_phases").notNull().default(false),
    hasJokers: boolean("has_jokers").notNull().default(true),
    sortOrder: smallint("sort_order").notNull().default(0),
    createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [uniqueIndex("competitions_slug_idx").on(t.slug)],
);

export const teams = pgTable(
  "teams",
  {
    code: varchar("code", { length: 3 }).primaryKey(),
    name: text("name").notNull(),
    iso2: varchar("iso2", { length: 6 }).notNull(),
  },
  (t) => [
    check("teams_iso2_format", sql`${t.iso2} ~ '^[a-z]{2}(-[a-z]{2,3})?$'`),
  ],
);

export const users = pgTable(
  "users",
  {
    id: uuid("id").primaryKey(),
    username: varchar("username", { length: 24 }).notNull(),
    email: text("email").notNull(),
    avatarUrl: text("avatar_url"),
    role: userRoleEnum("role").notNull().default("member"),
    createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp("updated_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [
    uniqueIndex("users_username_lower_idx").on(sql`lower(${t.username})`),
    check("users_username_format", sql`${t.username} ~ '^[A-Za-z0-9_]{2,24}$'`),
  ],
);

export function usernameMatches(value: string) {
  return sql`lower(${users.username}) = lower(${value})`;
}

export const groups = pgTable(
  "groups",
  {
    id: uuid("id").defaultRandom().primaryKey(),
    name: varchar("name", { length: 60 }).notNull(),
    description: varchar("description", { length: 280 }),
    emoji: varchar("emoji", { length: 16 }),
    inviteCode: varchar("invite_code", { length: 6 }).notNull().unique(),
    ownerId: uuid("owner_id")
      .notNull()
      .references(() => users.id, { onDelete: "restrict" }),
    createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [
    uniqueIndex("groups_invite_code_idx").on(t.inviteCode),
    check("groups_name_len", sql`char_length(${t.name}) BETWEEN 2 AND 60`),
    check("groups_invite_code_format", sql`${t.inviteCode} ~ '^[A-HJ-NP-Z2-9]{6}$'`),
  ],
);

export const groupMembers = pgTable(
  "group_members",
  {
    groupId: uuid("group_id")
      .notNull()
      .references(() => groups.id, { onDelete: "cascade" }),
    userId: uuid("user_id")
      .notNull()
      .references(() => users.id, { onDelete: "cascade" }),
    joinedAt: timestamp("joined_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [primaryKey({ columns: [t.groupId, t.userId] }), index("group_members_user_idx").on(t.userId)],
);

export const matches = pgTable(
  "matches",
  {
    id: uuid("id").defaultRandom().primaryKey(),
    externalId: integer("external_id").notNull().unique(),
    competitionId: uuid("competition_id")
      .notNull()
      .references(() => competitions.id, { onDelete: "restrict" }),
    phase: phaseEnum("phase").notNull(),
    scoringRule: scoringRuleEnum("scoring_rule").notNull(),
    groupLabel: varchar("group_label", { length: 2 }),
    homeTeamCode: varchar("home_team_code", { length: 3 })
      .notNull()
      .references(() => teams.code, { onDelete: "restrict" }),
    awayTeamCode: varchar("away_team_code", { length: 3 })
      .notNull()
      .references(() => teams.code, { onDelete: "restrict" }),
    venue: text("venue"),
    kickoffAt: timestamp("kickoff_at", { withTimezone: true }).notNull(),

    status: matchStatusEnum("status").notNull().default("scheduled"),
    homeScore: integer("home_score"),
    awayScore: integer("away_score"),
    winnerTeamCode: varchar("winner_team_code", { length: 3 }).references(() => teams.code, { onDelete: "restrict" }),
    scoredAt: timestamp("scored_at", { withTimezone: true }),
    createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp("updated_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [
    uniqueIndex("matches_external_id_idx").on(t.externalId),
    index("matches_competition_kickoff_idx").on(t.competitionId, t.kickoffAt),
    index("matches_phase_idx").on(t.phase),
    index("matches_kickoff_idx").on(t.kickoffAt),
    index("matches_status_idx").on(t.status),
    check(
      "matches_score_check",
      sql`(${t.status} = 'finished' AND ${t.homeScore} IS NOT NULL AND ${t.awayScore} IS NOT NULL)
          OR (${t.status} != 'finished')`,
    ),
    check(
      "matches_winner_in_match",
      sql`${t.winnerTeamCode} IS NULL OR ${t.winnerTeamCode} IN (${t.homeTeamCode}, ${t.awayTeamCode})`,
    ),
    check(
      "matches_winner_ko_draw",
      sql`${t.winnerTeamCode} IS NULL OR (${t.phase} <> 'group' AND ${t.homeScore} = ${t.awayScore})`,
    ),
    check("matches_scoring_rule_phase", sql`NOT (${t.phase} <> 'group' AND ${t.scoringRule} = 'simple')`),
  ],
);

export const matchLive = pgTable(
  "match_live",
  {
    matchId: uuid("match_id")
      .primaryKey()
      .references(() => matches.id, { onDelete: "cascade" }),
    period: matchPeriodEnum("period").notNull().default("pre"),
    periodStartedAt: timestamp("period_started_at", { withTimezone: true }),
    running: boolean("running").notNull().default(false),
    addedTime: smallint("added_time"),
    displayMinute: smallint("display_minute"),
    statusNote: text("status_note"),
    updatedAt: timestamp("updated_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [
    index("match_live_period_idx").on(t.period),
    check(
      "match_live_running_period",
      sql`${t.running} = false OR ${t.period} IN ('first_half','second_half','et_first','et_second')`,
    ),
    check("match_live_running_anchor", sql`${t.running} = false OR ${t.periodStartedAt} IS NOT NULL`),
    check("match_live_added_time_positive", sql`${t.addedTime} IS NULL OR ${t.addedTime} >= 0`),
    check("match_live_display_minute_positive", sql`${t.displayMinute} IS NULL OR ${t.displayMinute} >= 0`),
  ],
);

export const matchEvents = pgTable(
  "match_events",
  {
    id: uuid("id").defaultRandom().primaryKey(),
    matchId: uuid("match_id")
      .notNull()
      .references(() => matches.id, { onDelete: "cascade" }),
    type: matchEventTypeEnum("type").notNull(),
    period: matchPeriodEnum("period").notNull(),
    minute: smallint("minute").notNull(),
    minutePlus: smallint("minute_plus"),
    teamCode: varchar("team_code", { length: 3 }).references(() => teams.code, { onDelete: "restrict" }),
    player: text("player"),
    assist: text("assist"),
    detail: text("detail"),
    sortOrder: integer("sort_order").notNull().default(0),
    dedupKey: text("dedup_key").notNull().unique(),
    createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [
    uniqueIndex("match_events_dedup_idx").on(t.dedupKey),
    index("match_events_order_idx").on(t.matchId, t.minute, t.sortOrder),
    check("match_events_minute_positive", sql`${t.minute} >= 0`),
    check("match_events_plus_positive", sql`${t.minutePlus} IS NULL OR ${t.minutePlus} >= 0`),
    check("match_events_team_required", sql`${t.teamCode} IS NOT NULL OR ${t.type} = 'var'`),
    check(
      "match_events_shootout_period",
      sql`${t.type} NOT IN ('shootout_goal','shootout_miss') OR ${t.period} = 'penalties'`,
    ),
  ],
);

export const predictions = pgTable(
  "predictions",
  {
    id: uuid("id").defaultRandom().primaryKey(),
    userId: uuid("user_id")
      .notNull()
      .references(() => users.id, { onDelete: "cascade" }),
    matchId: uuid("match_id")
      .notNull()
      .references(() => matches.id, { onDelete: "cascade" }),
    homeScore: integer("home_score").notNull(),
    awayScore: integer("away_score").notNull(),
    winnerTeamCode: varchar("winner_team_code", { length: 3 }),
    jokerApplied: boolean("joker_applied").notNull().default(false),
    pointsAwarded: integer("points_awarded"),
    createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp("updated_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [
    uniqueIndex("predictions_user_match_idx").on(t.userId, t.matchId),
    index("predictions_match_idx").on(t.matchId),
    index("predictions_user_idx").on(t.userId),
    index("predictions_joker_idx").on(t.userId, t.matchId).where(sql`${t.jokerApplied} = true`),
    check(
      "predictions_score_range_check",
      sql`${t.homeScore} >= 0 AND ${t.homeScore} <= 20 AND ${t.awayScore} >= 0 AND ${t.awayScore} <= 20`,
    ),
    check(
      "predictions_winner_implies_draw",
      sql`${t.winnerTeamCode} IS NULL OR ${t.homeScore} = ${t.awayScore}`,
    ),
  ],
);

export const userCompetitionStandings = pgTable(
  "user_competition_standings",
  {
    userId: uuid("user_id")
      .notNull()
      .references(() => users.id, { onDelete: "cascade" }),
    competitionId: uuid("competition_id")
      .notNull()
      .references(() => competitions.id, { onDelete: "cascade" }),
    pointsTotal: integer("points_total").notNull().default(0),
    exactCount: integer("exact_count").notNull().default(0),
    updatedAt: timestamp("updated_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [
    primaryKey({ columns: [t.userId, t.competitionId] }),
    index("standings_competition_rank_idx").on(t.competitionId, t.pointsTotal.desc(), t.exactCount.desc()),
  ],
);

export const competitionsRelations = relations(competitions, ({ many }) => ({
  matches: many(matches),
  standings: many(userCompetitionStandings),
}));

export const teamsRelations = relations(teams, ({ many }) => ({
  homeMatches: many(matches, { relationName: "homeTeam" }),
  awayMatches: many(matches, { relationName: "awayTeam" }),
}));

export const usersRelations = relations(users, ({ many }) => ({
  predictions: many(predictions),
  memberships: many(groupMembers),
  standings: many(userCompetitionStandings),
  ownedGroups: many(groups),
}));

export const groupsRelations = relations(groups, ({ one, many }) => ({
  owner: one(users, { fields: [groups.ownerId], references: [users.id] }),
  members: many(groupMembers),
}));

export const groupMembersRelations = relations(groupMembers, ({ one }) => ({
  group: one(groups, { fields: [groupMembers.groupId], references: [groups.id] }),
  user: one(users, { fields: [groupMembers.userId], references: [users.id] }),
}));

export const matchesRelations = relations(matches, ({ one, many }) => ({
  competition: one(competitions, { fields: [matches.competitionId], references: [competitions.id] }),
  homeTeam: one(teams, { fields: [matches.homeTeamCode], references: [teams.code], relationName: "homeTeam" }),
  awayTeam: one(teams, { fields: [matches.awayTeamCode], references: [teams.code], relationName: "awayTeam" }),
  live: one(matchLive, { fields: [matches.id], references: [matchLive.matchId] }),
  predictions: many(predictions),
  events: many(matchEvents),
}));

export const matchLiveRelations = relations(matchLive, ({ one }) => ({
  match: one(matches, { fields: [matchLive.matchId], references: [matches.id] }),
}));

export const matchEventsRelations = relations(matchEvents, ({ one }) => ({
  match: one(matches, { fields: [matchEvents.matchId], references: [matches.id] }),
  team: one(teams, { fields: [matchEvents.teamCode], references: [teams.code] }),
}));

export const predictionsRelations = relations(predictions, ({ one }) => ({
  user: one(users, { fields: [predictions.userId], references: [users.id] }),
  match: one(matches, { fields: [predictions.matchId], references: [matches.id] }),
}));

export const userCompetitionStandingsRelations = relations(userCompetitionStandings, ({ one }) => ({
  user: one(users, { fields: [userCompetitionStandings.userId], references: [users.id] }),
  competition: one(competitions, { fields: [userCompetitionStandings.competitionId], references: [competitions.id] }),
}));

export type Competition = typeof competitions.$inferSelect;
export type NewCompetition = typeof competitions.$inferInsert;
export type Team = typeof teams.$inferSelect;
export type NewTeam = typeof teams.$inferInsert;
export type User = typeof users.$inferSelect;
export type NewUser = typeof users.$inferInsert;
export type Group = typeof groups.$inferSelect;
export type GroupMember = typeof groupMembers.$inferSelect;
export type Match = typeof matches.$inferSelect;
export type NewMatch = typeof matches.$inferInsert;
export type MatchLive = typeof matchLive.$inferSelect;
export type NewMatchLive = typeof matchLive.$inferInsert;
export type MatchEvent = typeof matchEvents.$inferSelect;
export type NewMatchEvent = typeof matchEvents.$inferInsert;
export type Prediction = typeof predictions.$inferSelect;
export type NewPrediction = typeof predictions.$inferInsert;
export type UserCompetitionStanding = typeof userCompetitionStandings.$inferSelect;
export type NewUserCompetitionStanding = typeof userCompetitionStandings.$inferInsert;

import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { ENABLE_WRITES, RATE_LIMIT_PER_MIN, TEST_USER_ID } from "@/lib/config";
import { rateLimit } from "@/lib/rate-limit";
import {
  getJokerStatus,
  getLeaderboard,
  getMatch,
  getMyPredictions,
  getMyStanding,
  listMatches,
  listMyGroups,
  search,
} from "@/lib/queries";
import { submitPredictionFor } from "@/lib/predictions";

type ToolExtra = { authInfo?: { extra?: Record<string, unknown> } };
type ToolResult = { content: { type: "text"; text: string }[]; isError?: boolean };

const COMP = z.enum(["wc", "friendlies"]).default("wc").describe('Compétition : "wc" (Coupe du monde) ou "friendlies" (amicaux). Défaut: wc.');

function jsonOk(data: unknown): ToolResult {
  return { content: [{ type: "text", text: JSON.stringify(data, null, 2) }] };
}
function jsonErr(message: string): ToolResult {
  return { content: [{ type: "text", text: JSON.stringify({ error: message }) }], isError: true };
}

function identity(extra: ToolExtra): { userId: string; email: string | null } | null {
  const a = extra?.authInfo?.extra as { userId?: string; email?: string | null } | undefined;
  if (a?.userId) return { userId: a.userId, email: a.email ?? null };
  if (TEST_USER_ID) return { userId: TEST_USER_ID, email: null };
  return null;
}

function gate(extra: ToolExtra): { me: { userId: string; email: string | null } } | { err: ToolResult } {
  const me = identity(extra);
  if (!me) return { err: jsonErr("Non authentifié.") };
  const retryInSec = rateLimit(me.userId);
  if (retryInSec > 0) {
    return { err: jsonErr(`Limite de débit atteinte (${RATE_LIMIT_PER_MIN} appels/min) — attends ~${retryInSec}s avant de réessayer.`) };
  }
  return { me };
}

export function registerTools(server: McpServer): void {
  server.registerTool(
    "whoami",
    { title: "Qui suis-je", description: "Renvoie l'identité Footics de l'utilisateur connecté (id, email). Utile pour vérifier la connexion.", inputSchema: {} },
    async (_args, extra) => {
      const g = gate(extra);
      if ("err" in g) return g.err;
      const me = g.me;
      return jsonOk({ userId: me.userId, email: me.email });
    },
  );

  server.registerTool(
    "list_matches",
    {
      title: "Lister les matchs",
      description:
        "Liste les matchs d'une compétition (avec mon prono, le statut, le score live/final). Filtrable par statut. Pour voir le calendrier, les matchs à venir, en cours ou terminés.",
      inputSchema: {
        competition: COMP,
        status: z.enum(["scheduled", "live", "finished", "postponed", "cancelled"]).optional().describe("Filtre de statut optionnel."),
        limit: z.number().int().min(1).max(200).optional().describe("Nombre max de matchs (défaut 200)."),
      },
    },
    async (args, extra) => {
      const g = gate(extra);
      if ("err" in g) return g.err;
      const me = g.me;
      const a = args as { competition: "wc" | "friendlies"; status?: never; limit?: number };
      return jsonOk(await listMatches(me.userId, a.competition, { status: args.status as never, limit: a.limit }));
    },
  );

  server.registerTool(
    "get_match",
    {
      title: "Détail d'un match",
      description: "Renvoie un match par son id : équipes, coup d'envoi, statut, score, timeline des buts/cartons (si commencé) et mon prono.",
      inputSchema: { matchId: z.string().min(1).describe("L'id (uuid) du match.") },
    },
    async (args, extra) => {
      const g = gate(extra);
      if ("err" in g) return g.err;
      const me = g.me;
      const match = await getMatch(me.userId, String(args.matchId));
      return match ? jsonOk(match) : jsonErr("Match introuvable.");
    },
  );

  server.registerTool(
    "get_my_standing",
    {
      title: "Mon classement",
      description: "Mes points, scores exacts, mon rang et le nombre total de joueurs pour une compétition.",
      inputSchema: { competition: COMP },
    },
    async (args, extra) => {
      const g = gate(extra);
      if ("err" in g) return g.err;
      const me = g.me;
      const s = await getMyStanding(me.userId, args.competition as "wc" | "friendlies");
      return s ? jsonOk(s) : jsonErr("Profil introuvable.");
    },
  );

  server.registerTool(
    "get_my_predictions",
    {
      title: "Mes pronos",
      description: "Mes pronostics pour une compétition. `when` = upcoming (à venir), past (terminés) ou all (tous, défaut).",
      inputSchema: {
        competition: COMP,
        when: z.enum(["upcoming", "past", "all"]).default("all"),
        limit: z.number().int().min(1).max(200).optional(),
      },
    },
    async (args, extra) => {
      const g = gate(extra);
      if ("err" in g) return g.err;
      const me = g.me;
      const a = args as { competition: "wc" | "friendlies"; when: "upcoming" | "past" | "all"; limit?: number };
      return jsonOk(await getMyPredictions(me.userId, a.competition, { when: a.when, limit: a.limit }));
    },
  );

  server.registerTool(
    "get_leaderboard",
    {
      title: "Classement",
      description: "Classement général d'une compétition, ou d'un de mes groupes (via groupId). Trié par points puis scores exacts.",
      inputSchema: {
        competition: COMP,
        groupId: z.string().optional().describe("Id d'un groupe dont je suis membre (sinon classement général)."),
        limit: z.number().int().min(1).max(200).optional(),
      },
    },
    async (args, extra) => {
      const g = gate(extra);
      if ("err" in g) return g.err;
      const me = g.me;
      const a = args as { competition: "wc" | "friendlies"; groupId?: string; limit?: number };
      return jsonOk(await getLeaderboard(me.userId, a.competition, { groupId: a.groupId, limit: a.limit }));
    },
  );

  server.registerTool(
    "list_my_groups",
    {
      title: "Mes groupes",
      description: "Les groupes dont je suis membre, avec mon rang et mes points dans chacun pour la compétition donnée.",
      inputSchema: { competition: COMP },
    },
    async (args, extra) => {
      const g = gate(extra);
      if ("err" in g) return g.err;
      const me = g.me;
      return jsonOk(await listMyGroups(me.userId, args.competition as "wc" | "friendlies"));
    },
  );

  server.registerTool(
    "get_joker_status",
    {
      title: "Mes jokers",
      description: "Mon stock de jokers par bucket (poules, 8es, …) pour une compétition : utilisés / quota / restants. Le joker double les points d'un match.",
      inputSchema: { competition: COMP },
    },
    async (args, extra) => {
      const g = gate(extra);
      if ("err" in g) return g.err;
      const me = g.me;
      return jsonOk(await getJokerStatus(me.userId, args.competition as "wc" | "friendlies"));
    },
  );

  server.registerTool(
    "search",
    {
      title: "Rechercher",
      description: "Recherche transverse : matchs (par équipe/poule), mes groupes (par nom), joueurs (par pseudo).",
      inputSchema: { query: z.string().min(1).describe("Texte recherché.") },
    },
    async (args, extra) => {
      const g = gate(extra);
      if ("err" in g) return g.err;
      const me = g.me;
      return jsonOk(await search(me.userId, String(args.query)));
    },
  );

  if (!ENABLE_WRITES) return;
  server.registerTool(
    "submit_prediction",
    {
      title: "Poser un prono",
      description:
        "Pose ou modifie MON pronostic sur un match (scores 0-20, joker optionnel). Refusé si le coup d'envoi est passé ou si je n'ai plus de joker pour ce bucket. Sur un match à élimination directe : le prono porte sur le score à la fin du temps réglementaire (90'), et si tu prédis un NUL, précise winnerTeamCode (l'équipe qui se qualifie) pour le +1 bonus. Confirme toujours avec l'utilisateur avant d'écrire.",
      inputSchema: {
        matchId: z.string().min(1).describe("L'id (uuid) du match — voir list_matches/search."),
        homeScore: z.number().int().min(0).max(20).describe("Score prédit de l'équipe à domicile (0-20)."),
        awayScore: z.number().int().min(0).max(20).describe("Score prédit de l'équipe à l'extérieur (0-20)."),
        joker: z.boolean().default(false).describe("Appliquer un joker (double les points). Défaut: false."),
        winnerTeamCode: z
          .string()
          .min(2)
          .max(3)
          .optional()
          .describe("Match à élimination directe + nul prédit UNIQUEMENT : code FIFA-3 de l'équipe qui se qualifie (pour le +1). Doit être l'une des 2 équipes. Ignoré sur un score décisif ou en phase de poules."),
      },
    },
    async (args, extra) => {
      const g = gate(extra);
      if ("err" in g) return g.err;
      const me = g.me;
      if (!ENABLE_WRITES) {
        return jsonErr("L'écriture de pronos via MCP est désactivée sur ce serveur (MCP_ENABLE_WRITES=false).");
      }
      const a = args as { matchId: string; homeScore: number; awayScore: number; joker: boolean; winnerTeamCode?: string };
      const res = await submitPredictionFor(me.userId, { matchId: a.matchId, homeScore: a.homeScore, awayScore: a.awayScore, joker: a.joker, winnerTeamCode: a.winnerTeamCode });
      if (!res.ok) return jsonErr(res.error);
      // Nul prédit en KO sans qualifié → on invite à le préciser (sinon pas de +1).
      const note = res.saved.koDrawNeedsQualifier
        ? "Nul prédit sur un match à élimination directe : appelle à nouveau submit_prediction avec winnerTeamCode (l'équipe qui se qualifie) pour activer le +1 bonus."
        : undefined;
      return jsonOk(note ? { ok: true, saved: res.saved, note } : { ok: true, saved: res.saved });
    },
  );
}

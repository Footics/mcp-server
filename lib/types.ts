export type Phase =
  | "group"
  | "round_of_32"
  | "round_of_16"
  | "quarter_final"
  | "semi_final"
  | "third_place"
  | "final";

export type MatchStatus = "scheduled" | "live" | "finished" | "postponed" | "cancelled";

export type ScoringRule = "simple" | "knockout";

export type MatchPeriod =
  | "pre"
  | "first_half"
  | "half_time"
  | "second_half"
  | "et_break"
  | "et_first"
  | "et_second"
  | "penalties"
  | "full_time";

export type CompetitionKind = "wc" | "friendlies";

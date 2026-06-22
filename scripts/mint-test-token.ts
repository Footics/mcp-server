import { createClient } from "@supabase/supabase-js";

const url = process.env.NEXT_PUBLIC_SUPABASE_URL ?? process.env.SUPABASE_URL;
const anon = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY ?? process.env.SUPABASE_ANON_KEY;
const email = process.env.TEST_EMAIL;
const password = process.env.TEST_PASSWORD;
const mcpUrl = (process.env.MCP_PUBLIC_URL ?? "http://localhost:3000").replace(/\/$/, "") + "/mcp";

function fail(msg: string): never {
  console.error(`\n✗ ${msg}\n`);
  process.exit(1);
}

async function main(): Promise<void> {
  if (!url || !anon) fail("NEXT_PUBLIC_SUPABASE_URL / NEXT_PUBLIC_SUPABASE_ANON_KEY missing (.env.local).");
  if (!email || !password) fail("TEST_EMAIL / TEST_PASSWORD missing (.env.local) — a real account on the target project.");

  const supabase = createClient(url, anon, { auth: { persistSession: false } });

  const { data, error } = await supabase.auth.signInWithPassword({ email, password });
  if (error || !data.session) fail(`Sign-in failed: ${error?.message ?? "no session"}`);

  const token = data.session.access_token;
  const expiresIn = data.session.expires_in;

  console.log("\n✓ Supabase access token (valid ~%ds):\n", expiresIn);
  console.log(token);
  console.log("\n— List tools via MCP Inspector (CLI):\n");
  console.log(
    `npx @modelcontextprotocol/inspector --cli ${mcpUrl} \\\n` +
      `  --transport http --method tools/list \\\n` +
      `  --header "Authorization: Bearer ${token}"`,
  );
  const callCmd = (tool: string, args: string[] = []) =>
    `npx @modelcontextprotocol/inspector --cli ${mcpUrl} \\\n` +
    `  --transport http --method tools/call --tool-name ${tool} ${args.map((a) => `--tool-arg ${a} `).join("")}\\\n` +
    `  --header "Authorization: Bearer ${token}"`;

  console.log("\n— whoami (check identity):\n");
  console.log(callCmd("whoami"));
  console.log("\n— my World Cup standing:\n");
  console.log(callCmd("get_my_standing", ["competition=wc"]));
  console.log("\n— my predictions (friendlies):\n");
  console.log(callCmd("get_my_predictions", ["competition=friendlies", "when=all"]));
  console.log("");
  process.exit(0);
}

void main();

import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  turbopack: { root: process.cwd() },

  serverExternalPackages: ["postgres"],

  async rewrites() {
    return [
      { source: "/.well-known/oauth-protected-resource/mcp", destination: "/api/oauth-prm" },
      { source: "/.well-known/oauth-protected-resource", destination: "/api/oauth-prm" },
    ];
  },
};

export default nextConfig;

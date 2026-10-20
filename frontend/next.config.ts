import path from "node:path";
import { loadEnvConfig } from "@next/env";
import type { NextConfig } from "next";

// One .env at the repository root serves the frontend and the backend. Next only
// looks in its own folder and caches that result, so reload from the parent (the
// fourth argument forces it) or this call would be silently ignored.
loadEnvConfig(path.resolve(process.cwd(), ".."), process.env.NODE_ENV !== "production", console, true);

const nextConfig: NextConfig = {
  reactStrictMode: true,
  // Keep the development badge clear of the sidebar's account controls.
  devIndicators: { position: "bottom-right" },
};

export default nextConfig;

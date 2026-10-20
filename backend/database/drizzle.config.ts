import path from "node:path";
import dotenv from "dotenv";
import { defineConfig } from "drizzle-kit";

// One .env at the repository root serves the frontend and the backend.
dotenv.config({ path: path.resolve(__dirname, "../../.env") });

export default defineConfig({
  schema: "./schema.ts",
  out: "./migrations",
  dialect: "postgresql",
  dbCredentials: {
    url: process.env.DATABASE_URL ?? "postgresql://localhost:5432/sprout",
  },
  strict: true,
  verbose: true,
});

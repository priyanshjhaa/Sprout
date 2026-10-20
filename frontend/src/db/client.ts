import "server-only";

import { drizzle } from "drizzle-orm/node-postgres";
import { Pool } from "pg";
import * as schema from "./schema";

const connectionString = process.env.DATABASE_URL;

if (!connectionString) {
  throw new Error("DATABASE_URL is required to connect to PostgreSQL");
}

const globalForDatabase = globalThis as unknown as { sproutPool?: Pool };

const pool = globalForDatabase.sproutPool ?? new Pool({ connectionString });

if (process.env.NODE_ENV !== "production") {
  globalForDatabase.sproutPool = pool;
}

export const db = drizzle({ client: pool, schema });

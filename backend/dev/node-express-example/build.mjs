import { mkdir, copyFile } from "node:fs/promises";
import isNumber from "is-number";

// Uses a devDependency at build time, to prove it is installed for the build
// and left out of the release.
if (!isNumber(1)) throw new Error("is-number did not load");
await mkdir("dist", { recursive: true });
await copyFile("server.mjs", "dist/server.mjs");

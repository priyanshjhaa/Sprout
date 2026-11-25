import express from "express";

const app = express();
app.get("/health", (request, response) => response.json({ status: "ok" }));
app.get("/", (request, response) => response.type("text").send("An Express app with npm packages, ready for Sprout.\n"));

const server = app.listen(Number(process.env.PORT ?? 3000), "0.0.0.0");
process.on("SIGTERM", () => {
  server.close(() => process.exit(0));
  setTimeout(() => process.exit(1), 5000).unref();
});

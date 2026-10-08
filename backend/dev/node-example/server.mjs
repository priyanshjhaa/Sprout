import { createServer } from "node:http";

const server = createServer((request, response) => {
  if (request.url === "/health") {
    response.writeHead(200, { "Content-Type": "application/json" });
    response.end('{"status":"ok"}');
    return;
  }
  response.writeHead(200, { "Content-Type": "text/plain" });
  response.end("A small Node.js app, ready for Sprout.\n");
});

server.listen(Number(process.env.PORT ?? 3000), "0.0.0.0");
process.on("SIGTERM", () => {
  server.close(() => process.exit(0));
  setTimeout(() => process.exit(1), 5000).unref();
});

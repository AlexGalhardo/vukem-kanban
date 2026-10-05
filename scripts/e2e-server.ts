// Starts the compiled server for Playwright with a brand new database.
import { mkdirSync, rmSync } from "node:fs";

const port = process.argv[2] ?? "8765";
const dir = "test-results/e2e-db";
rmSync(dir, { recursive: true, force: true });
mkdirSync(dir, { recursive: true });

const exe = process.platform === "win32" ? "bin/kanban.exe" : "bin/kanban";
const server = Bun.spawn([exe, "--addr", `127.0.0.1:${port}`, "--db", `${dir}/kanban.db`, "--log-level", "warn"], {
	stdout: "inherit",
	stderr: "inherit",
});
const stop = () => server.kill();
process.on("SIGINT", stop);
process.on("SIGTERM", stop);
process.on("exit", stop);
process.exit(await server.exited);

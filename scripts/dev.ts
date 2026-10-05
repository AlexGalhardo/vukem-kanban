export {};

// Dev loop: Tailwind in watch mode + the Go server on :8080 with a local DB.
const css = Bun.spawn(["bun", "run", "css:watch"], { stdout: "inherit", stderr: "inherit" });
const server = Bun.spawn(["go", "run", "./cmd/kanban", "--addr", "127.0.0.1:8080", "--db", "kanban.dev.db", "--log-level", "debug"], {
	stdout: "inherit",
	stderr: "inherit",
});

const stop = () => {
	css.kill();
	server.kill();
	process.exit(0);
};
process.on("SIGINT", stop);
process.on("SIGTERM", stop);
await server.exited;
css.kill();

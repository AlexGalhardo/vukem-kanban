// Smoke test against the compiled binary: boots it on a random port with a
// throwaway database and checks the critical HTTP surface.
// Usage: bun scripts/smoke.ts  (expects bin/kanban[.exe]; `bun run smoke` builds first)
import { existsSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const exe = process.platform === "win32" ? "bin/kanban.exe" : "bin/kanban";
const startupTimeoutMs = 10_000;

type Server = Bun.Subprocess<"ignore", "pipe", "pipe">;

export async function waitForListening(server: Server, timeoutMs: number): Promise<string> {
	const reader = server.stdout.getReader();
	const decoder = new TextDecoder();
	let buffered = "";
	const deadline = Date.now() + timeoutMs;
	try {
		while (true) {
			const remaining = deadline - Date.now();
			if (remaining <= 0) throw new Error(`timeout after ${timeoutMs}ms waiting for LISTENING line`);
			let timer: ReturnType<typeof setTimeout> | undefined;
			const timeout = new Promise<"timeout">((resolve) => {
				timer = setTimeout(() => resolve("timeout"), remaining);
			});
			const chunk = await Promise.race([reader.read(), timeout]);
			clearTimeout(timer);
			if (chunk === "timeout") throw new Error(`timeout after ${timeoutMs}ms waiting for LISTENING line`);
			if (chunk.done) throw new Error(`server exited before LISTENING (stdout: ${JSON.stringify(buffered)})`);
			buffered += decoder.decode(chunk.value, { stream: true });
			const match = /^LISTENING (127\.0\.0\.1:\d+)\r?$/m.exec(buffered);
			if (match?.[1]) return match[1];
		}
	} finally {
		reader.releaseLock();
	}
}

function assert(cond: boolean, message: string): asserts cond {
	if (!cond) throw new Error(message);
}

async function check(name: string, fn: () => Promise<void>, timings: [string, number][]): Promise<void> {
	const start = performance.now();
	try {
		await fn();
	} catch (err) {
		throw new Error(`${name}: ${err instanceof Error ? err.message : String(err)}`);
	}
	timings.push([name, performance.now() - start]);
}

async function expectOK(url: string, contentType?: string): Promise<string> {
	const res = await fetch(url, { redirect: "manual" });
	assert(res.status === 200, `GET ${url} returned ${res.status}, want 200`);
	if (contentType) {
		const ct = res.headers.get("content-type") ?? "";
		assert(ct.startsWith(contentType), `GET ${url} content-type ${JSON.stringify(ct)}, want ${contentType}`);
	}
	return res.text();
}

async function main(): Promise<void> {
	assert(existsSync(exe), `${exe} not found; run \`bun run build\` first`);
	const expectedVersion = readFileSync("VERSION", "utf8").trim();
	const timings: [string, number][] = [];
	const total = performance.now();

	await check(
		"--version",
		async () => {
			const proc = Bun.spawnSync([exe, "--version"]);
			const out = proc.stdout.toString().trim();
			assert(proc.exitCode === 0, `exit code ${proc.exitCode}`);
			assert(out === expectedVersion, `printed ${JSON.stringify(out)}, want ${JSON.stringify(expectedVersion)}`);
		},
		timings,
	);

	const dir = mkdtempSync(join(tmpdir(), "kanban-smoke-"));
	let server: Server | undefined;
	try {
		let base = "";
		await check(
			"startup",
			async () => {
				server = Bun.spawn(
					[exe, "--addr", "127.0.0.1:0", "--db", join(dir, "smoke.db"), "--log-level", "warn"],
					{ stdin: "ignore", stdout: "pipe", stderr: "pipe" },
				);
				base = `http://${await waitForListening(server, startupTimeoutMs)}`;
			},
			timings,
		);

		await check(
			"GET /healthz",
			async () => {
				const body = await expectOK(`${base}/healthz`);
				assert(body.trim() === "ok", `body ${JSON.stringify(body)}, want "ok"`);
			},
			timings,
		);

		await check(
			"GET /",
			async () => {
				const res = await fetch(`${base}/`);
				assert(res.status === 200, `status ${res.status}`);
				assert((res.headers.get("content-type") ?? "").startsWith("text/html"), "content-type is not text/html");
				for (const h of ["content-security-policy", "x-content-type-options"]) {
					assert(res.headers.has(h), `missing security header ${h}`);
				}
				const body = await res.text();
				assert(body.includes("<title>"), "missing <title>");
				assert(body.includes('data-testid="empty-state"'), "missing empty state");
			},
			timings,
		);

		await check("GET /static/htmx.min.js", async () => void (await expectOK(`${base}/static/htmx.min.js`)), timings);
		await check("GET /static/app.css", async () => void (await expectOK(`${base}/static/app.css`, "text/css")), timings);

		let boardPath = "";
		await check(
			"POST /boards",
			async () => {
				const res = await fetch(`${base}/boards`, {
					method: "POST",
					body: new URLSearchParams({ name: "smoke" }),
					redirect: "manual",
				});
				assert(res.status === 303, `status ${res.status}, want 303`);
				boardPath = res.headers.get("location") ?? "";
				assert(/^\/boards\/\d+$/.test(boardPath), `location ${JSON.stringify(boardPath)}, want /boards/<id>`);
			},
			timings,
		);

		await check(
			`GET ${boardPath}`,
			async () => {
				const body = await expectOK(`${base}${boardPath}`, "text/html");
				assert(body.includes("smoke"), "board page does not contain the board name");
			},
			timings,
		);
	} finally {
		if (server) {
			server.kill();
			await server.exited;
			const stderr = await new Response(server.stderr).text();
			if (stderr.trim()) console.error(`server stderr:\n${stderr.trim()}`);
		}
		// SQLite may hold the file briefly on Windows after exit.
		rmSync(dir, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
	}

	for (const [name, ms] of timings) console.log(`  ok  ${name.padEnd(28)} ${ms.toFixed(0).padStart(5)} ms`);
	console.log(`PASS smoke (${timings.length} checks, ${(performance.now() - total).toFixed(0)} ms)`);
}

if (import.meta.main) {
	try {
		await main();
	} catch (err) {
		console.error(`FAIL smoke: ${err instanceof Error ? err.message : String(err)}`);
		process.exit(1);
	}
}

import { defineConfig, devices } from "@playwright/test";

// E2E_PORT lets parallel worktrees run their suites without clashing.
const port = Number(process.env.E2E_PORT ?? 8765);

export default defineConfig({
	outputDir: "test-results",
	fullyParallel: false,
	workers: 1,
	retries: 0,
	reporter: [["list"], ["html", { open: "never" }]],
	use: {
		baseURL: `http://127.0.0.1:${port}`,
		trace: "retain-on-failure",
		screenshot: "only-on-failure",
	},
	projects: [
		{
			name: "web",
			testDir: "e2e/web",
			use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } },
		},
		{
			name: "desktop",
			testDir: "e2e/desktop",
			timeout: 60_000,
		},
	],
	// One server per run with a fresh database; specs use unique board names
	// so they never depend on each other's data.
	webServer: {
		command: `bun scripts/e2e-server.ts ${port}`,
		url: `http://127.0.0.1:${port}/healthz`,
		reuseExistingServer: false,
		timeout: 30_000,
		stdout: "ignore",
		stderr: "pipe",
	},
});

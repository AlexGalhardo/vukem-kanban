import { defineConfig, devices } from "@playwright/test";

// QA regression specs. Run with: bunx playwright test -c e2e/qa/playwright.config.ts
// Uses its own port so it never collides with the main e2e suite.
const port = 8791;

export default defineConfig({
	testDir: ".",
	outputDir: "../../test-results/qa",
	fullyParallel: false,
	workers: 1,
	retries: 0,
	reporter: [["list"]],
	use: {
		...devices["Desktop Chrome"],
		viewport: { width: 1440, height: 900 },
		baseURL: `http://127.0.0.1:${port}`,
	},
	webServer: {
		command: `bun scripts/e2e-server.ts ${port}`,
		cwd: "../..",
		url: `http://127.0.0.1:${port}/healthz`,
		reuseExistingServer: false,
		timeout: 30_000,
		stdout: "ignore",
		stderr: "pipe",
	},
});

import { spawn, spawnSync } from "node:child_process";
import { existsSync, mkdtempSync, readFileSync, rmSync, statSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { _electron as electron, type ElectronApplication, expect, test } from "@playwright/test";

let dataDir: string;
let app: ElectronApplication;

function launchEnv(): Record<string, string> {
	return { ...(process.env as Record<string, string>), KANBAN_DATA_DIR: dataDir };
}

function isRunning(pid: number): boolean {
	if (process.platform === "win32") {
		const out = spawnSync("tasklist", ["/FI", `PID eq ${pid}`, "/FI", "IMAGENAME eq kanban.exe", "/NH"], {
			encoding: "utf8",
		}).stdout;
		return out.toLowerCase().includes("kanban.exe");
	}
	try {
		process.kill(pid, 0);
		return true;
	} catch {
		return false;
	}
}

test.beforeEach(async () => {
	dataDir = mkdtempSync(path.join(tmpdir(), "kanban-desktop-"));
	app = await electron.launch({ args: ["."], env: launchEnv() });
});

test.afterEach(async () => {
	await app.close().catch(() => undefined);
	rmSync(dataDir, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });
});

test("opens the dashboard served by the sidecar and leaves no orphan on close", async () => {
	const win = await app.firstWindow();
	await expect(win.locator('[data-testid="empty-state"], [data-testid="board-list"]').first()).toBeVisible();
	expect(win.url()).toMatch(/^http:\/\/127\.0\.0\.1:\d+\/$/);

	// No Node globals leak into the page; only the minimal preload bridge.
	const renderer = await win.evaluate(() => ({
		process: typeof (globalThis as Record<string, unknown>).process,
		require: typeof (globalThis as Record<string, unknown>).require,
		bridge: Object.keys((globalThis as Record<string, unknown>).kanbanDesktop ?? {}).sort(),
	}));
	expect(renderer).toEqual({ process: "undefined", require: "undefined", bridge: ["platform", "version"] });

	const version = await app.evaluate(({ app: electronApp }) => electronApp.getVersion());
	const pkg = JSON.parse(readFileSync("package.json", "utf8")) as { version: string };
	expect(version).toBe(pkg.version);

	const menu = await app.evaluate(({ Menu }) =>
		Menu.getApplicationMenu()?.items.map((i) => [i.label, i.submenu?.items.map((s) => s.label).filter(Boolean)]),
	);
	expect(menu?.map(([label]) => label)).toEqual(["Arquivo", "Editar", "Exibir", "Ajuda"]);
	expect(menu?.[0]?.[1]).toEqual(["Fazer backup", "Abrir pasta de dados", "Sair"]);
	expect(menu?.[3]?.[1]).toEqual(["Sobre"]);

	expect(existsSync(path.join(dataDir, "kanban.db"))).toBe(true);
	expect(existsSync(path.join(dataDir, "logs", "kanban.log"))).toBe(true);

	const res = await fetch(new URL("/backup", win.url()), { method: "POST" });
	expect(res.status).toBe(200);
	expect(existsSync((await res.text()).trim())).toBe(true);

	const pid = await app.evaluate(() => globalThis.__kanbanSidecarPid);
	expect(pid).toBeGreaterThan(0);
	expect(isRunning(pid ?? 0)).toBe(true);

	await app.close();
	await expect.poll(() => isRunning(pid ?? 0), { timeout: 5_000 }).toBe(false);
	expect(existsSync(path.join(dataDir, "window-state.json"))).toBe(true);
});

test("rotates the sidecar log when it exceeds 5 MB", async () => {
	await app.close();
	const log = path.join(dataDir, "logs", "kanban.log");
	writeFileSync(log, Buffer.alloc(6 * 1024 * 1024, "x"));
	app = await electron.launch({ args: ["."], env: launchEnv() });
	await app.firstWindow();
	expect(statSync(`${log}.1`).size).toBe(6 * 1024 * 1024);
	await expect.poll(() => statSync(log).size).toBeGreaterThan(0);
	expect(statSync(log).size).toBeLessThan(1024 * 1024);
});

test("second instance focuses the first and exits", async () => {
	await app.firstWindow();
	const exe = await app.evaluate(() => process.execPath);
	const second = spawn(exe, ["."], { env: launchEnv(), stdio: "ignore" });
	const code = await new Promise<number | null>((resolve) => second.once("exit", resolve));
	expect(code).toBe(0);
	await expect.poll(() => app.evaluate(() => globalThis.__kanbanSecondInstances)).toBe(1);
	expect(app.windows()).toHaveLength(1);
});

test("full flow inside the desktop window: project, cards, move, edit, archive", async () => {
	const win = await app.firstWindow();
	win.on("dialog", (d) => void d.accept());
	await win.getByTestId("empty-state").getByRole("button", { name: /Criar projeto/ }).click();
	await win.getByTestId("new-board-dialog").getByLabel("Nome").fill("Desktop flow");
	await win.getByTestId("new-board-dialog").getByRole("button", { name: "Criar projeto" }).click();
	await expect(win.getByTestId("board-title")).toHaveText("Desktop flow");

	const input = win.getByLabel("Novo card em Backlog", { exact: true }).last();
	await input.fill("card desktop");
	await input.press("Enter");
	const card = win.getByTestId("card").filter({ hasText: "card desktop" });
	await expect(card).toHaveCount(1);

	await card.locator(".card-open").click();
	const dialog = win.getByTestId("card-dialog");
	await dialog.getByLabel("Status").selectOption({ label: "Em progresso" });
	await dialog.getByLabel("Prioridade").selectOption({ label: "Alta" });
	await dialog.getByRole("button", { name: "Salvar alterações" }).click();
	const progress = win.getByTestId("column").filter({ has: win.getByRole("heading", { name: "Em progresso", exact: true }) });
	await expect(progress.getByTestId("card")).toContainText("Prioridade: Alta");

	await win.getByRole("button", { name: "Configurações" }).click();
	await win.getByTestId("board-settings").getByRole("button", { name: "Arquivar projeto" }).click();
	await expect(win.getByTestId("archived-list").locator("a", { hasText: "Desktop flow" })).toHaveCount(1);
});

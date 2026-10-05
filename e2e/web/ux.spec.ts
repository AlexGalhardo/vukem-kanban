import { createBoard, expect, test, uniqueName } from "./fixtures";

test("command palette navigates to a project", async ({ page }) => {
	const target = uniqueName("Destino");
	await createBoard(page, target);
	await createBoard(page, uniqueName("Origem"));
	await page.keyboard.press("Control+k");
	const input = page.locator("#command-input");
	await expect(input).toBeFocused();
	await input.fill(`ir ${target}`);
	await expect(page.locator("#command-list [role=option]")).toHaveCount(1);
	await input.press("Enter");
	await expect(page.getByTestId("board-title")).toHaveText(target);
});

test("theme toggle persists across reloads", async ({ page }) => {
	await createBoard(page, uniqueName("Tema"));
	const before = await page.evaluate(() => document.documentElement.classList.contains("dark"));
	await page.keyboard.press("t");
	await expect.poll(() => page.evaluate(() => document.documentElement.classList.contains("dark"))).toBe(!before);
	await page.reload();
	await expect.poll(() => page.evaluate(() => document.documentElement.classList.contains("dark"))).toBe(!before);
});

test("shortcuts are ignored while typing and ? opens the help", async ({ page }) => {
	await createBoard(page, uniqueName("Teclas"));
	const search = page.getByLabel("Buscar cards");
	await search.click();
	await page.keyboard.type("tn?");
	await expect(search).toHaveValue("tn?");
	await expect(page.getByTestId("new-board-dialog")).toHaveCount(0);
	await search.press("Escape");
	await page.keyboard.press("?");
	await expect(page.locator("#shortcuts-dialog")).toBeVisible();
});

test("export downloads JSON and backup shows a toast", async ({ page }) => {
	await createBoard(page, uniqueName("Dados"));
	const download = page.waitForEvent("download");
	await page.getByRole("link", { name: "Exportar dados" }).click();
	expect((await download).suggestedFilename()).toMatch(/^kanban-\d{8}-\d{6}\.json$/);
	await page.getByRole("button", { name: "Fazer backup" }).click();
	await expect(page.locator("#toasts")).toContainText("Backup salvo em");
});

test("server errors surface as toasts without breaking the page", async ({ page }) => {
	await createBoard(page, uniqueName("Erros"));
	await page.evaluate(() => {
		const btn = document.createElement("button");
		btn.id = "ghost";
		btn.textContent = "ghost";
		btn.setAttribute("hx-delete", "/cards/999999");
		document.body.append(btn);
		// @ts-expect-error htmx is a global provided by htmx.min.js
		htmx.process(btn);
	});
	await page.locator("#ghost").click();
	await expect(page.locator("#toasts")).toContainText("Não encontrado");
	await expect(page.getByTestId("board-title")).toBeVisible();
});

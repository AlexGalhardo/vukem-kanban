import { createBoard, expect, test, uniqueName } from "./fixtures";

test("create project shows it in the sidebar with the default columns", async ({ page }) => {
	const name = uniqueName("Projeto");
	await createBoard(page, name);
	await expect(page.getByTestId("board-list").getByRole("link", { name })).toHaveAttribute("aria-current", "page");
	await expect(page.getByTestId("column").getByRole("heading")).toHaveText(["Backlog", "A fazer", "Em progresso", "Revisão", "Feito"]);
	await expect(page).toHaveURL(/\/boards\/\d+$/);
});

test("validation errors render inline and keep the dialog open", async ({ page }) => {
	await page.goto("/");
	await page.keyboard.press("n");
	const dialog = page.getByTestId("new-board-dialog");
	await dialog.getByLabel("Nome").fill("   ");
	// Bypass the native `required` check to exercise the server validation.
	await dialog.locator("form[hx-post]").evaluate((f) => f.setAttribute("novalidate", ""));
	await dialog.getByRole("button", { name: "Criar projeto" }).click();
	await expect(page.getByTestId("new-board-dialog").getByText("obrigatório")).toBeVisible();
});

test("names are escaped, never executed", async ({ page }) => {
	const name = `<img src=x onerror=alert(1)> ${uniqueName("xss")}`;
	await createBoard(page, name);
	await expect(page.getByTestId("board-title")).toHaveText(name);
	await expect(page.locator("img[src=x]")).toHaveCount(0);
});

test("URL navigation and htmx navigation render the same board; back/forward work", async ({ page }) => {
	const a = uniqueName("Nav A");
	const b = uniqueName("Nav B");
	await createBoard(page, a);
	const urlA = page.url();
	await createBoard(page, b);
	await page.getByTestId("board-list").getByRole("link", { name: a }).click();
	await expect(page.getByTestId("board-title")).toHaveText(a);
	await expect(page).toHaveURL(urlA);
	await page.goBack();
	await expect(page.getByTestId("board-title")).toHaveText(b);
	await page.goForward();
	await expect(page.getByTestId("board-title")).toHaveText(a);
	await page.reload();
	await expect(page.getByTestId("board-title")).toHaveText(a);
});

test("landing reopens the last visited board", async ({ page }) => {
	const name = uniqueName("Last");
	await createBoard(page, name);
	await page.goto("/");
	await expect(page.getByTestId("board-title")).toHaveText(name);
});

test("rename, archive, restore and delete a project", async ({ page }) => {
	const name = uniqueName("Ciclo");
	const renamed = `${name} v2`;
	await createBoard(page, name);

	await page.getByRole("button", { name: "Configurações" }).click();
	let dialog = page.getByTestId("board-settings");
	await dialog.getByLabel("Nome").fill(renamed);
	await dialog.getByRole("button", { name: "Salvar" }).click();
	await expect(page.getByTestId("board-title")).toHaveText(renamed);
	await expect(page.getByTestId("board-list").getByRole("link", { name: renamed })).toBeVisible();

	page.once("dialog", (d) => d.accept());
	await page.getByRole("button", { name: "Configurações" }).click();
	await page.getByTestId("board-settings").getByRole("button", { name: "Arquivar projeto" }).click();
	await expect(page.getByTestId("board-list").getByRole("link", { name: renamed })).toHaveCount(0);
	await page.getByText(/Arquivados/).click();
	await page.getByTestId("archived-list").getByRole("link", { name: renamed }).click();
	await expect(page.getByText("Arquivado", { exact: true })).toBeVisible();
	await page.getByRole("button", { name: "Restaurar" }).first().click();
	await expect(page.getByTestId("board-list").getByRole("link", { name: renamed })).toBeVisible();

	await page.getByRole("button", { name: "Configurações" }).click();
	dialog = page.getByTestId("board-settings");
	await dialog.getByLabel(/Zona de perigo/).fill("nome errado");
	await dialog.getByRole("button", { name: /Excluir projeto/ }).click();
	await expect(page.getByTestId("board-settings").getByText("digite o nome exato do projeto")).toBeVisible();
	await page.getByTestId("board-settings").getByLabel(/Zona de perigo/).fill(renamed);
	await page.getByTestId("board-settings").getByRole("button", { name: /Excluir projeto/ }).click();
	await expect(page.getByRole("link", { name: renamed })).toHaveCount(0);
});

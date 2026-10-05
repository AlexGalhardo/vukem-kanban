import { addCard, column, columnCards, createBoard, expect, test, uniqueName } from "./fixtures";

test("rename a column, set a WIP limit and see it exceeded", async ({ page }) => {
	await createBoard(page, uniqueName("Colunas"));
	const col = column(page, "A fazer");
	await col.getByLabel("Opções da coluna A fazer").click();
	await col.getByLabel("Nome").fill("Próximos");
	await col.getByLabel(/Limite WIP/).fill("1");
	await col.getByRole("button", { name: "Salvar coluna" }).click();
	await expect(column(page, "Próximos")).toBeVisible();
	await addCard(page, "Próximos", "a");
	await addCard(page, "Próximos", "b");
	await expect(column(page, "Próximos").getByTestId("column-count")).toHaveText("2 / 1");
	await expect(column(page, "Próximos").getByTestId("column-count")).toHaveAttribute("title", "Limite WIP excedido");
	await page.reload();
	await expect(column(page, "Próximos")).toBeVisible();
});

test("add, move and delete columns", async ({ page }) => {
	await createBoard(page, uniqueName("Estrutura"));
	await page.getByLabel("Nome da nova coluna").fill("Deploy");
	await page.getByLabel("Nome da nova coluna").press("Enter");
	await expect(page.getByTestId("column").getByRole("heading").last()).toHaveText("Deploy");

	const deploy = column(page, "Deploy");
	await deploy.getByLabel("Opções da coluna Deploy").click();
	await deploy.getByRole("button", { name: "← Mover" }).click();
	await expect(page.getByTestId("column").getByRole("heading").nth(4)).toHaveText("Deploy");

	await addCard(page, "Deploy", "card a mover");
	page.once("dialog", (d) => d.accept());
	await column(page, "Deploy").getByLabel("Opções da coluna Deploy").click();
	await column(page, "Deploy").getByLabel("Se houver cards, mover para").selectOption({ label: "Feito" });
	await column(page, "Deploy").getByRole("button", { name: "Excluir coluna" }).click();
	await expect(column(page, "Deploy")).toHaveCount(0);
	await expect(columnCards(page, "Feito").filter({ hasText: "card a mover" })).toHaveCount(1);
});

test("labels: create, attach to a card, filter by label", async ({ page }) => {
	await createBoard(page, uniqueName("Labels"));
	await addCard(page, "Backlog", "com bug");
	await addCard(page, "Backlog", "sem label");

	await page.getByRole("button", { name: "Labels" }).click();
	const dialog = page.getByTestId("labels-dialog");
	await dialog.getByLabel("Nova label").fill("bug");
	await dialog.getByRole("button", { name: "Criar label" }).click();
	await expect(page.getByTestId("labels-dialog").getByLabel("Nome da label")).toHaveValue("bug");
	await page.keyboard.press("Escape");

	await columnCards(page, "Backlog").filter({ hasText: "com bug" }).locator(".card-open").click();
	const toggle = page.getByTestId("card-dialog").getByRole("button", { name: "bug" });
	await toggle.click();
	await expect(page.getByTestId("card-dialog").getByRole("button", { name: "bug" })).toHaveAttribute("aria-pressed", "true");
	await page.keyboard.press("Escape");
	await expect(columnCards(page, "Backlog").filter({ hasText: "com bug" })).toContainText("bug");

	await page.getByLabel("Filtrar por label").selectOption({ label: "bug" });
	await expect(columnCards(page, "Backlog")).toHaveCount(1);
	await expect(page).toHaveURL(/label=\d+/);
	await expect(page.getByTestId("filter-summary")).toContainText("Mostrando 1 de 2");
	await page.reload();
	await expect(columnCards(page, "Backlog")).toHaveCount(1);
	await page.getByRole("link", { name: "Limpar filtros" }).click();
	await expect(columnCards(page, "Backlog")).toHaveCount(2);
});

test("search filters as you type and keeps focus; / focuses search", async ({ page }) => {
	await createBoard(page, uniqueName("Busca"));
	await addCard(page, "Backlog", "Configurar lint");
	await addCard(page, "A fazer", "Escrever docs");
	await page.locator("body").click({ position: { x: 700, y: 700 } });
	await page.keyboard.press("/");
	const search = page.getByLabel("Buscar cards");
	await expect(search).toBeFocused();
	await search.pressSequentially("lint", { delay: 30 });
	await expect(page.getByTestId("card")).toHaveCount(1);
	await expect(search).toBeFocused();
	await expect(page).toHaveURL(/q=lint/);

	// A mutation while filtered keeps the filter applied.
	await addCard(page, "Feito", "lint final");
	await expect(page.getByTestId("card")).toHaveCount(2);
});

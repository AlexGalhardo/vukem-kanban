import { addCard, column, columnCards, createBoard, expect, test, uniqueName } from "./fixtures";

test("create, inline-rename and delete a card", async ({ page }) => {
	await createBoard(page, uniqueName("Cards"));
	await addCard(page, "Backlog", "Primeiro card");
	await addCard(page, "Backlog", "Segundo card");
	await expect(column(page, "Backlog").getByTestId("column-count")).toHaveText("2");

	const card = columnCards(page, "Backlog").filter({ hasText: "Primeiro card" });
	await card.hover();
	await card.getByRole("button", { name: /Editar título/ }).click();
	const editor = page.getByLabel("Título do card");
	await expect(editor).toBeFocused();
	await editor.fill("Card renomeado");
	await editor.press("Enter");
	const renamed = columnCards(page, "Backlog").filter({ hasText: "Card renomeado" });
	await expect(renamed).toHaveCount(1);
	await expect(renamed.locator(".card-open")).toBeFocused();

	// Esc cancels the inline editor without saving.
	await renamed.hover();
	await renamed.getByRole("button", { name: /Editar título/ }).click();
	await page.getByLabel("Título do card").fill("não salvar");
	await page.getByLabel("Título do card").press("Escape");
	await expect(columnCards(page, "Backlog").filter({ hasText: "Card renomeado" })).toHaveCount(1);

	page.once("dialog", (d) => d.accept());
	await renamed.locator(".card-open").click();
	await page.getByTestId("card-dialog").getByRole("button", { name: "Excluir card" }).click();
	await expect(columnCards(page, "Backlog")).toHaveCount(1);
	await expect(column(page, "Backlog").getByTestId("column-count")).toHaveText("1");
	await page.reload();
	await expect(columnCards(page, "Backlog")).toHaveCount(1);
});

test("edit details in the card dialog and see them on the card", async ({ page }) => {
	await createBoard(page, uniqueName("Detalhes"));
	await addCard(page, "A fazer", "Detalhar");
	await columnCards(page, "A fazer").first().locator(".card-open").click();
	const dialog = page.getByTestId("card-dialog");
	await expect(dialog).toBeVisible();
	await dialog.getByLabel("Descrição").fill("linha 1\nlinha 2");
	await dialog.getByLabel("Prioridade").selectOption({ label: "Urgente" });
	await dialog.getByLabel("Entrega").fill("2020-01-02");
	await dialog.getByLabel("Descrição").press("Control+Enter");
	await expect(page.getByTestId("card-dialog")).toHaveCount(0);

	const card = columnCards(page, "A fazer").first();
	await expect(card).toContainText("02/01/2020");
	await expect(card).toContainText("Prioridade: Urgente");
	await expect(card).toContainText("(vencido)");

	await card.locator(".card-open").click();
	await expect(page.getByTestId("card-dialog").getByLabel("Descrição")).toHaveValue("linha 1\nlinha 2");
});

test("keyboard shortcut c focuses the first new-card input", async ({ page }) => {
	await createBoard(page, uniqueName("Atalho"));
	await page.locator("body").click({ position: { x: 700, y: 600 } });
	await page.keyboard.press("c");
	await expect(page.getByLabel("Novo card em Backlog", { exact: true }).last()).toBeFocused();
	await page.keyboard.type("via atalho");
	await page.keyboard.press("Enter");
	await expect(columnCards(page, "Backlog").filter({ hasText: "via atalho" })).toHaveCount(1);
});

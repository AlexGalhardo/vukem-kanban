import { addCard, column, columnCards, createBoard, expect, test, uniqueName } from "./fixtures";

test("drag and drop moves a card between columns and persists", async ({ page }) => {
	await createBoard(page, uniqueName("DnD"));
	await addCard(page, "Backlog", "Arrastar");
	await addCard(page, "Em progresso", "Já em progresso");

	const card = columnCards(page, "Backlog").filter({ hasText: "Arrastar" });
	const target = columnCards(page, "Em progresso").first();
	await card.dragTo(target, { targetPosition: { x: 20, y: 5 } });

	await expect(columnCards(page, "Em progresso")).toHaveText([/Arrastar/, /Já em progresso/]);
	await expect(column(page, "Backlog").getByTestId("column-count")).toHaveText("0");
	await page.reload();
	await expect(columnCards(page, "Em progresso")).toHaveText([/Arrastar/, /Já em progresso/]);
});

test("drag reorders inside the same column", async ({ page }) => {
	await createBoard(page, uniqueName("Ordem"));
	for (const t of ["um", "dois", "três"]) await addCard(page, "Backlog", t);
	const last = columnCards(page, "Backlog").filter({ hasText: "três" });
	await last.dragTo(columnCards(page, "Backlog").first(), { targetPosition: { x: 20, y: 3 } });
	await expect(columnCards(page, "Backlog")).toHaveText([/três/, /um/, /dois/]);
	await page.reload();
	await expect(columnCards(page, "Backlog")).toHaveText([/três/, /um/, /dois/]);
});

test("status select in the dialog moves the card (keyboard alternative to DnD)", async ({ page }) => {
	await createBoard(page, uniqueName("Status"));
	await addCard(page, "Backlog", "Mover pelo status");
	await columnCards(page, "Backlog").first().locator(".card-open").click();
	await page.getByTestId("card-dialog").getByLabel("Status").selectOption({ label: "Feito" });
	await expect(columnCards(page, "Feito").filter({ hasText: "Mover pelo status" })).toHaveCount(1);
	await page.keyboard.press("Escape");
	await page.reload();
	await expect(columnCards(page, "Feito")).toHaveCount(1);
	await expect(columnCards(page, "Backlog")).toHaveCount(0);
});

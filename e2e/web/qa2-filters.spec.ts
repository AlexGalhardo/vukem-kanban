import { expect, type Page, test } from "@playwright/test";

// QA round 2: the board must keep matching the active filter after edits.
// Each test asserts the CORRECT behaviour and fails until the bug is fixed.

async function createBoard(page: Page, name: string): Promise<string> {
	const res = await page.request.post("/boards", { form: { name }, maxRedirects: 0 });
	const location = res.headers().location;
	if (!location) throw new Error(`no redirect creating ${name}`);
	return location;
}

async function addCards(page: Page, ...titles: string[]): Promise<void> {
	const input = page.locator("[data-new-card]").first();
	for (const t of titles) {
		await input.fill(t);
		await input.press("Enter");
		await expect(page.locator("li.card", { hasText: t })).toHaveCount(1);
	}
}

const summary = (page: Page) => page.getByTestId("filter-summary");

test("saving a card so it no longer matches the search hides it and updates the summary", async ({ page }) => {
	const url = await createBoard(page, `QA2 edit filter ${Date.now()}`);
	await page.goto(url);
	await addCards(page, "foo one", "foo two", "bar");
	await page.goto(`${url}?q=foo`);
	await expect(summary(page)).toContainText("Mostrando 2 de 3");
	await page.locator("li.card", { hasText: "foo two" }).locator(".card-open").click();
	await page.fill("#card-title", "renamed");
	await page.getByTestId("card-dialog").getByRole("button", { name: "Salvar alterações" }).click();
	await expect(page.locator("dialog[open]")).toHaveCount(0);
	await expect(page.locator("li.card", { hasText: "renamed" })).toHaveCount(0);
	await expect(summary(page)).toContainText("Mostrando 1 de 3");
});

test("inline rename out of the search hides the card", async ({ page }) => {
	const url = await createBoard(page, `QA2 inline filter ${Date.now()}`);
	await page.goto(url);
	await addCards(page, "foo inline");
	await page.goto(`${url}?q=foo`);
	const card = page.locator("li.card", { hasText: "foo inline" });
	await card.hover();
	await card.locator(".card-edit").click();
	await page.locator("textarea[data-submit-on-enter]").fill("zzz");
	await page.locator("textarea[data-submit-on-enter]").press("Enter");
	await expect(page.locator("li.card", { hasText: "zzz" })).toHaveCount(0);
	await expect(summary(page)).toContainText("Mostrando 0 de 1");
});

test("removing the filtered label from a card hides it", async ({ page }) => {
	const url = await createBoard(page, `QA2 label filter ${Date.now()}`);
	await page.goto(url);
	await addCards(page, "tagged");
	await page.locator("[data-action=labels]").click();
	await page.fill("#new-label", "bug");
	await page.getByTestId("labels-dialog").getByRole("button", { name: "Criar label" }).click();
	await expect(page.locator("#filter-label option", { hasText: "bug" })).toHaveCount(1);
	const labelId = await page.locator("#filter-label option", { hasText: "bug" }).getAttribute("value");
	await page.keyboard.press("Escape");
	await page.locator("li.card .card-open").first().click();
	await page.getByTestId("card-labels").getByRole("button", { name: "bug" }).click();
	await expect(page.getByTestId("card-labels").getByRole("button", { name: "bug" })).toHaveAttribute("aria-pressed", "true");
	await page.keyboard.press("Escape");

	await page.goto(`${url}?label=${labelId}`);
	await expect(page.locator("li.card")).toHaveCount(1);
	await page.locator("li.card .card-open").first().click();
	await page.getByTestId("card-labels").getByRole("button", { name: "bug" }).click();
	await expect(page.getByTestId("card-labels").getByRole("button", { name: "bug" })).toHaveAttribute("aria-pressed", "false");
	await page.keyboard.press("Escape");
	await expect(page.locator("li.card")).toHaveCount(0);
});

test("deleting the label used by the active filter does not leave a hidden filter on", async ({ page }) => {
	const url = await createBoard(page, `QA2 delete filter label ${Date.now()}`);
	await page.goto(url);
	await addCards(page, "visible card");
	await page.locator("[data-action=labels]").click();
	await page.fill("#new-label", "temp");
	await page.getByTestId("labels-dialog").getByRole("button", { name: "Criar label" }).click();
	await expect(page.locator("#filter-label option", { hasText: "temp" })).toHaveCount(1);
	const labelId = await page.locator("#filter-label option", { hasText: "temp" }).getAttribute("value");
	await page.keyboard.press("Escape");

	await page.goto(`${url}?label=${labelId}`);
	await page.locator("[data-action=labels]").click();
	page.once("dialog", (d) => d.accept());
	await page.getByRole("button", { name: "Excluir label temp" }).click();
	await expect(page.locator("#filter-label option", { hasText: "temp" })).toHaveCount(0);
	await page.keyboard.press("Escape");
	// The select says "Label: todas", so the board must not stay filtered by the deleted label.
	await expect(page.locator("#filter-label")).toHaveValue("");
	await expect(page.locator("li.card")).toHaveCount(1);
});

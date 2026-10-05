import { expect, type Page, test } from "@playwright/test";

// Each test asserts the CORRECT behaviour; they fail until the bug is fixed.

async function createBoard(page: Page, name: string): Promise<string> {
	const res = await page.request.post("/boards", { form: { name }, maxRedirects: 0 });
	const location = res.headers().location;
	if (!location) throw new Error(`no redirect creating ${name}`);
	return location;
}

async function collectToasts(page: Page): Promise<void> {
	await page.addInitScript(() => {
		const w = window as unknown as { __toasts: string[] };
		w.__toasts = [];
		new MutationObserver((ms) => {
			for (const m of ms) {
				for (const n of m.addedNodes) {
					if (n instanceof HTMLElement && n.classList.contains("toast")) w.__toasts.push(n.textContent ?? "");
				}
			}
		}).observe(document, { childList: true, subtree: true });
	});
}

const toasts = (page: Page): Promise<string[]> =>
	page.evaluate(() => (window as unknown as { __toasts: string[] }).__toasts);

test("sidebar navigation renders the target board's columns", async ({ page }) => {
	const a = await createBoard(page, `QA nav A ${Date.now()}`);
	await createBoard(page, `QA nav B ${Date.now()}`);
	await page.goto(a);
	await expect(page.getByTestId("column")).toHaveCount(5);
	await page.locator("[data-board-link^='QA nav B']").click();
	await expect(page).toHaveURL(/\/boards\/\d+$/);
	await expect(page.getByTestId("board-title")).toHaveText(/QA nav B/);
	await expect(page.getByTestId("column")).toHaveCount(5);
});

test("creating a project shows its columns without a reload", async ({ page }) => {
	await page.goto(await createBoard(page, `QA create ${Date.now()}`));
	await page.locator("[data-action=new-board]").click();
	await page.fill("#board-name", `QA created ${Date.now()}`);
	await page.getByTestId("new-board-dialog").locator("button[type=submit]").click();
	await expect(page.getByTestId("board-title")).toHaveText(/QA created/);
	await expect(page.getByTestId("column")).toHaveCount(5);
});

test("error toasts keep accented characters (UTF-8 in HX-Trigger)", async ({ page }) => {
	await collectToasts(page);
	await page.goto(await createBoard(page, `QA toast ${Date.now()}`));
	const input = page.locator("[data-new-card]").first();
	await input.fill("   ");
	await input.press("Enter");
	await expect.poll(() => toasts(page)).toHaveLength(1);
	expect((await toasts(page))[0]).not.toContain("Ã");
});

test("saving and deleting a card in the dialog shows a success toast", async ({ page }) => {
	await collectToasts(page);
	await page.goto(await createBoard(page, `QA dialog ${Date.now()}`));
	const input = page.locator("[data-new-card]").first();
	await input.fill("toast me");
	await input.press("Enter");
	await page.locator("li.card .card-open").first().click();
	await page.fill("#card-title", "toast me 2");
	await page.getByTestId("card-dialog").getByRole("button", { name: "Salvar alterações" }).click();
	await expect(page.locator("dialog[open]")).toHaveCount(0);
	await expect.poll(() => toasts(page)).toContainEqual(expect.stringContaining("Card salvo"));
});

test("label filter offers labels created in the labels dialog", async ({ page }) => {
	await page.goto(await createBoard(page, `QA labels ${Date.now()}`));
	await page.locator("[data-action=labels]").click();
	await page.fill("#new-label", "bug");
	await page.getByTestId("labels-dialog").getByRole("button", { name: "Criar label" }).click();
	await expect(page.locator("#filter-label option", { hasText: "bug" })).toHaveCount(1);
});

test("changing status to a deleted card does not claim success", async ({ page, request }) => {
	await collectToasts(page);
	await page.goto(await createBoard(page, `QA status ${Date.now()}`));
	const input = page.locator("[data-new-card]").first();
	await input.fill("doomed");
	await input.press("Enter");
	const card = page.locator("li.card").first();
	const id = await card.getAttribute("data-card-id");
	await card.locator(".card-open").click();
	await request.delete(`/cards/${id}`);
	await page.locator("#card-status").selectOption({ index: 2 });
	await expect.poll(() => toasts(page)).toHaveLength(1);
	expect(await toasts(page)).not.toContainEqual(expect.stringContaining("Status atualizado"));
});

test("two backups in the same second do not fail with 500", async ({ request }) => {
	const [a, b] = await Promise.all([request.post("/backup"), request.post("/backup")]);
	expect(a.status()).toBe(200);
	expect(b.status()).toBe(200);
});

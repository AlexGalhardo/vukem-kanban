import { expect, type Page, test } from "@playwright/test";

// QA round 2: stale tabs, payload size and the narrow-screen sidebar. Each
// test asserts the CORRECT behaviour and fails until the bug is fixed.

async function createBoard(page: Page, name: string): Promise<string> {
	const res = await page.request.post("/boards", { form: { name }, maxRedirects: 0 });
	const location = res.headers().location;
	if (!location) throw new Error(`no redirect creating ${name}`);
	return location;
}

test("filtering a board deleted in another tab tells the user it is gone", async ({ page }) => {
	const name = `QA2 ghost ${Date.now()}`;
	const url = await createBoard(page, name);
	await page.goto(url);
	const id = url.split("/").pop();
	const del = await page.request.delete(`/boards/${id}?confirm_name=${encodeURIComponent(name)}`, { maxRedirects: 0 });
	expect(del.status()).toBe(303);
	await page.locator("#search").fill("anything");
	// Today the ghost board stays on screen with a generic toast.
	await expect(page.getByText("Projeto não encontrado")).toBeVisible();
});

test("opening the labels dialog does not re-send the whole board", async ({ page }) => {
	const url = await createBoard(page, `QA2 labels payload ${Date.now()}`);
	await page.goto(url);
	const res = page.waitForResponse((r) => r.url().endsWith("/labels") && r.request().method() === "GET");
	await page.locator("[data-action=labels]").click();
	const body = await (await res).text();
	// The board did not change; shipping #columns costs ~2.8 MB on a 2000-card board.
	expect(body).not.toContain('id="columns"');
});

test("the narrow-screen sidebar overlay takes keyboard focus", async ({ page }) => {
	await page.setViewportSize({ width: 390, height: 800 });
	await page.goto(await createBoard(page, `QA2 narrow ${Date.now()}`));
	await page.locator("main [data-action=sidebar]").click();
	await expect(page.locator("#sidebar")).toBeVisible();
	await page.keyboard.press("Tab");
	const inSidebar = await page.evaluate(() => Boolean(document.activeElement?.closest("#sidebar")));
	expect(inSidebar).toBe(true);
});

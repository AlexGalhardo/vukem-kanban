import http from "node:http";
import { expect, type Page, test } from "@playwright/test";

// QA round 2: data loss / wrong data. Each test asserts the CORRECT behaviour
// and fails until the bug is fixed.

async function createBoard(page: Page, name: string): Promise<string> {
	const res = await page.request.post("/boards", { form: { name }, maxRedirects: 0 });
	const location = res.headers().location;
	if (!location) throw new Error(`no redirect creating ${name}`);
	return location;
}

const cardTitles = (page: Page) => page.locator("li.card [data-testid=card-title]");

test("typing the next card title right after Enter is not lost or truncated", async ({ page }) => {
	await page.goto(await createBoard(page, `QA2 typing ${Date.now()}`));
	// A slow first response makes the race deterministic: the user starts the
	// next title while "alpha" is still in flight. Today form.reset() on its
	// success wipes what was typed, and hx-sync drop discards an early Enter.
	let first = true;
	await page.route("**/columns/*/cards", async (route) => {
		if (first) {
			first = false;
			await new Promise((r) => setTimeout(r, 400));
		}
		await route.continue();
	});
	const input = page.locator("[data-new-card]").first();
	await input.fill("alpha");
	await input.press("Enter");
	await input.pressSequentially("be", { delay: 20 });
	await page.waitForTimeout(600);
	await input.pressSequentially("ta", { delay: 20 });
	await input.press("Enter");
	await expect(cardTitles(page)).toHaveText(["alpha", "beta"]);
	await page.reload();
	await expect(cardTitles(page)).toHaveText(["alpha", "beta"]);
});

test("pressing Enter on two different column names in a row creates both", async ({ page }) => {
	await page.goto(await createBoard(page, `QA2 columns ${Date.now()}`));
	// Delay the first response so the second Enter lands while it is in flight
	// (hx-sync="this:drop" silently discards it and the #columns swap clears the field).
	let first = true;
	await page.route("**/boards/*/columns", async (route) => {
		if (first) {
			first = false;
			await new Promise((r) => setTimeout(r, 400));
		}
		await route.continue();
	});
	const input = page.locator("#new-column");
	await input.fill("k1");
	await input.press("Enter");
	await input.fill("k2");
	await input.press("Enter");
	await page.waitForLoadState("networkidle");
	await page.reload();
	await expect(page.getByTestId("column").locator("h2")).toContainText(["k1", "k2"]);
});

test("a drag that cannot reach the server puts the card back where it was", async ({ page }) => {
	await page.goto(await createBoard(page, `QA2 offline drag ${Date.now()}`));
	const input = page.locator("[data-new-card]").first();
	await input.fill("stay home");
	await input.press("Enter");
	const card = page.locator("li.card", { hasText: "stay home" });
	await expect(card).toBeVisible();
	// Server unreachable: both the move and the board re-fetch fail.
	await page.route("**/cards/*/move", (r) => r.abort("connectionrefused"));
	await page.route("**/boards/*", (r) => (r.request().method() === "GET" ? r.abort("connectionrefused") : r.continue()));
	await card.dragTo(page.locator("[data-dropzone]").nth(3));
	await expect(card.locator("xpath=ancestor::section//h2")).toHaveText("Backlog");
});

test("card titles made only of invisible characters are rejected", async ({ request, page }) => {
	await page.goto(await createBoard(page, `QA2 invisible ${Date.now()}`));
	const col = await page.locator("[data-dropzone]").first().getAttribute("data-column-id");
	const res = await request.post(`/columns/${col}/cards`, { form: { title: "​​" } });
	expect(res.status()).toBe(400);
});

test("an oversized chunked body is answered with 413", async ({ page }) => {
	await page.goto(await createBoard(page, `QA2 chunked ${Date.now()}`));
	const col = await page.locator("[data-dropzone]").first().getAttribute("data-column-id");
	// Without Content-Length (chunked) the middleware's early 413 is skipped and
	// FormValue swallows the MaxBytesError, so the handler sees an empty title.
	const url = new URL(`/columns/${col}/cards`, page.url());
	const status = await new Promise<number>((resolve, reject) => {
		const req = http.request(url, {
			method: "POST",
			headers: { "Content-Type": "application/x-www-form-urlencoded", "Transfer-Encoding": "chunked" },
		});
		req.on("response", (res) => {
			res.resume();
			resolve(res.statusCode ?? 0);
		});
		req.on("error", reject);
		req.end(`title=${"a".repeat(2_000_000)}`);
	});
	expect(status).toBe(413);
});

test("card keys use three letters for non-ASCII project names", async ({ page }) => {
	await page.goto(await createBoard(page, `Ação ${Date.now()}`));
	const input = page.locator("[data-new-card]").first();
	await input.fill("key");
	await input.press("Enter");
	await expect(page.locator("li.card .font-mono").first()).toHaveText(/^AÇÃ-\d+$/);
});

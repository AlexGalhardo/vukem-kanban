import { expect, type Page, test } from "@playwright/test";

// QA round 2: keyboard focus, document title and contrast. Each test asserts
// the CORRECT behaviour and fails until the bug is fixed.

async function createBoard(page: Page, name: string): Promise<string> {
	const res = await page.request.post("/boards", { form: { name }, maxRedirects: 0 });
	const location = res.headers().location;
	if (!location) throw new Error(`no redirect creating ${name}`);
	return location;
}

async function addCard(page: Page, title: string): Promise<void> {
	const input = page.locator("[data-new-card]").first();
	await input.fill(title);
	await input.press("Enter");
	await expect(page.locator("li.card", { hasText: title })).toHaveCount(1);
}

const focusIsLost = (page: Page): Promise<boolean> =>
	page.evaluate(() => document.activeElement === null || document.activeElement === document.body);

test("saving the card dialog with Ctrl+Enter returns focus to the card", async ({ page }) => {
	await page.goto(await createBoard(page, `QA2 focus save ${Date.now()}`));
	await addCard(page, "focus me");
	await page.locator("li.card .card-open").first().focus();
	await page.keyboard.press("Enter");
	await expect(page.locator("dialog[open]")).toHaveCount(1);
	await page.locator("#card-description").focus();
	await page.keyboard.type("text");
	await page.keyboard.press("Control+Enter");
	await expect(page.locator("dialog[open]")).toHaveCount(0);
	await expect(page.locator("li.card", { hasText: "focus me" }).locator(".card-open")).toBeFocused();
});

test("toggling a label with the keyboard keeps focus on that label", async ({ page }) => {
	await page.goto(await createBoard(page, `QA2 focus label ${Date.now()}`));
	await addCard(page, "label me");
	await page.locator("[data-action=labels]").click();
	await page.fill("#new-label", "kb");
	await page.getByTestId("labels-dialog").getByRole("button", { name: "Criar label" }).click();
	await expect(page.locator("#filter-label option", { hasText: "kb" })).toHaveCount(1);
	await page.keyboard.press("Escape");
	await page.locator("li.card .card-open").first().click();
	const chip = page.getByTestId("card-labels").getByRole("button", { name: "kb" });
	await chip.focus();
	await page.keyboard.press("Enter");
	await expect(chip).toHaveAttribute("aria-pressed", "true");
	await expect(chip).toBeFocused();
});

test("creating a label keeps focus in the new label field", async ({ page }) => {
	await page.goto(await createBoard(page, `QA2 focus new label ${Date.now()}`));
	await page.locator("[data-action=labels]").click();
	await page.locator("#new-label").fill("one");
	await page.locator("#new-label").press("Enter");
	await expect(page.locator("#filter-label option", { hasText: "one" })).toHaveCount(1);
	await expect(page.locator("#new-label")).toBeFocused();
});

test("creating a project from the dialog does not drop focus on <body>", async ({ page }) => {
	await page.goto(await createBoard(page, `QA2 focus project ${Date.now()}`));
	await page.locator("body").click({ position: { x: 900, y: 600 } });
	await page.keyboard.press("n");
	await page.locator("#board-name").fill(`QA2 focus new ${Date.now()}`);
	await page.locator("#board-name").press("Enter");
	await expect(page.getByTestId("board-title")).toHaveText(/QA2 focus new/);
	expect(await focusIsLost(page)).toBe(false);
});

test("the document title follows the board after sidebar navigation", async ({ page }) => {
	const stamp = Date.now();
	const a = await createBoard(page, `QA2 title A ${stamp}`);
	await createBoard(page, `QA2 title B ${stamp}`);
	await page.goto(a);
	await expect(page).toHaveTitle(/QA2 title A/);
	await page.locator(`[data-board-link='QA2 title B ${stamp}']`).click();
	await expect(page.getByTestId("board-title")).toHaveText(/QA2 title B/);
	await expect(page).toHaveTitle(/QA2 title B/);
});

test("Escape closes an open column menu", async ({ page }) => {
	await page.goto(await createBoard(page, `QA2 menu esc ${Date.now()}`));
	const summary = page.getByTestId("column").first().locator("summary");
	await summary.focus();
	await page.keyboard.press("Enter");
	await expect(page.locator("details.col-menu[open]")).toHaveCount(1);
	await page.keyboard.press("Escape");
	await expect(page.locator("details.col-menu[open]")).toHaveCount(0);
});

// WCAG relative-luminance contrast of an element's text against the opaque
// background behind it (alpha layers blended, element opacity ignored).
async function contrast(page: Page, selector: string): Promise<number> {
	return page.locator(selector).first().evaluate((el) => {
		const ctx = document.createElement("canvas").getContext("2d");
		if (!ctx) throw new Error("no canvas");
		const rgba = (c: string): number[] => {
			ctx.clearRect(0, 0, 1, 1);
			ctx.fillStyle = c;
			ctx.fillRect(0, 0, 1, 1);
			const d = ctx.getImageData(0, 0, 1, 1).data;
			return [d[0] ?? 0, d[1] ?? 0, d[2] ?? 0, (d[3] ?? 0) / 255];
		};
		const blend = (fg: number[], bg: number[]): number[] =>
			[0, 1, 2].map((i) => (fg[i] ?? 0) * (fg[3] ?? 1) + (bg[i] ?? 0) * (1 - (fg[3] ?? 1)));
		const layers: number[][] = [];
		for (let e: Element | null = el; e; e = e.parentElement) {
			const b = rgba(getComputedStyle(e).backgroundColor);
			if ((b[3] ?? 0) > 0) layers.push(b);
			if ((b[3] ?? 0) >= 1) break;
		}
		let bg = [255, 255, 255];
		for (const l of layers.reverse()) bg = blend(l, bg);
		const fg = blend(rgba(getComputedStyle(el).color), bg);
		const lum = (c: number[]): number => {
			const [r, g, b] = c.map((v) => {
				const s = v / 255;
				return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
			});
			return 0.2126 * (r ?? 0) + 0.7152 * (g ?? 0) + 0.0722 * (b ?? 0);
		};
		const [hi, lo] = [lum(fg), lum(bg)].sort((x, y) => y - x);
		return ((hi ?? 0) + 0.05) / ((lo ?? 0) + 0.05);
	});
}

test("small secondary and danger texts meet WCAG AA contrast in the light theme", async ({ page }) => {
	await page.emulateMedia({ colorScheme: "light" });
	await page.goto(await createBoard(page, `QA2 contrast ${Date.now()}`));
	await page.evaluate(() => document.documentElement.classList.remove("dark"));
	await addCard(page, "contrast");
	// Sidebar section heading and version use text-faint on the panel background.
	expect(await contrast(page, "#sidebar h2")).toBeGreaterThanOrEqual(4.5);
	await page.locator("li.card .card-open").first().click();
	expect(await contrast(page, "dialog[open] .btn-danger")).toBeGreaterThanOrEqual(4.5);
});

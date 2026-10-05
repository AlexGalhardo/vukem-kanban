import { test as base, expect, type Page } from "@playwright/test";

// Every spec fails on browser console errors or uncaught exceptions; that is
// how CSP violations and JS bugs surface in E2E.
export const test = base.extend<{ consoleErrors: string[] }>({
	consoleErrors: [
		async ({ page }, use) => {
			const errors: string[] = [];
			page.on("console", (msg) => {
				// 4xx/5xx responses are part of the tested flows; the browser
				// logs each one as a failed resource load.
				if (msg.type() === "error" && !msg.text().startsWith("Failed to load resource")) errors.push(msg.text());
			});
			page.on("pageerror", (err) => errors.push(err.message));
			await use(errors);
			expect(errors, "browser console errors").toEqual([]);
		},
		{ auto: true },
	],
});

export { expect };

let seq = 0;

/** Returns a board name unique within the run so specs never collide. */
export function uniqueName(prefix: string): string {
	seq += 1;
	return `${prefix} ${Date.now().toString(36)}${seq}`;
}

/** Creates a board through the UI and waits until it is open. */
export async function createBoard(page: Page, name: string): Promise<void> {
	await page.goto("/");
	await page.locator("[data-action=new-board]").click();
	const dialog = page.getByTestId("new-board-dialog");
	await expect(dialog).toBeVisible();
	await dialog.getByLabel("Nome").fill(name);
	await dialog.getByRole("button", { name: "Criar projeto" }).click();
	await expect(page.getByTestId("board-title")).toHaveText(name);
	await expect(dialog).toBeHidden();
}

/** Adds a card to the column with the given name using its footer input. */
export async function addCard(page: Page, column: string, title: string): Promise<void> {
	const input = page.getByLabel(`Novo card em ${column}`, { exact: true }).last();
	await input.fill(title);
	await input.press("Enter");
	await expect(columnCards(page, column).filter({ hasText: title })).toHaveCount(1);
	await expect(input).toHaveValue("");
}

export function column(page: Page, name: string) {
	return page.getByTestId("column").filter({ has: page.getByRole("heading", { name, exact: true }) });
}

export function columnCards(page: Page, name: string) {
	return column(page, name).getByTestId("card");
}

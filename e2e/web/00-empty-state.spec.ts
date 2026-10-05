import { expect, test } from "./fixtures";

// Runs first (file order) against the fresh database of the run.
test("landing shows the empty state with a create button", async ({ page }) => {
	await page.goto("/");
	await expect(page).toHaveTitle("Kanban");
	await expect(page.getByTestId("empty-state")).toBeVisible();
	await page.getByTestId("empty-state").getByRole("button", { name: /Criar projeto/ }).click();
	await expect(page.getByTestId("new-board-dialog")).toBeVisible();
	await page.keyboard.press("Escape");
	await expect(page.getByTestId("new-board-dialog")).toHaveCount(0);
});

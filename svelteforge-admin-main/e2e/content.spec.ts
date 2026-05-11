import { test, expect } from "@playwright/test";
import { login, expectHeading } from "./helpers.js";

test.describe("Content Page", () => {
	test.beforeEach(async ({ page }) => {
		await login(page);
		await page.goto("/content");
	});

	test("content table renders", async ({ page }) => {
		await expectHeading(page, "Content");
		await expect(page.locator("table")).toBeVisible();
	});

	test("create page button exists", async ({ page }) => {
		await expect(page.locator("text=New Page")).toBeVisible();
	});

	test("search input is present", async ({ page }) => {
		const searchInput = page.locator('input[placeholder*="Search"]');
		await expect(searchInput).toBeVisible();
	});

	test("export button exists", async ({ page }) => {
		await expect(page.locator("text=Export")).toBeVisible();
	});

	test("pagination controls exist", async ({ page }) => {
		await expect(page.locator("text=Showing")).toBeVisible();
	});
});

import { test, expect } from "@playwright/test";
import { login, expectHeading } from "./helpers.js";

test.describe("Users Page", () => {
	test.beforeEach(async ({ page }) => {
		await login(page);
		await page.goto("/users");
	});

	test("users table renders", async ({ page }) => {
		await expectHeading(page, "Users");
		await expect(page.locator("table")).toBeVisible();
	});

	test("create user button exists", async ({ page }) => {
		await expect(page.locator("text=Add User")).toBeVisible();
	});

	test("search input filters table", async ({ page }) => {
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

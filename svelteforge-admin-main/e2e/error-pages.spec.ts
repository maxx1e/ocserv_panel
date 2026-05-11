import { test, expect } from "@playwright/test";
import { login } from "./helpers.js";

test.describe("Error Pages", () => {
	test("404 page for unknown route", async ({ page }) => {
		await login(page);
		await page.goto("/this-does-not-exist");
		await expect(page.locator("text=404")).toBeVisible();
		await expect(page.locator("text=Page Not Found")).toBeVisible();
		await expect(page.locator("text=Go Home")).toBeVisible();
	});

	test("404 page has go back button", async ({ page }) => {
		await login(page);
		await page.goto("/this-does-not-exist");
		await expect(page.locator("text=Go Back")).toBeVisible();
	});

	test("404 page go home link works", async ({ page }) => {
		await login(page);
		await page.goto("/this-does-not-exist");
		await page.click("text=Go Home");
		await page.waitForURL("/");
	});
});

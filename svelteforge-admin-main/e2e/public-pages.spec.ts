import { test, expect } from "@playwright/test";

test.describe("Public Pages", () => {
	test("pricing page renders without auth", async ({ page }) => {
		await page.goto("/pricing");
		await expect(page.locator("h1")).toContainText("pricing");
		await expect(page.locator("text=Free")).toBeVisible();
		await expect(page.locator("text=Pro")).toBeVisible();
		await expect(page.locator("text=Enterprise")).toBeVisible();
	});

	test("pricing page has sign in link", async ({ page }) => {
		await page.goto("/pricing");
		await expect(page.locator('a[href="/login"]')).toBeVisible();
	});

	test("pricing cards have CTA buttons", async ({ page }) => {
		await page.goto("/pricing");
		await expect(page.locator("text=Get Started")).toBeVisible();
		await expect(page.locator("text=Start Free Trial")).toBeVisible();
		await expect(page.locator("text=Contact Sales")).toBeVisible();
	});
});

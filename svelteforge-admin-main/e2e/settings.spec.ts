import { test, expect } from "@playwright/test";
import { login, expectHeading } from "./helpers.js";

test.describe("Settings Page", () => {
	test.beforeEach(async ({ page }) => {
		await login(page);
		await page.goto("/settings");
	});

	test("settings page renders with tabs", async ({ page }) => {
		await expectHeading(page, "Settings");
		await expect(page.locator("text=Profile")).toBeVisible();
		await expect(page.locator("text=Sessions")).toBeVisible();
		await expect(page.locator("text=Notifications")).toBeVisible();
	});

	test("profile tab shows form", async ({ page }) => {
		await expect(page.locator('input[name="name"]')).toBeVisible();
		await expect(page.locator('input[name="email"]')).toBeVisible();
	});

	test("sessions tab shows active sessions", async ({ page }) => {
		await page.click("text=Sessions");
		await expect(page.locator("text=Active Sessions")).toBeVisible();
	});

	test("notifications tab shows toggles", async ({ page }) => {
		await page.click("text=Notifications");
		await expect(page.locator("text=Notification Preferences")).toBeVisible();
		await expect(page.locator("text=New user registrations")).toBeVisible();
		await expect(page.locator("text=Security alerts")).toBeVisible();
	});

	test("application tab visible for admin", async ({ page }) => {
		await expect(page.locator('button:text("Application")')).toBeVisible();
	});
});

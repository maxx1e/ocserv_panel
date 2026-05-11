import { test, expect } from "@playwright/test";
import { login } from "./helpers.js";

test.describe("Dashboard", () => {
	test.beforeEach(async ({ page }) => {
		await login(page);
	});

	test("dashboard displays KPI cards", async ({ page }) => {
		await expect(page.locator("text=Total Users")).toBeVisible();
		await expect(page.locator("text=Total Pages")).toBeVisible();
		await expect(page.locator("text=Notifications")).toBeVisible();
	});

	test("dashboard displays charts section", async ({ page }) => {
		await expect(page.locator("text=User Signups")).toBeVisible();
		await expect(page.locator("text=User Roles")).toBeVisible();
	});

	test("dashboard displays recent activity", async ({ page }) => {
		await expect(page.locator("text=Recent Activity")).toBeVisible();
	});

	test("quick stats section renders", async ({ page }) => {
		await expect(page.locator("text=Published Pages")).toBeVisible();
		await expect(page.locator("text=Active Editors")).toBeVisible();
	});
});

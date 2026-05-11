import { type Page, expect } from "@playwright/test";

export async function login(page: Page, username = "admin", password = "password123") {
	await page.goto("/login");
	await page.fill('input[name="username"]', username);
	await page.fill('input[name="password"]', password);
	await page.click('button[type="submit"]');
	await page.waitForURL("/");
}

export async function register(
	page: Page,
	{ name, email, username, password }: Record<string, string>
) {
	await page.goto("/register");
	await page.fill('input[name="name"]', name);
	await page.fill('input[name="email"]', email);
	await page.fill('input[name="username"]', username);
	await page.fill('input[name="password"]', password);
	await page.click('button[type="submit"]');
	await page.waitForURL("/");
}

export async function expectHeading(page: Page, text: string) {
	await expect(page.locator("h1").first()).toContainText(text);
}

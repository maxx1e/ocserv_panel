import Database from "better-sqlite3";
import { drizzle } from "drizzle-orm/better-sqlite3";
import * as schema from "./schema.js";
import { hash } from "@node-rs/argon2";
import { generateId } from "../id.js";

const SCHEMA_SQL = `
CREATE TABLE IF NOT EXISTS users (
	id text PRIMARY KEY NOT NULL,
	email text NOT NULL,
	username text NOT NULL,
	password_hash text NOT NULL,
	name text NOT NULL,
	avatar_url text,
	role text DEFAULT 'viewer' NOT NULL,
	created_at integer NOT NULL,
	updated_at integer NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS users_email_unique ON users (email);
CREATE UNIQUE INDEX IF NOT EXISTS users_username_unique ON users (username);

CREATE TABLE IF NOT EXISTS sessions (
	id text PRIMARY KEY NOT NULL,
	user_id text NOT NULL,
	expires_at integer NOT NULL,
	user_agent text,
	ip_address text,
	created_at integer,
	FOREIGN KEY (user_id) REFERENCES users(id) ON UPDATE no action ON DELETE no action
);

CREATE TABLE IF NOT EXISTS password_reset_tokens (
	id text PRIMARY KEY NOT NULL,
	user_id text NOT NULL,
	token_hash text NOT NULL,
	expires_at integer NOT NULL,
	FOREIGN KEY (user_id) REFERENCES users(id) ON UPDATE no action ON DELETE no action
);

CREATE TABLE IF NOT EXISTS pages (
	id text PRIMARY KEY NOT NULL,
	title text NOT NULL,
	slug text NOT NULL,
	content text DEFAULT '' NOT NULL,
	template text DEFAULT 'default' NOT NULL,
	status text DEFAULT 'draft' NOT NULL,
	author_id text NOT NULL,
	created_at integer NOT NULL,
	updated_at integer NOT NULL,
	published_at integer,
	FOREIGN KEY (author_id) REFERENCES users(id) ON UPDATE no action ON DELETE no action
);
CREATE UNIQUE INDEX IF NOT EXISTS pages_slug_unique ON pages (slug);

CREATE TABLE IF NOT EXISTS notifications (
	id text PRIMARY KEY NOT NULL,
	user_id text,
	title text NOT NULL,
	message text NOT NULL,
	type text DEFAULT 'info' NOT NULL,
	read integer DEFAULT false NOT NULL,
	created_at integer NOT NULL,
	FOREIGN KEY (user_id) REFERENCES users(id) ON UPDATE no action ON DELETE no action
);

CREATE TABLE IF NOT EXISTS oauth_accounts (
	id text PRIMARY KEY NOT NULL,
	user_id text NOT NULL,
	provider text NOT NULL,
	provider_user_id text NOT NULL,
	created_at integer NOT NULL,
	FOREIGN KEY (user_id) REFERENCES users(id) ON UPDATE no action ON DELETE no action
);
CREATE UNIQUE INDEX IF NOT EXISTS oauth_provider_user_idx ON oauth_accounts (provider, provider_user_id);

CREATE TABLE IF NOT EXISTS app_settings (
	key text PRIMARY KEY NOT NULL,
	value text NOT NULL,
	updated_at integer NOT NULL
);
`;

export function createTestDb() {
	const sqlite = new Database(":memory:");
	sqlite.pragma("journal_mode = WAL");
	sqlite.exec(SCHEMA_SQL);
	return drizzle(sqlite, { schema });
}

export async function createTestUser(
	db: ReturnType<typeof createTestDb>,
	overrides: Partial<{
		id: string;
		name: string;
		email: string;
		username: string;
		role: "admin" | "editor" | "viewer";
	}> = {}
) {
	const id = overrides.id ?? generateId(10);
	const passwordHash = await hash("password123", {
		memoryCost: 19456,
		timeCost: 2,
		outputLen: 32,
		parallelism: 1,
	});

	await db.insert(schema.users).values({
		id,
		name: overrides.name ?? "Test User",
		email: overrides.email ?? `${id}@test.com`,
		username: overrides.username ?? `user_${id.slice(0, 8)}`,
		passwordHash,
		role: overrides.role ?? "viewer",
		createdAt: new Date(),
		updatedAt: new Date(),
	});

	return id;
}

export function createMockLocals(userId: string, role: string = "admin") {
	return {
		user: {
			id: userId,
			name: "Test User",
			email: "test@test.com",
			username: "testuser",
			role,
		},
		session: { id: "test-session", userId, expiresAt: Date.now() + 86400000 },
	};
}

export function createFormData(entries: Record<string, string>): FormData {
	const fd = new FormData();
	for (const [key, value] of Object.entries(entries)) {
		fd.set(key, value);
	}
	return fd;
}

export function createMockRequest(formData: FormData): Request {
	return new Request("http://localhost", {
		method: "POST",
		body: formData,
	});
}

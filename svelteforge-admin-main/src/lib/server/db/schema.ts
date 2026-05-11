import { sqliteTable, text, integer, uniqueIndex } from "drizzle-orm/sqlite-core";

export const users = sqliteTable("users", {
	id: text("id").primaryKey(),
	email: text("email").notNull().unique(),
	username: text("username").notNull().unique(),
	passwordHash: text("password_hash").notNull(),
	name: text("name").notNull(),
	avatarUrl: text("avatar_url"),
	role: text("role", { enum: ["admin", "editor", "viewer"] })
		.notNull()
		.default("viewer"),
	createdAt: integer("created_at", { mode: "timestamp" })
		.notNull()
		.$defaultFn(() => new Date()),
	updatedAt: integer("updated_at", { mode: "timestamp" })
		.notNull()
		.$defaultFn(() => new Date()),
});

export const sessions = sqliteTable("sessions", {
	id: text("id").primaryKey(),
	userId: text("user_id")
		.notNull()
		.references(() => users.id),
	expiresAt: integer("expires_at").notNull(),
	userAgent: text("user_agent"),
	ipAddress: text("ip_address"),
	createdAt: integer("created_at", { mode: "timestamp" }).$defaultFn(() => new Date()),
});

export const pages = sqliteTable("pages", {
	id: text("id").primaryKey(),
	title: text("title").notNull(),
	slug: text("slug").notNull().unique(),
	content: text("content").notNull().default(""),
	template: text("template", { enum: ["default", "landing", "blog"] })
		.notNull()
		.default("default"),
	status: text("status", { enum: ["draft", "published", "archived"] })
		.notNull()
		.default("draft"),
	authorId: text("author_id")
		.notNull()
		.references(() => users.id),
	createdAt: integer("created_at", { mode: "timestamp" })
		.notNull()
		.$defaultFn(() => new Date()),
	updatedAt: integer("updated_at", { mode: "timestamp" })
		.notNull()
		.$defaultFn(() => new Date()),
	publishedAt: integer("published_at", { mode: "timestamp" }),
});

export const notifications = sqliteTable("notifications", {
	id: text("id").primaryKey(),
	userId: text("user_id").references(() => users.id),
	title: text("title").notNull(),
	message: text("message").notNull(),
	type: text("type", { enum: ["info", "warning", "error", "success"] })
		.notNull()
		.default("info"),
	read: integer("read", { mode: "boolean" }).notNull().default(false),
	createdAt: integer("created_at", { mode: "timestamp" })
		.notNull()
		.$defaultFn(() => new Date()),
});

export const passwordResetTokens = sqliteTable("password_reset_tokens", {
	id: text("id").primaryKey(),
	userId: text("user_id")
		.notNull()
		.references(() => users.id),
	tokenHash: text("token_hash").notNull(),
	expiresAt: integer("expires_at", { mode: "timestamp" }).notNull(),
});

export const oauthAccounts = sqliteTable(
	"oauth_accounts",
	{
		id: text("id").primaryKey(),
		userId: text("user_id")
			.notNull()
			.references(() => users.id),
		provider: text("provider", { enum: ["google", "github"] }).notNull(),
		providerUserId: text("provider_user_id").notNull(),
		createdAt: integer("created_at", { mode: "timestamp" })
			.notNull()
			.$defaultFn(() => new Date()),
	},
	(table) => [uniqueIndex("oauth_provider_user_idx").on(table.provider, table.providerUserId)]
);

export const appSettings = sqliteTable("app_settings", {
	key: text("key").primaryKey(),
	value: text("value").notNull(),
	updatedAt: integer("updated_at", { mode: "timestamp" })
		.notNull()
		.$defaultFn(() => new Date()),
});

export type User = typeof users.$inferSelect;
export type NewUser = typeof users.$inferInsert;
export type Session = typeof sessions.$inferSelect;
export type Page = typeof pages.$inferSelect;
export type NewPage = typeof pages.$inferInsert;
export type Notification = typeof notifications.$inferSelect;
export type OAuthAccount = typeof oauthAccounts.$inferSelect;
export type AppSetting = typeof appSettings.$inferSelect;

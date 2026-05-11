import { db } from "$lib/server/db/index.js";
import { pages, users } from "$lib/server/db/schema.js";
import { fail } from "@sveltejs/kit";
import { eq, inArray } from "drizzle-orm";
import type { Actions, PageServerLoad } from "./$types.js";

export const load: PageServerLoad = async () => {
	const allPages = await db
		.select({
			id: pages.id,
			title: pages.title,
			slug: pages.slug,
			template: pages.template,
			status: pages.status,
			authorName: users.name,
			createdAt: pages.createdAt,
			updatedAt: pages.updatedAt,
			publishedAt: pages.publishedAt,
		})
		.from(pages)
		.leftJoin(users, eq(pages.authorId, users.id))
		.orderBy(pages.updatedAt);

	return { pages: allPages };
};

export const actions: Actions = {
	delete: async ({ request }) => {
		const formData = await request.formData();
		const id = formData.get("id");

		if (typeof id !== "string") {
			return fail(400, { message: "Page ID is required" });
		}

		await db.delete(pages).where(eq(pages.id, id));

		return { success: true };
	},

	bulkDelete: async ({ request }) => {
		const formData = await request.formData();
		const idsRaw = formData.get("ids");

		if (typeof idsRaw !== "string" || !idsRaw.trim()) {
			return fail(400, { message: "No pages selected" });
		}

		const ids = idsRaw.split(",").filter(Boolean);
		await db.delete(pages).where(inArray(pages.id, ids));

		return { success: true };
	},
};

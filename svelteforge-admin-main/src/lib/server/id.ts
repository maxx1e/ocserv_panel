import { encodeBase32LowerCaseNoPadding } from "@oslojs/encoding";

export function generateId(length: number = 15): string {
	const bytes = new Uint8Array(length);
	crypto.getRandomValues(bytes);
	return encodeBase32LowerCaseNoPadding(bytes);
}

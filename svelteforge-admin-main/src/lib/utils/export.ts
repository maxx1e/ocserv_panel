export function exportToCSV(data: Record<string, unknown>[], filename: string) {
	if (data.length === 0) return;

	const headers = Object.keys(data[0]);
	const csvRows = [
		headers.join(","),
		...data.map((row) =>
			headers
				.map((h) => {
					const val = row[h];
					const str = val === null || val === undefined ? "" : String(val);
					return str.includes(",") || str.includes('"') || str.includes("\n")
						? `"${str.replace(/"/g, '""')}"`
						: str;
				})
				.join(",")
		),
	];

	downloadBlob(csvRows.join("\n"), `${filename}.csv`, "text/csv");
}

export function exportToJSON(data: Record<string, unknown>[], filename: string) {
	downloadBlob(JSON.stringify(data, null, 2), `${filename}.json`, "application/json");
}

function downloadBlob(content: string, filename: string, mimeType: string) {
	const blob = new Blob([content], { type: mimeType });
	const url = URL.createObjectURL(blob);
	const a = document.createElement("a");
	a.href = url;
	a.download = filename;
	a.click();
	URL.revokeObjectURL(url);
}

type ParsedUA = {
	browser: string;
	os: string;
	device: string;
};

export function parseUserAgent(ua: string | null): ParsedUA {
	if (!ua) return { browser: "Unknown", os: "Unknown", device: "Unknown" };

	return {
		browser: parseBrowser(ua),
		os: parseOS(ua),
		device: parseDevice(ua),
	};
}

function parseBrowser(ua: string): string {
	if (/Edg\/(\d+)/.test(ua)) return `Edge ${RegExp.$1}`;
	if (/OPR\/(\d+)/.test(ua)) return `Opera ${RegExp.$1}`;
	if (/Chrome\/(\d+)/.test(ua)) return `Chrome ${RegExp.$1}`;
	if (/Firefox\/(\d+)/.test(ua)) return `Firefox ${RegExp.$1}`;
	if (/Safari\/(\d+)/.test(ua) && /Version\/(\d+)/.test(ua)) return `Safari ${RegExp.$1}`;
	return "Unknown";
}

function parseOS(ua: string): string {
	if (/Windows NT 10/.test(ua)) return "Windows 10+";
	if (/Windows NT/.test(ua)) return "Windows";
	if (/Mac OS X (\d+[._]\d+)/.test(ua)) return `macOS ${RegExp.$1.replace(/_/g, ".")}`;
	if (/Android (\d+)/.test(ua)) return `Android ${RegExp.$1}`;
	if (/iPhone OS (\d+)/.test(ua)) return `iOS ${RegExp.$1}`;
	if (/iPad/.test(ua)) return "iPadOS";
	if (/Linux/.test(ua)) return "Linux";
	if (/CrOS/.test(ua)) return "ChromeOS";
	return "Unknown";
}

function parseDevice(ua: string): string {
	if (/Mobile|Android.*Mobile|iPhone/.test(ua)) return "Mobile";
	if (/iPad|Android(?!.*Mobile)|Tablet/.test(ua)) return "Tablet";
	return "Desktop";
}

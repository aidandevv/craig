export interface DaemonHealth {
	status: "ok";
	daemon_id?: string;
}

export interface DetectedDaemon {
	url: string;
	daemonID: string;
}

const defaultDaemonURLs = ["http://127.0.0.1:8765", "http://localhost:8765"];

export function validateDaemonURL(value: string): string | undefined {
	try {
		const parsed = new URL(value.trim());
		if (parsed.protocol !== "http:" || (parsed.hostname !== "127.0.0.1" && parsed.hostname !== "localhost")) return undefined;
		if (parsed.username || parsed.password || parsed.search || parsed.hash || parsed.pathname !== "/") return undefined;
		return parsed.toString().replace(/\/$/, "");
	} catch {
		return undefined;
	}
}

export function daemonCandidates(preferred?: string): string[] {
	const candidates = [preferred && validateDaemonURL(preferred), ...defaultDaemonURLs];
	return [...new Set(candidates.filter((candidate): candidate is string => Boolean(candidate)))];
}

export async function detectDaemon(
	preferred?: string,
	fetcher: typeof fetch = fetch
): Promise<DetectedDaemon | undefined> {
	for (const url of daemonCandidates(preferred)) {
		try {
			const response = await fetcher(`${url}/healthz`, { signal: AbortSignal.timeout(4_000) });
			if (!response.ok) continue;
			const health = await response.json() as DaemonHealth;
			if (health.status !== "ok") continue;
			return { url, daemonID: typeof health.daemon_id === "string" ? health.daemon_id : "" };
		} catch {
			// A missing local port is expected while the helper is not running.
		}
	}
	return undefined;
}

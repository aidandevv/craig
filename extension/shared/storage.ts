import { defaultSettings, type Settings } from "./types";

// Chrome's callback and Promise extension APIs vary by browser/type package.
// These wrappers keep the local storage area (which holds the user's Vision
// key) and the non-secret session cache explicit, while also surfacing
// storage failures to callers.
const positiveInt = (value: unknown, fallback: number): number =>
	typeof value === "number" && Number.isInteger(value) && value >= 1 ? value : fallback;

export async function loadSettings(): Promise<Settings> {
	const defaults: Record<string, unknown> = { ...defaultSettings };
	const values = await get(chrome.storage.local, defaults);
	return {
		visionApiKey: typeof values.visionApiKey === "string" ? values.visionApiKey : defaultSettings.visionApiKey,
		monthlyCap: positiveInt(values.monthlyCap, defaultSettings.monthlyCap),
		maxPhotos: positiveInt(values.maxPhotos, defaultSettings.maxPhotos),
		daemonUrl: typeof values.daemonUrl === "string" ? values.daemonUrl : defaultSettings.daemonUrl,
		daemonId: typeof values.daemonId === "string" ? values.daemonId : defaultSettings.daemonId,
		token: typeof values.token === "string" ? values.token : defaultSettings.token,
		autoRun: typeof values.autoRun === "boolean" ? values.autoRun : defaultSettings.autoRun
	};
}

export function saveSettings(settings: Partial<Settings>): Promise<void> {
	return set(chrome.storage.local, { ...settings });
}

export async function getSessionValue<T>(key: string): Promise<T | undefined> {
	return (await get(chrome.storage.session, key))[key] as T | undefined;
}

export function setSessionValue(key: string, value: unknown): Promise<void> {
	return set(chrome.storage.session, { [key]: value });
}

function get(area: chrome.storage.StorageArea, keys: string | Record<string, unknown>): Promise<Record<string, unknown>> {
	return new Promise((resolve, reject) => {
		area.get(keys, (items) => {
			const error = chrome.runtime.lastError;
			if (error) {
				reject(new Error(error.message));
				return;
			}
			resolve(items as Record<string, unknown>);
		});
	});
}

function set(area: chrome.storage.StorageArea, values: Record<string, unknown>): Promise<void> {
	return new Promise((resolve, reject) => {
		area.set(values, () => {
			const error = chrome.runtime.lastError;
			if (error) {
				reject(new Error(error.message));
				return;
			}
			resolve();
		});
	});
}

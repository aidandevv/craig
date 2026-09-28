// Durable storage for the WebAssembly engine: Vision evidence (24h) and the
// per-feature monthly unit ledger. The service worker can be stopped at any
// moment, so nothing here may live only in memory.

const EVIDENCE_PREFIX = "craig:ev:";
const UNITS_PREFIX = "craig:units:";
const EVIDENCE_TTL_MS = 24 * 60 * 60 * 1000;

export interface StorageArea {
  get(keys: string | string[] | null): Promise<Record<string, unknown>>;
  set(items: Record<string, unknown>): Promise<void>;
  remove(keys: string | string[]): Promise<void>;
}

export interface CraigStore {
  getEvidence(hash: string, feature: string): Promise<string | null>;
  putEvidence(hash: string, feature: string, response: string, checkedAt: string): Promise<void>;
  reserveUnit(feature: string, cap: number): Promise<boolean>;
  unitsUsed(feature: string): Promise<number>;
  prune(): Promise<number>;
}

export function createEvidenceStore(area: StorageArea, now: () => Date = () => new Date()): CraigStore {
  // Ledger updates are read-modify-write; chaining them makes each atomic
  // with respect to the others inside this worker.
  let ledger: Promise<unknown> = Promise.resolve();
  const serial = <T,>(task: () => Promise<T>): Promise<T> => {
    const run = ledger.then(task, task);
    ledger = run.catch(() => undefined);
    return run;
  };
  const evidenceKey = (hash: string, feature: string) => `${EVIDENCE_PREFIX}${hash}:${feature}`;
  const unitsKey = (feature: string) => `${UNITS_PREFIX}${now().toISOString().slice(0, 7)}:${feature}`;
  const readCount = async (key: string) => {
    const value = (await area.get(key))[key];
    return typeof value === "number" && Number.isFinite(value) ? value : 0;
  };

  return {
    async getEvidence(hash, feature) {
      const key = evidenceKey(hash, feature);
      const value = (await area.get(key))[key];
      return typeof value === "string" ? value : null;
    },
    putEvidence(hash, feature, response, checkedAt) {
      return area.set({ [evidenceKey(hash, feature)]: JSON.stringify({ response, checked_at: checkedAt }) });
    },
    reserveUnit(feature, cap) {
      return serial(async () => {
        const key = unitsKey(feature);
        const used = await readCount(key);
        if (used >= cap) return false;
        await area.set({ [key]: used + 1 });
        return true;
      });
    },
    unitsUsed(feature) {
      return readCount(unitsKey(feature));
    },
    async prune() {
      const all = await area.get(null);
      const cutoff = now().getTime() - EVIDENCE_TTL_MS;
      const stale = Object.entries(all)
        .filter(([key, value]) => key.startsWith(EVIDENCE_PREFIX) && !isFresh(value, cutoff))
        .map(([key]) => key);
      if (stale.length) await area.remove(stale);
      return stale.length;
    }
  };
}

function isFresh(value: unknown, cutoff: number): boolean {
  if (typeof value !== "string") return false;
  try {
    const checked = Date.parse((JSON.parse(value) as { checked_at?: string }).checked_at ?? "");
    return Number.isFinite(checked) && checked >= cutoff;
  } catch {
    return false;
  }
}

export function chromeStorageArea(area: chrome.storage.StorageArea): StorageArea {
  const wrap = <T,>(call: (done: (value: T) => void) => void) => new Promise<T>((resolve, reject) => {
    call((value) => {
      const error = chrome.runtime.lastError;
      if (error) reject(new Error(error.message));
      else resolve(value);
    });
  });
  return {
    get: (keys) => wrap((done) => area.get(keys, (items) => done(items as Record<string, unknown>))),
    set: (items) => wrap((done) => area.set(items, () => done(undefined))),
    remove: (keys) => wrap((done) => area.remove(keys, () => done(undefined)))
  };
}

export function memoryStorageArea(): StorageArea {
  const data = new Map<string, unknown>();
  return {
    async get(keys) {
      const wanted = keys === null ? [...data.keys()] : Array.isArray(keys) ? keys : [keys];
      return Object.fromEntries(wanted.filter((key) => data.has(key)).map((key) => [key, data.get(key)]));
    },
    async set(items) {
      for (const [key, value] of Object.entries(items)) data.set(key, value);
    },
    async remove(keys) {
      for (const key of Array.isArray(keys) ? keys : [keys]) data.delete(key);
    }
  };
}

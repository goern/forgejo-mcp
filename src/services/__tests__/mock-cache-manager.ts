import { ICacheManager } from "../types.js";

export class MockCacheManager implements ICacheManager {
  private cache = new Map<string, any>();

  async get<T>(key: string): Promise<T | undefined> {
    return this.cache.get(key) as T | undefined;
  }

  async set<T>(key: string, value: T, ttlSeconds: number): Promise<void> {
    this.cache.set(key, value);
    if (ttlSeconds > 0) {
      setTimeout(() => this.cache.delete(key), ttlSeconds * 1000);
    }
  }

  async delete(key: string): Promise<void> {
    this.cache.delete(key);
  }

  async clear(): Promise<void> {
    this.cache.clear();
  }
}

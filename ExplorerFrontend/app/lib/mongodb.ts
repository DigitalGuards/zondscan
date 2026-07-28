import 'server-only';

import { MongoClient, type Db } from 'mongodb';

/**
 * Cached MongoDB client for server-side route handlers.
 *
 * The explorer frontend has historically been DB-free (all reads go through the
 * Go backend API), but the faucet needs to persist per-address/per-IP claim
 * cooldowns somewhere durable and shared across instances. This module gives the
 * faucet route a single pooled connection.
 *
 * The connection promise is memoised on `globalThis` so Next's dev HMR (which
 * re-evaluates modules on every edit) doesn't open a new pool on each reload.
 * `DATABASE_URL` is the same connection string the rest of the stack already
 * configures (see CLAUDE.md); the database name defaults to `qrldata-z` but can
 * be overridden with `MONGO_DB_NAME`.
 */

const DB_NAME = process.env.MONGO_DB_NAME || 'qrldata-z';

interface MongoCache {
  client: MongoClient;
  promise: Promise<MongoClient>;
}

declare global {
  var __faucetMongo: MongoCache | undefined;
}

function getClientPromise(): Promise<MongoClient> {
  const uri = process.env.DATABASE_URL;
  if (!uri) {
    return Promise.reject(
      new Error('DATABASE_URL is not configured; faucet cooldown store unavailable'),
    );
  }

  if (globalThis.__faucetMongo) {
    return globalThis.__faucetMongo.promise;
  }

  const client = new MongoClient(uri, {
    // Keep the pool small; the faucet is low-QPS by design (rate-limited).
    maxPoolSize: 5,
    serverSelectionTimeoutMS: 5000,
  });
  const promise = client.connect();
  globalThis.__faucetMongo = { client, promise };
  return promise;
}

/** Resolve the faucet database handle, connecting (once) on first use. */
export async function getFaucetDb(): Promise<Db> {
  const client = await getClientPromise();
  return client.db(DB_NAME);
}

import * as path from "path";
import { STORAGE_ROOT } from "./settings";
import { Id } from "lsif-protocol";
import { DefinitionReferenceResultId } from "./models.database";

/**
 * Reads an integer from an environment variable or defaults to the given value.
 *
 * @param key The environment variable name.
 * @param defaultValue The default value.
 */
export function readEnvInt(key: string, defaultValue: number): number {
  return (
    (process.env[key] && parseInt(process.env[key] || "", 10)) || defaultValue
  );
}

/**
 * Determine if an exception value has the given error code.
 *
 * @param e The exception value.
 * @param expectedCode The expected error code.
 */
export function hasErrorCode(e: any, expectedCode: string): boolean {
  return e && e.code === expectedCode;
}

// TODO - arguments

/**
 *.Computes the filename of the LSIF dump from the given repository and commit hash.
 *
 */
export function makeFilename(repository: string, commit: string): string {
  return path.join(
    STORAGE_ROOT,
    `${encodeURIComponent(repository)}@${commit}.lsif.db`
  );
}

/**
 * Return the value of the given key from the given map. If the key does not
 * exist in the map, an exception is thrown with the given error text.
 *
 * @param map The map to query.
 * @param key The key to search for.
 * @param elementType The type of element (used for exception message).
 */
export function mustGet<K, V>(map: Map<K, V>, key: K, elementType: string): V {
  const value = map.get(key);
  if (value !== undefined) {
    return value;
  }

  throw new Error(`Unknown ${elementType} '${key}'.`);
}

/**
 * Return the value of the given key from one of the given maps. The first
 * non-undefined value to be found is returned. If the key does not exist in
 * either map, an exception is thrown with the given error text.
 *
 * @param map1 The first map to query.
 * @param map2 The second map to query.
 * @param key The key to search for.
 * @param elementType The type of element (used for exception message).
 */
export function mustGetFromEither<K, V>(
  map1: Map<K, V>,
  map2: Map<K, V>,
  key: K,
  elementType: string
): V {
  for (const map of [map1, map2]) {
    const value = map.get(key);
    if (value !== undefined) {
      return value;
    }
  }

  throw new Error(`Unknown ${elementType} '${key}'.`);
}

/**
 * Return the value of `id`, or throw an exception if it is undefined.
 *
 * @param id The identifier.
 */
export function assertId<T extends Id>(id: T | undefined): T {
  if (id !== undefined) {
    return id;
  }

  throw new Error("id is undefined");
}

/**
 * Hash a string or numeric identifier into the range `[0, maxIndex)`. The
 * hash algorithm here is similar to the one used in Java's String.hashCode.
 *
 * @param id The identifier to hash.
 * @param maxIndex The maximum of the range.
 */
export function hashKey(
  id: DefinitionReferenceResultId,
  maxIndex: number
): number {
  const s = `${id}`;

  let hash = 0;
  for (let i = 0; i < s.length; i++) {
    const chr = s.charCodeAt(i);
    hash = (hash << 5) - hash + chr;
    hash |= 0;
  }

  // Hash value may be negative - must unset sign bit before modulus
  return Math.abs(hash) % maxIndex;
}

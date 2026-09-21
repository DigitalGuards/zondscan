/**
 * Typed access to the hand-written OpenAPI 3.1 spec in app/lib/openapi.json.
 *
 * The JSON file is the single source of truth for the public API surface;
 * scripts/check-api-docs-drift.mjs keeps it in sync with the Go route
 * registrations in CI. This module exposes the document plus a tag-ordered
 * view for rendering documentation pages.
 */

import spec from './openapi.json';

/** One parameter of an operation (path or query). */
export interface OpenApiParameter {
  name: string;
  in: string;
  description?: string;
  required?: boolean;
  schema?: Record<string, unknown>;
}

/** One operation (method + path) in the spec. */
export interface OpenApiOperation {
  tags?: string[];
  operationId?: string;
  summary?: string;
  description?: string;
  parameters?: OpenApiParameter[];
  requestBody?: Record<string, unknown>;
  responses?: Record<string, unknown>;
}

/** A tag entry from the document's top-level tags array. */
export interface OpenApiTag {
  name: string;
  description?: string;
}

/** Minimal structural type for the OpenAPI 3.1 document. */
export interface OpenApiDocument {
  openapi: string;
  info: {
    title: string;
    version: string;
    description?: string;
  };
  servers?: Array<{ url: string; description?: string }>;
  tags?: OpenApiTag[];
  paths: Record<string, Record<string, OpenApiOperation>>;
  components?: Record<string, unknown>;
}

/** The parsed spec document. */
export const openApiSpec: OpenApiDocument = spec;

/** One endpoint row in a tag group. */
export interface TaggedEndpoint {
  method: string;
  path: string;
  operation: OpenApiOperation;
  /** Stable kebab-case anchor id, e.g. "get-api-blocks". */
  anchor: string;
}

/** All endpoints of one tag, in spec order. */
export interface TagGroup {
  tag: string;
  description?: string;
  endpoints: TaggedEndpoint[];
}

/** Builds a stable kebab-case anchor from a method and path. */
function anchorFor(method: string, path: string): string {
  return `${method} ${path}`
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+/, '')
    .replace(/-+$/, '');
}

/**
 * Groups every operation by its first tag, preserving the tag order of the
 * document's tags array. Operations tagged with a name missing from that
 * array are appended in encounter order so nothing is silently dropped.
 */
export function getTaggedEndpoints(): TagGroup[] {
  const groups = new Map<string, TagGroup>();
  for (const tag of openApiSpec.tags ?? []) {
    groups.set(tag.name, { tag: tag.name, description: tag.description, endpoints: [] });
  }
  for (const [path, operations] of Object.entries(openApiSpec.paths)) {
    for (const [method, operation] of Object.entries(operations)) {
      const tagName = operation.tags?.[0];
      if (!tagName) continue;
      let group = groups.get(tagName);
      if (!group) {
        group = { tag: tagName, endpoints: [] };
        groups.set(tagName, group);
      }
      group.endpoints.push({
        method: method.toUpperCase(),
        path,
        operation,
        anchor: anchorFor(method, path),
      });
    }
  }
  return Array.from(groups.values());
}

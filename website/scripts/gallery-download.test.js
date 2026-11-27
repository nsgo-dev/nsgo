// Covers the gallery download's resume rules with a fake server and store. It
// checks which request follows an interruption, and when received bytes are
// thrown away instead of joined.

import { expect, test } from 'bun:test';
import { createHash } from 'node:crypto';
import { PausedError, download, parseContentRange } from './gallery-download.js';
import { assembleParts } from './gallery-store.js';

const MiB = 1024 * 1024;
const ETAG = '"file-1"';

function sourceBytes(length) {
  const bytes = new Uint8Array(length);
  let state = 7;
  for (let i = 0; i < length; i++) {
    state = (state * 1103515245 + 12345) & 0x7fffffff;
    bytes[i] = state & 0xff;
  }
  return bytes;
}

const digest = (bytes) => createHash('sha256').update(bytes).digest('hex');

// chunkStream delivers bytes in 64 KiB chunks, and fails once failAfter is
// reached when it is a number.
function chunkStream(bytes, failAfter, onFail) {
  let offset = 0;
  return new ReadableStream({
    pull(controller) {
      if (typeof failAfter === 'number' && offset >= failAfter) {
        if (onFail) onFail();
        controller.error(new Error('The connection dropped.'));
        return;
      }
      if (offset >= bytes.length) {
        controller.close();
        return;
      }
      let end = Math.min(offset + 64 * 1024, bytes.length);
      if (typeof failAfter === 'number' && end > failAfter) end = failAfter;
      controller.enqueue(bytes.slice(offset, end));
      offset = end;
    },
  });
}

// makeServer answers like GitHub Pages: Range and If-Range resume, and a 416
// rejects a start past the end. failAfter is per request index.
function makeServer({ bytes, ignoreRange = false, badStart = false, badTotal = false, failAfter, onFail } = {}) {
  const requests = [];
  const impl = async (url, init = {}) => {
    const index = requests.length;
    const headers = init.headers || {};
    const requested = headers.Range ? Number(/^bytes=(\d+)-$/.exec(headers.Range)[1]) : 0;
    requests.push({ range: headers.Range, ifRange: headers['If-Range'], cache: init.cache });
    let start = requested;
    if (ignoreRange || (headers['If-Range'] && headers['If-Range'] !== ETAG)) start = 0;
    if (requested > 0 && start === 0) {
      // An ignored Range answers with the whole file.
    } else if (requested >= bytes.length) {
      return new Response(null, { status: 416 });
    }
    if (badStart && start > 0) start = Math.max(0, start - 1000);
    const body = bytes.subarray(start);
    const responseHeaders = new Headers();
    responseHeaders.set('etag', ETAG);
    responseHeaders.set('content-length', String(body.length));
    const status = start > 0 ? 206 : 200;
    if (status === 206) {
      const total = badTotal ? bytes.length + 5 : bytes.length;
      responseHeaders.set('content-range', `bytes ${start}-${bytes.length - 1}/${total}`);
    }
    const failure = typeof failAfter === 'function' ? failAfter(index) : failAfter;
    return new Response(chunkStream(body, failure, onFail), { status, headers: responseHeaders });
  };
  return { impl, requests };
}

// memoryStore keeps the parts the downloader stores, as IndexedDB would.
function memoryStore({ failLoad = false, failAppend = false } = {}) {
  const state = { meta: null, parts: [] };
  return {
    state,
    calls: { load: 0, append: 0, clear: 0 },
    async load() {
      this.calls.load++;
      if (failLoad) throw new Error('The store is unavailable.');
      if (!state.meta) return null;
      const bytes = assembleParts(state.parts);
      if (!bytes) return null;
      return { etag: state.meta.etag, total: state.meta.total, bytes };
    },
    async append(url, meta, offset, pieces) {
      this.calls.append++;
      if (failAppend) throw new Error('The store is full.');
      const length = pieces.reduce((total, piece) => total + piece.byteLength, 0);
      const batch = new Uint8Array(length);
      let at = 0;
      for (const piece of pieces) {
        batch.set(piece, at);
        at += piece.byteLength;
      }
      state.parts.push({ offset, bytes: batch.buffer });
      state.meta = { etag: meta.etag, total: meta.total };
    },
    async clear() {
      this.calls.clear++;
      state.meta = null;
      state.parts = [];
    },
    async pruneExcept() {},
  };
}

const options = (bytes, server, store, extra = {}) => ({
  url: 'app.x.wasm.gzip',
  expected: bytes.length,
  fetch: server.impl,
  store,
  sleep: async () => {},
  ...extra,
});

test('continues a dropped download with a Range request', async () => {
  const bytes = sourceBytes(2 * MiB);
  const server = makeServer({ bytes, failAfter: (index) => (index === 0 ? MiB : null) });
  const store = memoryStore();
  const result = await download(options(bytes, server, store));
  expect(digest(result)).toBe(digest(bytes));
  expect(server.requests).toHaveLength(2);
  expect(server.requests[1].range).toBe(`bytes=${MiB}-`);
  expect(server.requests[1].ifRange).toBe(ETAG);
  expect(store.state.meta).toBeNull();
});

test('restarts when the server ignores the Range', async () => {
  const bytes = sourceBytes(2 * MiB);
  const server = makeServer({ bytes, failAfter: (index) => (index === 0 ? MiB : null), ignoreRange: true });
  const store = memoryStore();
  const result = await download(options(bytes, server, store));
  expect(digest(result)).toBe(digest(bytes));
  expect(server.requests.length).toBeGreaterThanOrEqual(2);
  expect(server.requests[1].range).toBe(`bytes=${MiB}-`);
  expect(store.state.meta).toBeNull();
});

test('restarts when the answer covers another range', async () => {
  const bytes = sourceBytes(2 * MiB);
  const server = makeServer({ bytes, failAfter: (index) => (index === 0 ? MiB : null), badStart: true });
  const store = memoryStore();
  const result = await download(options(bytes, server, store));
  expect(digest(result)).toBe(digest(bytes));
  expect(server.requests).toHaveLength(3);
  expect(server.requests[2].range).toBeUndefined();
});

test('restarts when the answer names another total', async () => {
  const bytes = sourceBytes(2 * MiB);
  const server = makeServer({ bytes, failAfter: (index) => (index === 0 ? MiB : null), badTotal: true });
  const store = memoryStore();
  const result = await download(options(bytes, server, store));
  expect(digest(result)).toBe(digest(bytes));
  expect(server.requests[2].range).toBeUndefined();
});

test('restarts after a 416', async () => {
  const bytes = sourceBytes(2 * MiB);
  const server = makeServer({ bytes });
  const store = memoryStore();
  store.state.meta = { etag: ETAG, total: bytes.length };
  store.state.parts = [{ offset: 0, bytes: bytes.slice(0, MiB).buffer }];
  // The first ranged request is rejected, so the prefix goes and a full
  // request follows.
  const reject = server.impl;
  const fetchImpl = async (url, init) => {
    if (init.headers && init.headers.Range) {
      server.requests.push({ range: init.headers.Range, ifRange: init.headers['If-Range'] });
      return new Response(null, { status: 416 });
    }
    return reject(url, init);
  };
  const result = await download(options(bytes, { impl: fetchImpl, requests: server.requests }, store));
  expect(digest(result)).toBe(digest(bytes));
  expect(server.requests[1].range).toBeUndefined();
});

test('downloads without a store', async () => {
  const bytes = sourceBytes(MiB);
  const server = makeServer({ bytes });
  const store = memoryStore({ failLoad: true, failAppend: true });
  const result = await download(options(bytes, server, store));
  expect(digest(result)).toBe(digest(bytes));
});

test('joins stored parts in offset order and rejects a gap', () => {
  const first = new Uint8Array([1, 2, 3, 4]);
  const second = new Uint8Array([5, 6]);
  const joined = assembleParts([
    { offset: 4, bytes: second.buffer },
    { offset: 0, bytes: first.buffer },
  ]);
  expect(Array.from(joined)).toEqual([1, 2, 3, 4, 5, 6]);
  expect(
    assembleParts([
      { offset: 0, bytes: first.buffer },
      { offset: 8, bytes: second.buffer },
    ]),
  ).toBeNull();
});

test('pauses while hidden and resumes from the stored bytes', async () => {
  const bytes = sourceBytes(2 * MiB);
  let hidden = false;
  const server = makeServer({
    bytes,
    failAfter: (index) => (index === 0 ? MiB : null),
    onFail: () => {
      hidden = true;
    },
  });
  const store = memoryStore();
  let paused = null;
  try {
    await download(options(bytes, server, store, { isHidden: () => hidden }));
  } catch (error) {
    paused = error;
  }
  expect(paused).toBeInstanceOf(PausedError);
  expect(store.state.meta).not.toBeNull();

  hidden = false;
  const result = await download(options(bytes, server, store, { isHidden: () => false }));
  expect(digest(result)).toBe(digest(bytes));
  expect(server.requests[1].range).toBe(`bytes=${MiB}-`);
});

test('drops a stored total that differs from the build size', async () => {
  const bytes = sourceBytes(MiB);
  const server = makeServer({ bytes });
  const store = memoryStore();
  store.state.meta = { etag: ETAG, total: bytes.length - 100 };
  store.state.parts = [{ offset: 0, bytes: bytes.slice(0, 1024).buffer }];
  const result = await download(options(bytes, server, store));
  expect(digest(result)).toBe(digest(bytes));
  expect(server.requests[0].range).toBeUndefined();
  expect(server.requests).toHaveLength(1);
});

test('reports progress in order', async () => {
  const bytes = sourceBytes(MiB);
  const server = makeServer({ bytes });
  const store = memoryStore();
  const seen = [];
  await download(options(bytes, server, store, { onProgress: (loaded) => seen.push(loaded) }));
  expect(seen.length).toBeGreaterThan(0);
  expect(seen).toEqual(seen.slice().sort((a, b) => a - b));
  expect(seen[seen.length - 1]).toBe(bytes.length);
});

test('reads a Content-Range header', () => {
  expect(parseContentRange('bytes 10-19/100')).toEqual({ start: 10, end: 19, total: 100 });
  expect(parseContentRange('bytes 10-19/*')).toEqual({ start: 10, end: 19, total: 0 });
  expect(parseContentRange('items 1-2/3')).toBeNull();
  expect(parseContentRange(null)).toBeNull();
});

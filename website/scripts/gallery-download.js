// Downloads the gallery's compressed wasm and continues an interrupted
// download from the bytes it kept.
//
// The file is content-named, so one URL means one file. A Range request from
// the kept prefix can only return that file's next bytes. Content-Range still
// checks the prefix and the total before the parts are joined.

// A frozen page loses at most the batches it has not stored yet.
export const BATCH_BYTES = 512 * 1024;

// The attempts one download makes while the page is visible.
const ATTEMPTS = 4;

// A connection that sends no byte for this long is abandoned and resumed.
const STALL_MS = 30 * 1000;

// PausedError reports that the page hid during the download. The caller
// starts the download again when the page is visible.
export class PausedError extends Error {
  constructor() {
    super('The download paused while the page was hidden.');
    this.name = 'PausedError';
  }
}

// Restart reports an answer that cannot continue the kept prefix.
class Restart extends Error {}

const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

// download returns the file's bytes, continuing a stored prefix when the
// store has one. A store that fails costs only the ability to resume.
export async function download({
  url,
  expected,
  store,
  fetch: fetchImpl = typeof fetch === 'function' ? fetch : undefined,
  onProgress = () => {},
  sleep = delay,
  isHidden = () => false,
  signal,
  stallMs = STALL_MS,
}) {
  if (!fetchImpl) throw new Error('This browser cannot download the engine.');
  let persist = Boolean(store);
  let state = null;
  if (store) {
    try {
      state = await store.load(url);
    } catch (error) {
      persist = false;
    }
  }
  if (state && expected > 0 && state.total !== expected) {
    await clearStored(url, store);
    state = null;
  }

  let parts = state ? [state.bytes] : [];
  let received = state ? state.bytes.byteLength : 0;
  let persisted = received;
  let etag = state ? state.etag : null;
  let total = state ? state.total : expected;
  let pending = [];
  let pendingBytes = 0;
  let restarts = 0;
  let attempt = 0;

  const flush = async () => {
    if (!persist || pendingBytes === 0) return;
    try {
      await store.append(url, { etag, total }, persisted, pending);
      persisted += pendingBytes;
      pending = [];
      pendingBytes = 0;
    } catch (error) {
      persist = false;
    }
  };

  const restart = async () => {
    parts = [];
    received = 0;
    persisted = 0;
    pending = [];
    pendingBytes = 0;
    etag = null;
    total = expected;
    await clearStored(url, store);
  };

  for (;;) {
    if (signal && signal.aborted) throw new PausedError();
    if (attempt > 0) {
      if (isHidden()) throw new PausedError();
      await sleep(Math.min(1000 * 2 ** (attempt - 1), 8000));
    }
    const request = new AbortController();
    const abort = () => request.abort();
    if (signal) signal.addEventListener('abort', abort, { once: true });
    let stalled = false;
    let timer = 0;
    const hold = () => {
      clearTimeout(timer);
      timer = setTimeout(() => {
        stalled = true;
        request.abort();
      }, stallMs);
    };
    try {
      hold();
      const headers = {};
      if (received > 0) {
        headers.Range = 'bytes=' + received + '-';
        if (etag && !etag.startsWith('W/')) headers['If-Range'] = etag;
      }
      const response = await fetchImpl(url, {
        headers,
        cache: received > 0 ? 'no-store' : 'force-cache',
        signal: request.signal,
      });
      if (response.status === 416) throw new Restart('range not satisfiable');
      if (!response.ok) throw new Error('The WebAssembly download failed (' + response.status + ').');
      const encoding = response.headers.get('content-encoding');
      // An encoded response makes byte offsets ambiguous, so it never resumes.
      const resumable = !encoding || encoding === 'identity';
      if (response.status === 206) {
        if (!resumable) throw new Restart('an encoded response cannot resume');
        const range = parseContentRange(response.headers.get('content-range'));
        if (!range || range.start !== received) throw new Restart('another range answered');
        if (total > 0 && range.total > 0 && range.total !== total) {
          throw new Restart('another file answered');
        }
        total = range.total || total;
      } else {
        // A 200 carries the whole file, so a kept prefix is void.
        if (received > 0) await restart();
        etag = response.headers.get('etag') || null;
        total = Number(response.headers.get('content-length')) || expected;
        if (expected > 0 && total !== expected) throw new Restart('another file answered');
        if (!resumable) persist = false;
      }
      if (!response.body) {
        const whole = new Uint8Array(await response.arrayBuffer());
        parts.push(whole);
        received += whole.byteLength;
        onProgress(received, total);
      } else {
        const reader = response.body.getReader();
        for (;;) {
          const result = await reader.read();
          if (result.done) break;
          const value = result.value;
          if (!value || value.byteLength === 0) continue;
          hold();
          const piece = value instanceof Uint8Array ? value : new Uint8Array(value);
          parts.push(piece);
          pending.push(piece);
          received += piece.byteLength;
          pendingBytes += piece.byteLength;
          onProgress(received, total);
          if (pendingBytes >= BATCH_BYTES) await flush();
        }
      }
      await flush();
      if (!Number.isFinite(total) || total <= 0) total = received;
      if (received !== total) throw new Restart('a truncated response answered');
      const bytes = join(parts, received);
      await clearStored(url, store);
      return bytes;
    } catch (error) {
      // Keep what this attempt received, so the next one resumes from it.
      await flush();
      if (error instanceof Restart) {
        if (++restarts > ATTEMPTS) throw new Error('The server kept answering with inconsistent ranges.');
        await restart();
        attempt = 0;
        continue;
      }
      if ((signal && signal.aborted) || isHidden()) throw new PausedError();
      attempt++;
      if (attempt >= ATTEMPTS) throw stalled ? new Error('The download stopped sending bytes.') : error;
    } finally {
      clearTimeout(timer);
      if (signal) signal.removeEventListener('abort', abort);
    }
  }
}

// parseContentRange reads a response's "bytes start-end/total" header.
export function parseContentRange(value) {
  if (!value) return null;
  const match = /^bytes (\d+)-(\d+)\/(\d+|\*)$/.exec(value);
  if (!match) return null;
  return {
    start: Number(match[1]),
    end: Number(match[2]),
    total: match[3] === '*' ? 0 : Number(match[3]),
  };
}

function join(parts, length) {
  const bytes = new Uint8Array(length);
  let offset = 0;
  for (const part of parts) {
    bytes.set(part, offset);
    offset += part.byteLength;
  }
  return bytes;
}

async function clearStored(url, store) {
  if (!store) return;
  try {
    await store.clear(url);
  } catch (error) {
    // A store that cannot clear costs nothing here: the download has its bytes.
  }
}

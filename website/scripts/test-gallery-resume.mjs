// Drives the built gallery in Chrome. An interrupted wasm download must resume
// from the stored bytes after a reload, not start again. Run `bun run gallery`
// first; needs Google Chrome. The dropped response prints a Bun error to
// stderr; the run still exits 0.

import { spawn } from 'node:child_process';
import { existsSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const website = path.resolve(here, '..');
const publicDir = path.join(website, 'public');
const chrome = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const group = 'apps';
const port = 9330;

const page = path.join(publicDir, 'gallery', group, 'app.html');
if (!existsSync(page)) throw new Error('public/gallery/ is missing: run `bun run gallery` first');
const config = JSON.parse(readFileSync(path.join(publicDir, 'gallery', group, 'nsgo.json'), 'utf8'));
const wasmPath = `/gallery/${group}/${config.gzip}`;

// A slow answer lets the test interrupt the download on purpose.
const chunkBytes = 256 * 1024;
const chunkDelay = 100;

let requests = [];
let dropNow = false;
let failAll = false;

const server = Bun.serve({
  port: 0,
  idleTimeout: 60,
  async fetch(request) {
    const { pathname } = new URL(request.url);
    if (pathname !== wasmPath) {
      const name = decodeURIComponent(pathname.replace(/^\//, '')).replace(/(^|\/)$/, '$1index.html');
      const file = Bun.file(path.join(publicDir, name));
      return (await file.exists()) ? new Response(file) : new Response('Not found', { status: 404 });
    }
    if (failAll) return new Response('unavailable', { status: 503 });
    const file = Bun.file(path.join(publicDir, 'gallery', group, config.gzip));
    const size = file.size;
    const range = request.headers.get('range');
    const start = range ? Number(/^bytes=(\d+)-$/.exec(range)[1]) : 0;
    const record = { range, start, delivered: 0 };
    requests.push(record);
    const stream = new ReadableStream({
      async pull(controller) {
        if (dropNow) {
          dropNow = false;
          controller.error(new Error('The connection dropped.'));
          return;
        }
        const offset = start + record.delivered;
        if (offset >= size) {
          controller.close();
          return;
        }
        const end = Math.min(offset + chunkBytes, size);
        const chunk = new Uint8Array(await file.slice(offset, end).arrayBuffer());
        record.delivered += chunk.byteLength;
        controller.enqueue(chunk);
        await Bun.sleep(chunkDelay);
      },
    });
    const headers = {
      'content-type': 'application/octet-stream',
      'accept-ranges': 'bytes',
      'cache-control': 'no-store',
      etag: '"gallery-test"',
    };
    if (start > 0) {
      headers['content-range'] = `bytes ${start}-${size - 1}/${size}`;
      headers['content-length'] = String(size - start);
      return new Response(stream, { status: 206, headers });
    }
    headers['content-length'] = String(size);
    return new Response(stream, { status: 200, headers });
  },
});

const profile = mkdtempSync(path.join(tmpdir(), 'nsgo-resume-'));
const browser = spawn(
  chrome,
  ['--headless=new', `--remote-debugging-port=${port}`, `--user-data-dir=${profile}`, 'about:blank'],
  { stdio: 'ignore' },
);

function send(socket, pending, method, params = {}) {
  return new Promise((resolve) => {
    const id = send.calls = (send.calls || 0) + 1;
    pending.set(id, resolve);
    socket.send(JSON.stringify({ id, method, params }));
  });
}

async function evaluate(call, expression) {
  const { result, exceptionDetails } = await call('Runtime.evaluate', {
    expression,
    awaitPromise: true,
    returnByValue: true,
  });
  if (exceptionDetails) throw new Error(exceptionDetails.text || 'The page could not run the check.');
  return result.value;
}

const bootTitle = () => `document.querySelector('.boot__title')?.textContent ?? ''`;
const bootVisible = () => `document.querySelector('.boot') !== null`;
const ready = () => `document.querySelector('.boot') === null`;
const progressWidth = () => `parseFloat(document.getElementById('boot-fill')?.style.width || '0')`;

async function waitFor(check, timeout, label) {
  const started = Date.now();
  for (;;) {
    if (await check()) return;
    if (Date.now() - started > timeout) throw new Error(`timed out: ${label}`);
    await Bun.sleep(200);
  }
}

let socket;
try {
  let target;
  for (let i = 0; i < 50 && !target; i++) {
    try {
      const targets = await (await fetch(`http://127.0.0.1:${port}/json`)).json();
      target = targets.find((entry) => entry.type === 'page');
    } catch {
      await Bun.sleep(200);
    }
  }
  if (!target) throw new Error('Chrome did not open a page.');

  socket = new WebSocket(target.webSocketDebuggerUrl);
  await new Promise((resolve) => (socket.onopen = resolve));
  const pending = new Map();
  socket.onmessage = (event) => {
    const message = JSON.parse(String(event.data));
    const resolve = pending.get(message.id);
    if (typeof resolve === 'function') {
      pending.delete(message.id);
      resolve(message.result);
    }
  };
  const call = (method, params = {}) => send(socket, pending, method, params);

  await call('Runtime.enable');
  await call('Network.enable');

  const url = `http://127.0.0.1:${server.port}/gallery/${group}/app.html`;

  // A whole download reaches ready on one plain request.
  await call('Page.navigate', { url });
  await waitFor(async () => evaluate(call, bootVisible()), 30000, 'the first load to show the boot screen');
  await waitFor(async () => evaluate(call, ready()), 90000, 'the first load to start');
  if (requests.length !== 1 || requests[0].start !== 0) {
    throw new Error(`the first load made ${requests.length} requests; expected one plain request`);
  }
  console.log('ok: a plain load downloads the whole file once');

  // A dropped connection leaves a stored prefix; the reload continues it.
  requests = [];
  await call('Page.reload', { ignoreCache: true });
  await waitFor(async () => evaluate(call, bootVisible()), 30000, 'the reloaded page to show the boot screen');
  await waitFor(
    async () => (await evaluate(call, progressWidth())) > 0,
    30000,
    'the reloaded download to start',
  );
  dropNow = true;
  failAll = true;
  const interrupted = requests[0];
  await waitFor(
    async () => (await evaluate(call, bootTitle())) === 'The gallery could not start',
    30000,
    'the interrupted download to report a failure',
  );
  failAll = false;
  const before = requests.length;

  await call('Page.reload', { ignoreCache: true });
  await waitFor(async () => evaluate(call, ready()), 120000, 'the resumed download to start');
  const resumed = requests.slice(before).filter((record) => record.start > 0);
  if (resumed.length === 0) throw new Error('the resumed load sent no Range request');
  if (resumed[0].start > interrupted.delivered) {
    throw new Error(`the resumed load asked for byte ${resumed[0].start}, past the ${interrupted.delivered} received`);
  }
  console.log(`ok: the resumed load asked for byte ${resumed[0].start} of ${interrupted.delivered} received`);

  const stored = await evaluate(
    call,
    `new Promise((resolve, reject) => {
      const opening = indexedDB.open('nsgo-gallery-downloads');
      opening.onerror = () => reject(opening.error);
      opening.onsuccess = () => {
        const db = opening.result;
        if (!db.objectStoreNames.contains('partials')) return resolve(0);
        const tx = db.transaction(['partials', 'parts'], 'readonly');
        const meta = tx.objectStore('partials').count();
        const parts = tx.objectStore('parts').count();
        tx.oncomplete = () => resolve(meta.result + parts.result);
      };
    })`,
  );
  if (stored !== 0) throw new Error(`the store still holds ${stored} entries after the download`);
  console.log('ok: the store is empty after the download');
  console.log('PASS');
} finally {
  if (socket) socket.close();
  const exited = new Promise((resolve) => browser.once('exit', resolve));
  browser.kill();
  await exited;
  rmSync(profile, { recursive: true, force: true });
  server.stop(true);
}

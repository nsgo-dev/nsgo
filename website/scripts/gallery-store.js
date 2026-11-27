// Keeps an interrupted download in IndexedDB, one part per stored batch. A
// reload or a suspended page then continues where it stopped.

const NAME = 'nsgo-gallery-downloads';
const VERSION = 1;

// openStore returns a store over IndexedDB, or null when the browser has
// none. The caller ignores a store that fails: the download still runs.
export function openStore() {
  if (typeof indexedDB === 'undefined') return null;
  return new Store();
}

// assembleParts joins a URL's stored parts in offset order. It returns null
// when the parts do not form one prefix, so unrelated bytes are never joined.
export function assembleParts(parts) {
  const ordered = parts.slice().sort((a, b) => a.offset - b.offset);
  let received = 0;
  for (const part of ordered) {
    if (part.offset !== received) return null;
    received += part.bytes.byteLength;
  }
  if (received === 0) return null;
  const bytes = new Uint8Array(received);
  let offset = 0;
  for (const part of ordered) {
    bytes.set(new Uint8Array(part.bytes), offset);
    offset += part.bytes.byteLength;
  }
  return bytes;
}

function request(source) {
  return new Promise((resolve, reject) => {
    source.onsuccess = () => resolve(source.result);
    source.onerror = () => reject(source.error);
  });
}

function done(transaction) {
  return new Promise((resolve, reject) => {
    transaction.oncomplete = () => resolve();
    transaction.onerror = () => reject(transaction.error);
    transaction.onabort = () => reject(transaction.error || new Error('The download store aborted.'));
  });
}

const partsRange = (url) => IDBKeyRange.bound([url, 0], [url, Number.MAX_SAFE_INTEGER]);

class Store {
  constructor() {
    this.opening = null;
  }

  open() {
    if (!this.opening) {
      this.opening = new Promise((resolve, reject) => {
        const opening = indexedDB.open(NAME, VERSION);
        opening.onupgradeneeded = () => {
          const db = opening.result;
          if (!db.objectStoreNames.contains('partials')) db.createObjectStore('partials', { keyPath: 'url' });
          if (!db.objectStoreNames.contains('parts')) db.createObjectStore('parts', { keyPath: ['url', 'offset'] });
        };
        opening.onsuccess = () => resolve(opening.result);
        opening.onerror = () => reject(opening.error);
        opening.onblocked = () => reject(new Error('The download store is blocked by another page.'));
      }).catch((error) => {
        this.opening = null;
        throw error;
      });
    }
    return this.opening;
  }

  async load(url) {
    const db = await this.open();
    const transaction = db.transaction(['partials', 'parts'], 'readonly');
    const partial = request(transaction.objectStore('partials').get(url));
    const parts = request(transaction.objectStore('parts').getAll(partsRange(url)));
    const [meta, stored] = await Promise.all([partial, parts]);
    if (!meta) return null;
    const bytes = assembleParts(stored);
    if (!bytes) {
      await this.clear(url);
      return null;
    }
    return { etag: meta.etag || null, total: Number(meta.total) || 0, bytes };
  }

  async append(url, meta, offset, pieces) {
    const length = pieces.reduce((total, piece) => total + piece.byteLength, 0);
    if (length === 0) return;
    const batch = new Uint8Array(length);
    let at = 0;
    for (const piece of pieces) {
      batch.set(piece, at);
      at += piece.byteLength;
    }
    const db = await this.open();
    const transaction = db.transaction(['partials', 'parts'], 'readwrite');
    transaction.objectStore('parts').put({ url, offset, bytes: batch.buffer });
    transaction.objectStore('partials').put({
      url,
      etag: meta.etag || null,
      total: Number(meta.total) || 0,
      received: offset + length,
    });
    await done(transaction);
  }

  async clear(url) {
    const db = await this.open();
    const transaction = db.transaction(['partials', 'parts'], 'readwrite');
    transaction.objectStore('partials').delete(url);
    transaction.objectStore('parts').delete(partsRange(url));
    await done(transaction);
  }

  async pruneExcept(url) {
    const db = await this.open();
    const partials = await request(db.transaction('partials', 'readonly').objectStore('partials').getAllKeys());
    const parts = await request(db.transaction('parts', 'readonly').objectStore('parts').getAllKeys());
    const stale = partials.filter((key) => key !== url);
    const staleParts = parts.filter((key) => key[0] !== url);
    if (stale.length === 0 && staleParts.length === 0) return;
    const transaction = db.transaction(['partials', 'parts'], 'readwrite');
    for (const key of stale) transaction.objectStore('partials').delete(key);
    for (const key of staleParts) transaction.objectStore('parts').delete(key);
    await done(transaction);
  }
}

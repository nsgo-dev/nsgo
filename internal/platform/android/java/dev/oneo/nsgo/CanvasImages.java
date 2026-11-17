package dev.oneo.nsgo;

import java.util.LinkedHashMap;
import java.util.Map;
import java.util.function.Consumer;

/** One surface's bounded canvas cache, including loading and failed entries. */
final class CanvasImages<T> {
    static final int LIMIT = 64;

    interface Requester {
        void request(String key, int generation, boolean cancel);
    }

    /** A frame holds a lease until its GL upload or canvas drawing finishes. */
    final class Lease implements AutoCloseable {
        final T value;
        private Entry entry;

        Lease(Entry entry) {
            this.entry = entry;
            value = entry.value;
            entry.refs++;
        }

        @Override
        public void close() {
            synchronized (CanvasImages.this) {
                if (entry == null) return;
                if (--entry.refs == 0 && entry.retired) dispose.accept(entry.value);
                entry = null;
            }
        }
    }

    private final class Entry {
        final int generation;
        T value;
        boolean pending = true, retired;
        int refs;

        Entry(int generation) { this.generation = generation; }
    }

    private final LinkedHashMap<String, Entry> entries = new LinkedHashMap<>(LIMIT, .75f, true);
    private final Requester requester;
    private final Consumer<T> dispose;
    private int generation;
    private boolean closed;

    CanvasImages(Requester requester, Consumer<T> dispose) {
        this.requester = requester;
        this.dispose = dispose;
    }

    synchronized Lease acquire(String key) {
        if (closed) return null;
        Entry entry = entries.get(key);
        if (entry == null) {
            if (entries.size() == LIMIT) {
                Map.Entry<String, Entry> oldest = entries.entrySet().iterator().next();
                entries.remove(oldest.getKey());
                retire(oldest.getKey(), oldest.getValue());
            }
            entry = new Entry(++generation);
            entries.put(key, entry);
            requester.request(key, entry.generation, false);
        }
        return entry.value == null ? null : new Lease(entry);
    }

    synchronized boolean wants(String key, int generation) {
        // Looking up an async response must not make it recently drawn.
        for (Map.Entry<String, Entry> item : entries.entrySet()) {
            Entry entry = item.getValue();
            if (item.getKey().equals(key)) return entry.generation == generation && entry.pending;
        }
        return false;
    }

    /** Takes ownership even of obsolete results, disposing them immediately. */
    synchronized boolean complete(String key, int generation, T value) {
        if (!wants(key, generation)) {
            if (value != null) dispose.accept(value);
            return false;
        }
        for (Map.Entry<String, Entry> item : entries.entrySet()) {
            if (!item.getKey().equals(key)) continue;
            Entry entry = item.getValue();
            entry.pending = false;
            entry.value = value;
            break;
        }
        return value != null;
    }

    synchronized boolean unavailable(String key, int generation) {
        if (!wants(key, generation)) return false;
        entries.remove(key);
        return true;
    }

    synchronized void close() {
        if (closed) return;
        closed = true;
        for (Map.Entry<String, Entry> item : entries.entrySet()) retire(item.getKey(), item.getValue());
        entries.clear();
    }

    synchronized int size() { return entries.size(); }

    /** The native test agent observes readiness without affecting eviction order. */
    synchronized boolean ready(String key) {
        for (Map.Entry<String, Entry> item : entries.entrySet()) {
            if (item.getKey().equals(key)) return item.getValue().value != null;
        }
        return false;
    }

    private void retire(String key, Entry entry) {
        entry.retired = true;
        if (entry.pending) requester.request(key, entry.generation, true);
        if (entry.value != null && entry.refs == 0) dispose.accept(entry.value);
    }
}

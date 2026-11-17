package dev.oneo.nsgo;

import java.util.HashMap;
import java.util.Map;

/** Exercises the runtime's actual cache with resources that detect double frees. */
public final class CanvasImagesCheck {
    static final class Picture {
        boolean freed;
        void free() {
            check(!freed, "double free");
            freed = true;
        }
    }

    static void check(boolean ok, String message) {
        if (!ok) throw new AssertionError(message);
    }

    public static void main(String[] args) throws Exception {
        Map<String, Integer> requests = new HashMap<>();
        Map<String, Integer> cancellations = new HashMap<>();
        CanvasImages<Picture> cache = new CanvasImages<>((key, generation, cancel) -> {
            (cancel ? cancellations : requests).put(key, generation);
        }, Picture::free);
        Picture first = null;
        for (int i = 0; i < 1000; i++) {
            String key = "image-" + i;
            check(cache.acquire(key) == null, "new source was already loaded");
            Picture pic = new Picture();
            check(cache.complete(key, requests.get(key), pic), "current request rejected");
            if (i == 0) first = pic;
            check(cache.size() <= 64, "unbounded decoded cache");
        }
        check(first.freed, "evicted bitmap was not recycled");
        int oldGeneration = requests.get("image-0");
        check(cache.acquire("image-0") == null, "eviction did not reload");
        int generation = requests.get("image-0");
        check(generation != oldGeneration, "request generation reused");
        Picture stale = new Picture();
        check(!cache.complete("image-0", oldGeneration, stale) && stale.freed, "stale response survived");
        Picture reloaded = new Picture();
        cache.complete("image-0", generation, reloaded);
        CanvasImages<Picture>.Lease lease = cache.acquire("image-0");
        check(lease.value == reloaded, "reloaded image differs");
        for (int i = 0; i < 1000; i++) {
            cache.acquire("pending-" + i);
            check(cache.size() <= 64, "unbounded pending cache");
        }
        check(cancellations.containsKey("pending-0"), "evicted load was not cancelled");
        check(!reloaded.freed, "GL frame's leased bitmap freed early");
        Thread gl = new Thread(lease::close);
        gl.start(); gl.join();
        lease.close();
        check(reloaded.freed, "bitmap leaked after GL frame");
        for (int i = 0; i < 1000; i++) {
            String key = "failed-" + i;
            cache.acquire(key);
            cache.complete(key, requests.get(key), null);
            cache.acquire(key);
            check(cache.size() <= 64, "unbounded failed cache");
        }
        cache.acquire("recent");
        int recent = requests.get("recent");
        for (int i = 0; i < 100; i++) {
            cache.acquire("recent");
            cache.acquire("other-" + i);
        }
        check(requests.get("recent") == recent, "recently drawn image evicted");
        cache.acquire("removed-source");
        int removed = requests.get("removed-source");
        check(cache.unavailable("removed-source", removed), "obsolete source response lost");
        cache.acquire("removed-source");
        check(requests.get("removed-source") != removed, "remounted source stuck pending");
        CanvasImages<Picture> other = new CanvasImages<>((k, g, c) -> {}, Picture::free);
        other.acquire("same");
        Picture isolated = new Picture();
        other.complete("same", 1, isolated);
        cache.close(); cache.close();
        check(cache.size() == 0 && cache.acquire("closed") == null, "closed surface resurrected");
        check(!isolated.freed, "closing one surface freed another's image");
        Picture late = new Picture();
        check(!cache.complete("recent", recent, late) && late.freed, "closed surface accepted a late decode");
        other.close();
        check(isolated.freed, "surface close leaked decoded bitmap");
    }
}

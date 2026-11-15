package dev.oneo.nsgo;

/** Decisions shared by framework adapters and JVM fixtures. No Android dependency. */
final class SensorPolicy {
    static String provider(int api, boolean fine, boolean fused, boolean gps, boolean network, double accuracy) {
        if (api >= 31 && fused) return "fused";
        if (accuracy > 100 && network) return "network";
        if (fine && gps) return "gps";
        if (network) return "network";
        return null;
    }

    static boolean fresh(long sampleNanos, long startedNanos, long nowNanos, long maxAge) {
        return sampleNanos <= nowNanos && (sampleNanos >= startedNanos || maxAge > 0 && nowNanos-sampleNanos <= maxAge);
    }

    static int period(long intervalNanos, int minimumMicros) {
        return Math.max(minimumMicros, (int)Math.max(10000L, Math.min(1000000L, intervalNanos/1000L)));
    }
}

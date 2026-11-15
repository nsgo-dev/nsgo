package dev.oneo.nsgo;

public final class SensorCheck {
    private static void check(boolean value) { if (!value) throw new AssertionError(); }
    public static void main(String[] args) {
        check("fused".equals(SensorPolicy.provider(31, false, true, true, true, 5)));
        check("gps".equals(SensorPolicy.provider(30, true, true, true, true, 5)));
        check("network".equals(SensorPolicy.provider(28, false, false, true, true, 5)));
        check("network".equals(SensorPolicy.provider(28, true, false, true, true, 1000)));
        check(SensorPolicy.provider(28, false, false, true, false, 5) == null);
        check(!SensorPolicy.fresh(90, 100, 110, 0));
        check(SensorPolicy.fresh(105, 100, 110, 0));
        check(SensorPolicy.fresh(90, 100, 110, 20));
        check(!SensorPolicy.fresh(89, 100, 110, 20));
        check(!SensorPolicy.fresh(120, 100, 110, 20));
        check(SensorPolicy.period(1, 0) == 10000);
        check(SensorPolicy.period(20000000, 40000) == 40000);
        check(SensorPolicy.period(2000000000L, 0) == 1000000);
        System.out.println("sensor provider, freshness and rate fixtures passed");
    }
}

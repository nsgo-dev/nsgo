package dev.oneo.nsgo;

import android.app.Activity;
import android.graphics.Rect;
import android.os.Build;
import android.util.Log;
import java.lang.reflect.InvocationHandler;
import java.lang.reflect.Method;
import java.lang.reflect.Proxy;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.function.Consumer;

/**
 * The folds and hinges of an activity's window, from the androidx.window.extensions library the device's maker ships, which the manifest declares for an app that reads its fold geometry. The runtime reaches it by reflection, as Jetpack's androidx.window sits on it, so it bundles none of Jetpack's classes: WindowExtensionsProvider.getWindowExtensions(), its WindowLayoutComponent and addWindowLayoutInfoListener, whose FoldingFeatures give bounds in the window, a type and a state. Without the library or its component, or before API 30, it reports nothing, which the app sees as one region.
 */
final class Folds {
    private static final String TAG = "nsgo";

    /** The activities listened to, each with the views told of its reports: the library takes one listener for each context and ignores another, as Jetpack's ExtensionWindowLayoutInfoBackend shares one between its consumers. Used on the main thread. */
    private static final HashMap<Activity, Folds> listening = new HashMap<>();

    private final Object component;
    private final Object consumer;
    private final Method remove;
    private final Activity activity;
    /** What listen was given for each view listening. */
    private final ArrayList<Runnable> views = new ArrayList<>();
    /** The latest report: each feature's left, top, right and bottom in window pixels, its type and its state; and the window's bounds it was measured at. */
    private int[] features = new int[0];
    private Rect measured;

    private Folds(Activity activity, Object component, Object consumer, Method remove) {
        this.activity = activity;
        this.component = component;
        this.consumer = consumer;
        this.remove = remove;
    }

    /** Listens to activity's window layout for a view, calling changed on the main thread after each report from now on; null where the device reports none. The report the library sends as it registers calls no view, so a view sends its size with current() once this returns. Call on the main thread. */
    static Folds listen(Activity activity, Runnable changed) {
        if (activity == null || Build.VERSION.SDK_INT < 30) return null;
        Folds folds = listening.get(activity);
        if (folds == null) {
            folds = register(activity);
            if (folds == null) return null;
            listening.put(activity, folds);
        }
        folds.views.add(changed);
        return folds;
    }

    private static Folds register(Activity activity) {
        try {
            ClassLoader loader = Folds.class.getClassLoader();
            Class<?> provider = Class.forName("androidx.window.extensions.WindowExtensionsProvider", false, loader);
            Object extensions = provider.getMethod("getWindowExtensions").invoke(null);
            if (extensions == null) return null;
            // The interfaces' methods, as the maker's classes implementing them need not be public.
            Class<?> api = Class.forName("androidx.window.extensions.WindowExtensions", false, loader);
            int level = (Integer) api.getMethod("getVendorApiLevel").invoke(extensions);
            Object component = api.getMethod("getWindowLayoutComponent").invoke(extensions);
            if (level < 1 || component == null) return null;
            Class<?> type = Class.forName("androidx.window.extensions.layout.WindowLayoutComponent", false, loader);
            Folds[] self = new Folds[1];
            Consumer<Object> report = info -> activity.runOnUiThread(() -> {
                if (self[0] != null) {
                    self[0].take(info);
                    for (Runnable view : new ArrayList<>(self[0].views)) view.run();
                }
            });
            Object consumer;
            Method add, remove;
            if (level >= 2) {
                // From vendor level 2 a listener takes any visual context and the library's own Consumer.
                Class<?> consumerType = Class.forName("androidx.window.extensions.core.util.function.Consumer", false, loader);
                InvocationHandler handler = (proxy, method, args) -> {
                    switch (method.getName()) {
                        case "accept": report.accept(args[0]); return null;
                        case "hashCode": return System.identityHashCode(proxy);
                        case "equals": return proxy == args[0];
                        case "toString": return "nsgo folds";
                    }
                    return null;
                };
                consumer = Proxy.newProxyInstance(loader, new Class<?>[] {consumerType}, handler);
                add = type.getMethod("addWindowLayoutInfoListener", android.content.Context.class, consumerType);
                remove = type.getMethod("removeWindowLayoutInfoListener", consumerType);
            } else {
                consumer = report;
                add = type.getMethod("addWindowLayoutInfoListener", Activity.class, Consumer.class);
                remove = type.getMethod("removeWindowLayoutInfoListener", Consumer.class);
            }
            self[0] = new Folds(activity, component, consumer, remove);
            add.invoke(component, activity, consumer);
            return self[0];
        } catch (ClassNotFoundException | NoSuchMethodException missing) {
            return null;
        } catch (ReflectiveOperationException | RuntimeException | LinkageError e) {
            Log.w(TAG, "the device's window extensions failed", e);
            return null;
        }
    }

    /** Stops calling changed, which listen was given, and stops listening once no view is left. */
    void stop(Runnable changed) {
        views.remove(changed);
        if (!views.isEmpty()) return;
        listening.remove(activity);
        try {
            remove.invoke(component, consumer);
        } catch (ReflectiveOperationException | RuntimeException e) {
            Log.w(TAG, "the device's window extensions failed", e);
        }
    }

    /** Keeps the FoldingFeatures of a WindowLayoutInfo and the window's bounds now, which they were measured at. */
    private void take(Object info) {
        int[] out = new int[0];
        try {
            ClassLoader loader = Folds.class.getClassLoader();
            Class<?> layout = Class.forName("androidx.window.extensions.layout.WindowLayoutInfo", false, loader);
            List<?> list = (List<?>) layout.getMethod("getDisplayFeatures").invoke(info);
            Class<?> folding = Class.forName("androidx.window.extensions.layout.FoldingFeature", false, loader);
            int n = 0;
            out = new int[list.size() * 6];
            for (Object f : list) {
                if (!folding.isInstance(f)) continue;
                Rect b = (Rect) folding.getMethod("getBounds").invoke(f);
                out[n++] = b.left;
                out[n++] = b.top;
                out[n++] = b.right;
                out[n++] = b.bottom;
                out[n++] = (Integer) folding.getMethod("getType").invoke(f);
                out[n++] = (Integer) folding.getMethod("getState").invoke(f);
            }
            out = java.util.Arrays.copyOf(out, n);
        } catch (ReflectiveOperationException | RuntimeException e) {
            Log.w(TAG, "the device's window extensions failed", e);
        }
        features = out;
        measured = activity.getWindowManager().getCurrentWindowMetrics().getBounds();
    }

    /** Returns the latest report's features, six ints each, while the window has the bounds they were measured at; none once it changed, as their frames are of another size until the next report. */
    int[] current() {
        Rect now = activity.getWindowManager().getCurrentWindowMetrics().getBounds();
        return now.equals(measured) ? features : new int[0];
    }
}

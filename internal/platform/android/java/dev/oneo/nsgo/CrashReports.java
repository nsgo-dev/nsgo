package dev.oneo.nsgo;

import java.io.File;
import java.io.FileOutputStream;
import java.io.PrintWriter;
import java.io.StringWriter;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.atomic.AtomicBoolean;

/**
 * Installs a process-wide Thread.UncaughtExceptionHandler that writes uncaught
 * Java exceptions into <dataDir>/logs/crashes/active-native.log so package
 * crash in the Go process promotes them into pending CrashReport entries on
 * the next launch, then chains to any previously installed handler.
 */
final class CrashReports {
    private static final AtomicBoolean installed = new AtomicBoolean(false);
    private static volatile File crashFile;

    private CrashReports() {}

    static void install(File dataDir) {
        if (dataDir != null) {
            crashFile = new File(new File(new File(dataDir, "logs"), "crashes"), "active-native.log");
        }
        if (!installed.compareAndSet(false, true)) {
            return;
        }
        final Thread.UncaughtExceptionHandler previous = Thread.getDefaultUncaughtExceptionHandler();
        Thread.setDefaultUncaughtExceptionHandler((thread, throwable) -> {
            writeCrash(thread, throwable);
            if (previous != null) {
                previous.uncaughtException(thread, throwable);
            }
        });
    }

    private static void writeCrash(Thread thread, Throwable throwable) {
        File target = crashFile;
        if (target == null || throwable == null) {
            return;
        }
        try {
            File parent = target.getParentFile();
            if (parent != null) {
                parent.mkdirs();
            }
            StringWriter sw = new StringWriter();
            PrintWriter pw = new PrintWriter(sw);
            pw.print("Android Uncaught Exception on thread ");
            pw.println(thread != null ? thread.getName() : "unknown");
            throwable.printStackTrace(pw);
            pw.flush();
            try (FileOutputStream out = new FileOutputStream(target, false)) {
                out.write(sw.toString().getBytes(StandardCharsets.UTF_8));
                out.getFD().sync();
            }
        } catch (Throwable ignored) {
            // Never mask the original uncaught exception.
        }
    }
}

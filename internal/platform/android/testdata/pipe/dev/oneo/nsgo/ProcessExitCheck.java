package dev.oneo.nsgo;

import android.content.Context;
import java.io.File;
import java.io.IOException;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;

/** Real child exits must finish a test after its last stderr line. */
public final class ProcessExitCheck {
    public static void main(String[] args) throws Exception {
        if (args.length != 0) {
            child(args[0]);
            return;
        }
        for (String scenario : new String[]{"early", "exit", "panic", "pass", "fail", "error", "foreign", "app", "destroy"}) {
            engine(scenario);
        }
    }

    private static void child(String scenario) throws Exception {
        // Let the protocol reader finish before stderr arrives, as can happen when a Go panic closes stdout while its stack is still being read.
        System.out.close();
        Thread.sleep(100);
        switch (scenario) {
            case "panic":
                System.err.println("panic: exit fixture");
                for (int i = 0; i < 2000; i++) System.err.println("stack frame " + i);
                System.exit(2);
                break;
            case "pass": System.err.println("nsgo-test-exit current-test 0"); break;
            case "fail": System.err.println("nsgo-test-exit current-test 1"); System.exit(1); break;
            case "error": System.err.println("nsgo-test-error current-test fixture failure"); break;
            case "foreign": System.err.println("nsgo-test-exit old-test 0"); break;
            case "exit": System.exit(1); break;
            case "destroy": Thread.sleep(60_000); break;
        }
    }

    private static void engine(String scenario) throws Exception {
        String java = new File(System.getProperty("java.home"), "bin/java").getPath();
        ProcessBuilder builder = new ProcessBuilder(java, "-Xmx32m", "-cp", System.getProperty("java.class.path"),
                ProcessExitCheck.class.getName(), scenario);
        builder.environment().remove("NSGO_TEST_RUN");
        if (!scenario.equals("app")) builder.environment().put("NSGO_TEST_RUN", "current-test");
        NsgoEngine engine = new NsgoEngine(new Context());
        CountDownLatch protocolEnded = new CountDownLatch(1);
        CountDownLatch terminalLog = new CountDownLatch(1);
        List<String> logs = new ArrayList<>();
        engine.start(builder);
        engine.attach(new NsgoEngine.Listener() {
            public void onMessage(int kind, byte[] payload) { throw new AssertionError("unexpected message"); }
            public void onProcessEnded(IOException error) { protocolEnded.countDown(); }
            public void onLog(String line) {
                synchronized (logs) { logs.add(line); }
                if (line.startsWith("nsgo-test-error current-test ") || line.startsWith("nsgo-test-exit current-test ")) {
                    terminalLog.countDown();
                }
            }
        });
        try {
            check(protocolEnded.await(5, TimeUnit.SECONDS), scenario + ": protocol did not end");
            if (scenario.equals("destroy") || scenario.equals("app")) {
                if (scenario.equals("destroy")) engine.destroy();
                check(!terminalLog.await(300, TimeUnit.MILLISECONDS), scenario + ": unexpected test failure");
                return;
            }
            check(terminalLog.await(5, TimeUnit.SECONDS), scenario + ": test exit was never reported");
            synchronized (logs) {
                String last = logs.get(logs.size() - 1);
                switch (scenario) {
                    case "pass": check(last.equals("nsgo-test-exit current-test 0"), last); break;
                    case "fail": check(last.equals("nsgo-test-exit current-test 1"), last); break;
                    case "error": check(last.equals("nsgo-test-error current-test fixture failure"), last); break;
                    default:
                        int status = scenario.equals("panic") ? 2 : scenario.equals("exit") ? 1 : 0;
                        check(last.equals("nsgo-test-error current-test the test process ended without its exit code (status " + status + ")"), last);
                }
                if (scenario.equals("panic")) {
                    check(logs.size() == 2002 && logs.get(0).equals("panic: exit fixture") && logs.get(2000).equals("stack frame 1999"),
                            "panic output was lost or followed the failure: " + logs.size());
                }
            }
        } finally {
            engine.destroy();
        }
    }

    private static void check(boolean ok, String why) {
        if (!ok) throw new AssertionError(why);
    }
}

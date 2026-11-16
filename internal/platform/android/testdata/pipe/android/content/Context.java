package android.content;

import java.io.File;

/** Only the application-context calls the engine makes; no Android UI. */
public class Context {
    public Context getApplicationContext() { return this; }
    public File getFilesDir() { return new File("."); }
    public Intent registerReceiver(BroadcastReceiver receiver, IntentFilter filter) { return null; }
    public void unregisterReceiver(BroadcastReceiver receiver) {}
}

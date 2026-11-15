package dev.oneo.nsgo;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.content.pm.ServiceInfo;
import android.os.Build;
import android.os.IBinder;
import java.util.LinkedHashMap;

/** Explicit, non-restarting location foreground service, which keeps the app's process while a background location stream runs; NsgoActivity keeps the view off screen meanwhile (NsgoView.keepOffScreen). All state is UI-thread owned. */
public final class LocationService extends Service {
    private static final LinkedHashMap<Object, Entry> entries = new LinkedHashMap<>();
    private static LocationService current;
    private static final String CHANNEL = "nsgo.location";
    private static final int NOTIFICATION = 0x6e6c6f;
    private static final class Entry {
        String title, body; Runnable failed;
        Entry(String title, String body, Runnable failed) { this.title = title; this.body = body; this.failed = failed; }
    }

    static void add(Context context, Object token, String title, String body, Runnable failed) {
        entries.put(token, new Entry(title, body, failed));
        try { context.startForegroundService(new Intent(context, LocationService.class)); }
        catch (RuntimeException e) { entries.remove(token); throw e; }
    }

    static void remove(Context context, Object token) {
        entries.remove(token);
        if (entries.isEmpty()) {
            context.stopService(new Intent(context, LocationService.class));
        } else if (current != null) current.show();
    }

    @Override public int onStartCommand(Intent intent, int flags, int startId) {
        current = this;
        if (entries.isEmpty()) { stopSelf(); return START_NOT_STICKY; }
        show();
        return START_NOT_STICKY;
    }

    private void show() {
        if (entries.isEmpty()) return;
        Entry entry = entries.values().iterator().next();
        NotificationManager manager = getSystemService(NotificationManager.class);
        manager.createNotificationChannel(new NotificationChannel(CHANNEL, entry.title, NotificationManager.IMPORTANCE_LOW));
        int icon = getApplicationInfo().icon;
        Notification.Builder builder = new Notification.Builder(this, CHANNEL)
            .setSmallIcon(icon == 0 ? android.R.drawable.ic_menu_mylocation : icon)
            .setContentTitle(entry.title).setContentText(entry.body).setOngoing(true);
        Intent launch = getPackageManager().getLaunchIntentForPackage(getPackageName());
        if (launch != null) builder.setContentIntent(PendingIntent.getActivity(this, 0, launch, PendingIntent.FLAG_IMMUTABLE | PendingIntent.FLAG_UPDATE_CURRENT));
        try {
            if (Build.VERSION.SDK_INT >= 29) startForeground(NOTIFICATION, builder.build(), ServiceInfo.FOREGROUND_SERVICE_TYPE_LOCATION);
            else startForeground(NOTIFICATION, builder.build());
        } catch (RuntimeException e) {
            for (Entry item : new java.util.ArrayList<>(entries.values())) item.failed.run();
            stopSelf();
        }
    }

    @Override public void onDestroy() {
        current = null;
        for (Entry entry : new java.util.ArrayList<>(entries.values())) entry.failed.run();
        entries.clear();
        stopForeground(STOP_FOREGROUND_REMOVE);
        super.onDestroy();
    }

    @Override public IBinder onBind(Intent intent) { return null; }
}

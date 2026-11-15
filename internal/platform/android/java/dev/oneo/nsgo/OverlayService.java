package dev.oneo.nsgo;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Intent;
import android.content.pm.ServiceInfo;
import android.os.Build;
import android.os.IBinder;
import android.util.Log;

/**
 * The foreground service that keeps the app's process while an overlay over other apps shows (Overlays), of the specialUse type, with an ongoing notification titled with the app's name that brings the app back. nsgo build declares it, with FOREGROUND_SERVICE and FOREGROUND_SERVICE_SPECIAL_USE, only for an app whose android.permissions list SYSTEM_ALERT_WINDOW. Without the notification permission it runs all the same, its notification only in the system's list of running apps. Its callbacks run on the main thread, as Overlays does.
 */
public final class OverlayService extends Service {
    private static final String TAG = "nsgo";
    private static final String CHANNEL = "nsgo.overlay";
    private static final int NOTIFICATION = 0x6e6f76;

    /** The overlays that start it, which hear it end unasked. */
    static Overlays owner;
    /** Whether the overlays want it running: a start on its way as they stop it stops itself once in the foreground. */
    static boolean wanted;
    /** The service in the foreground, which the overlays stop; null before onStartCommand and after it stops. */
    static OverlayService running;
    /** Whether the overlays stopped this one, which then ends without telling them. */
    private boolean asked;

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        try {
            if (Build.VERSION.SDK_INT >= 34) {
                startForeground(NOTIFICATION, notification(), ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE);
            } else {
                startForeground(NOTIFICATION, notification());
            }
        } catch (RuntimeException e) {
            // ForegroundServiceStartNotAllowedException, or a permission the manifest lacks.
            Log.w(TAG, "the overlay's service cannot go into the foreground", e);
            boolean ended = wanted;
            stop();
            if (ended && owner != null) owner.serviceEnded(true);
            return START_NOT_STICKY;
        }
        running = this;
        // The last overlay closed before this start arrived: stopping the service before startForeground would have crashed the app.
        if (!wanted) stop();
        // The process does not come back for it: an overlay is not restored after the process dies.
        return START_NOT_STICKY;
    }

    /** Stops this service for the overlays, which hear nothing of its end. */
    void stop() {
        asked = true;
        if (running == this) running = null;
        stopSelf();
    }

    @Override
    public void onDestroy() {
        super.onDestroy();
        if (running == this) running = null;
        // Stopped by the system or the user, the overlays it kept end.
        if (!asked && wanted && owner != null) owner.serviceEnded(false);
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }

    /** The ongoing notification: the app's name and icon, in a quiet channel named after the app, bringing the app back when pressed. */
    private Notification notification() {
        CharSequence name = getApplicationInfo().loadLabel(getPackageManager());
        NotificationManager manager = getSystemService(NotificationManager.class);
        if (manager != null && manager.getNotificationChannel(CHANNEL) == null) {
            manager.createNotificationChannel(new NotificationChannel(CHANNEL, name, NotificationManager.IMPORTANCE_LOW));
        }
        int icon = getApplicationInfo().icon;
        Notification.Builder b = new Notification.Builder(this, CHANNEL)
                .setSmallIcon(icon != 0 ? icon : android.R.drawable.ic_dialog_info)
                .setContentTitle(name)
                .setOngoing(true);
        Intent launch = getPackageManager().getLaunchIntentForPackage(getPackageName());
        if (launch != null) {
            b.setContentIntent(PendingIntent.getActivity(this, 0, launch,
                    PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE));
        }
        if (Build.VERSION.SDK_INT >= 31) b.setForegroundServiceBehavior(Notification.FOREGROUND_SERVICE_IMMEDIATE);
        return b.build();
    }
}

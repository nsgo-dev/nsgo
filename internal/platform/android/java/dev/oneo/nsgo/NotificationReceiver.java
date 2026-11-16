package dev.oneo.nsgo;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;

/**
 * Hears the alarms of scheduled notifications and shows them, in the app's process without its activity or its Go (Notifications), and sets the alarms again after a boot, which Android forgets them across, and after an update of the app. The runtime's manifest declares it, not exported: alarms name it, and the system sends the boot's broadcasts to it all the same.
 */
public final class NotificationReceiver extends BroadcastReceiver {
    @Override
    public void onReceive(Context context, Intent intent) {
        String action = intent.getAction();
        if (Notifications.ALARM.equals(action)) {
            if (intent.getData() != null) Notifications.fire(context, intent.getData().getSchemeSpecificPart());
        } else if (Intent.ACTION_BOOT_COMPLETED.equals(action) || Intent.ACTION_MY_PACKAGE_REPLACED.equals(action)) {
            Notifications.rearm(context);
        }
    }
}

package dev.oneo.nsgo;

import android.app.Activity;
import android.content.MutableContextWrapper;
import android.content.res.Configuration;
import android.graphics.Color;
import android.os.Build;
import android.os.Bundle;
import android.view.ContextThemeWrapper;
import android.view.ViewGroup;
import android.view.Window;
import android.view.WindowManager;

/**
 * The launcher activity of a standalone NSGo app: one NsgoView filling the
 * window edge to edge, behind the system bars it reports as the safe area.
 * Its manifest entry, as in AndroidManifest.xml, declares the
 * configuration changes it handles, or the system restarts the app on each.
 */
public class NsgoActivity extends Activity {
    /**
     * The view of an activity that ended while the app's overlays over other apps kept its process running, as when the user removes the app's task, which the next activity shows again: the engine serves the one view that first attached to it, so the view moves to the new activity, its context wrapping that one. Null otherwise.
     */
    private static NsgoView kept;

    private NsgoView view;
    /** The theme the window shows, which a kept view's context takes while no activity holds it. */
    private int theme = android.R.style.Theme_DeviceDefault_NoActionBar;

    @Override
    protected void onCreate(Bundle state) {
        // The manifest's theme is dark; web views take prefers-color-scheme
        // from the theme, so it follows the system's mode at launch.
        int night = getResources().getConfiguration().uiMode & Configuration.UI_MODE_NIGHT_MASK;
        if (night != Configuration.UI_MODE_NIGHT_YES) {
            theme = android.R.style.Theme_DeviceDefault_Light_NoActionBar;
            setTheme(theme);
        }
        super.onCreate(state);
        Window window = getWindow();
        WindowManager.LayoutParams lp = window.getAttributes();
        lp.layoutInDisplayCutoutMode = WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES;
        window.setAttributes(lp);
        // Edge to edge, this reports the keyboard in the insets rather
        // than resizing the view.
        window.setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_ADJUST_RESIZE);
        window.setStatusBarColor(Color.TRANSPARENT);
        window.setNavigationBarColor(Color.TRANSPARENT);
        if (Build.VERSION.SDK_INT >= 30) window.setDecorFitsSystemWindows(false);
        // The view lays itself out behind the system bars and styles them.
        NsgoView again = kept;
        kept = null;
        if (again != null && again.keptForOverlays()) {
            ((MutableContextWrapper) again.getContext()).setBaseContext(this);
            view = again;
            view.showAgain();
        } else {
            view = new NsgoView(new MutableContextWrapper(this));
            view.setFlags(getIntent().getExtras());
            view.setTest(getIntent().getStringExtra(TestAgent.EXTRA));
        }
        setContentView(view);
        view.notificationPressed(getIntent().getStringExtra(Notifications.EXTRA));
        view.liveActionPressed(getIntent());
    }

    @Override
    protected void onNewIntent(android.content.Intent intent) {
        super.onNewIntent(intent);
        // A notification's press, the activity running.
        view.notificationPressed(intent.getStringExtra(Notifications.EXTRA));
        view.liveActionPressed(intent);
    }

    /**
     * Restyles the window in the appearance the app shows: the theme the
     * views it makes from now on read, and the window's background.
     */
    void followAppearance(boolean dark) {
        theme = dark ? android.R.style.Theme_DeviceDefault_NoActionBar
                : android.R.style.Theme_DeviceDefault_Light_NoActionBar;
        setTheme(theme);
        getTheme().applyStyle(theme, true);
        android.content.res.TypedArray a = getTheme().obtainStyledAttributes(new int[] {android.R.attr.windowBackground});
        getWindow().setBackgroundDrawable(a.getDrawable(0));
        a.recycle();
    }

    @Override
    public void onBackPressed() {
        // Back arrives here before API 33, or without predictive back.
        if (!view.back()) super.onBackPressed();
    }

    @Override
    protected void onActivityResult(int request, int result, android.content.Intent data) {
        super.onActivityResult(request, result, data);
        view.activityResult(request, result, data);
    }

    @Override
    public void onRequestPermissionsResult(int request, String[] permissions, int[] results) {
        super.onRequestPermissionsResult(request, permissions, results);
        view.permissionResult(request, permissions, results);
    }

    /** Ends the app with the activity, unless its overlays over other apps keep it running, off screen, for the next activity. */
    @Override
    protected void onDestroy() {
        if (view.keepForOverlays()) {
            ((ViewGroup) view.getParent()).removeView(view);
            ((MutableContextWrapper) view.getContext()).setBaseContext(new ContextThemeWrapper(getApplicationContext(), theme));
            kept = view;
        } else {
            view.stop();
        }
        super.onDestroy();
    }
}

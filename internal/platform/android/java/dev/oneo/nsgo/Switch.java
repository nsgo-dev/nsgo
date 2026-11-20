package dev.oneo.nsgo;

import android.content.Context;
import android.content.res.Configuration;
import android.graphics.Canvas;
import android.graphics.Paint;
import android.graphics.RectF;
import android.view.Gravity;
import android.view.View;
import android.view.accessibility.AccessibilityNodeInfo;
import android.widget.CompoundButton;

/** Material 3 switch drawn with framework primitives. */
public final class Switch extends CompoundButton {
    private final Paint paint = new Paint(Paint.ANTI_ALIAS_FLAG);
    private final float density;
    private final RectF track = new RectF();
    private boolean updating;
    private String size = "";

    public Switch(Context context) {
        super(context);
        density = context.getResources().getDisplayMetrics().density;
        setGravity(Gravity.CENTER);
        setClickable(true);
        setFocusable(true);
        setOnCheckedChangeListener((button, checked) -> {
            if (!updating) invalidate();
        });
        setButtonDrawable(null);
        setBackground(null);
    }

    public void setChecked(boolean checked) {
        updating = true;
        super.setChecked(checked);
        updating = false;
        invalidate();
    }

    @Override
    public void toggle() {
        super.toggle();
        invalidate();
    }

    public void setDisabled(boolean disabled) {
        setEnabled(!disabled);
    }

    public void setSize(String value) {
        size = value == null ? "" : value;
        requestLayout();
        invalidate();
    }

    @Override
    protected void onMeasure(int widthSpec, int heightSpec) {
        int width = dp(size.equals("small") ? 44 : size.equals("large") ? 60 : 52);
        int height = dp(size.equals("small") ? 28 : size.equals("large") ? 36 : 32);
        setMeasuredDimension(resolveSize(width, widthSpec), resolveSize(height, heightSpec));
    }

    @Override
    protected void onDraw(Canvas canvas) {
        super.onDraw(canvas);
        float w = getWidth(), h = getHeight();
        float trackW = dp(size.equals("small") ? 44 : size.equals("large") ? 60 : 52);
        float trackH = dp(size.equals("small") ? 28 : size.equals("large") ? 36 : 32);
        float left = (w - trackW) / 2f, top = (h - trackH) / 2f;
        track.set(left, top, left + trackW, top + trackH);
        boolean dark = (getResources().getConfiguration().uiMode & Configuration.UI_MODE_NIGHT_MASK) == Configuration.UI_MODE_NIGHT_YES;
        int accent = resolveAccent();
        int offTrack = dark ? 0xff49454f : 0xff79747e;
        int offOutline = dark ? 0xff938f99 : 0xff79747e;
        int onTrack = disabledColor(accent, dark ? 0xff4d4a50 : 0xffd0bcff);
        paint.setStyle(Paint.Style.FILL);
        paint.setColor(isChecked() ? onTrack : (dark ? 0xff1d1b20 : 0xfffef7ff));
        canvas.drawRoundRect(track, trackH / 2f, trackH / 2f, paint);
        if (!isChecked()) {
            paint.setStyle(Paint.Style.STROKE);
            paint.setStrokeWidth(dp(2));
            paint.setColor(isEnabled() ? offOutline : disabledColor(offOutline, dark ? 0xff49454f : 0xffe6e0e9));
            canvas.drawRoundRect(track, trackH / 2f, trackH / 2f, paint);
            paint.setStyle(Paint.Style.FILL);
        }
        float off = dp(size.equals("small") ? 8 : 8);
        float offThumb = dp(size.equals("small") ? 16 : 16);
        float onThumb = dp(size.equals("large") ? 24 : 24);
        float thumb = isChecked() ? onThumb : offThumb;
        float minX = left + off + thumb / 2f, maxX = left + trackW - off - thumb / 2f;
        float cx = isChecked() ? maxX : minX;
        paint.setColor(isChecked() ? 0xffffffff : (dark ? 0xffcac4d0 : 0xff79747e));
        if (!isEnabled()) paint.setColor(disabledColor(paint.getColor(), dark ? 0xff49454f : 0xffe6e0e9));
        canvas.drawCircle(cx, top + trackH / 2f, thumb / 2f, paint);
    }

    @Override
    public void onInitializeAccessibilityNodeInfo(AccessibilityNodeInfo info) {
        super.onInitializeAccessibilityNodeInfo(info);
        info.setClassName("android.widget.Switch");
        info.setCheckable(true);
        info.setChecked(isChecked());
    }

    private int resolveAccent() {
        android.util.TypedValue value = new android.util.TypedValue();
        if (getContext().getTheme().resolveAttribute(android.R.attr.colorControlActivated, value, true)) {
            return value.data;
        }
        if (getContext().getTheme().resolveAttribute(android.R.attr.colorAccent, value, true)) {
            return value.data;
        }
        return 0xff6750a4;
    }

    private int disabledColor(int color, int fallback) {
        return isEnabled() ? color : fallback;
    }

    private int dp(float value) { return Math.round(value * density); }
}

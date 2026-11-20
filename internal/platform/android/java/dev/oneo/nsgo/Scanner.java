package dev.oneo.nsgo;

import android.app.Activity;
import android.app.Dialog;
import android.content.Context;
import android.content.pm.PackageManager;
import android.graphics.Matrix;
import android.graphics.SurfaceTexture;
import android.graphics.drawable.ColorDrawable;
import android.hardware.camera2.CameraAccessException;
import android.hardware.camera2.CameraCaptureSession;
import android.hardware.camera2.CameraCharacteristics;
import android.hardware.camera2.CameraDevice;
import android.hardware.camera2.CameraManager;
import android.hardware.camera2.CaptureRequest;
import android.hardware.camera2.params.StreamConfigurationMap;
import android.graphics.ImageFormat;
import android.media.Image;
import android.media.ImageReader;
import android.os.Handler;
import android.os.HandlerThread;
import android.os.SystemClock;
import android.util.Size;
import android.view.Gravity;
import android.view.Surface;
import android.view.TextureView;
import android.view.ViewGroup;
import android.view.WindowManager;
import android.widget.FrameLayout;
import android.widget.ImageButton;
import java.nio.ByteBuffer;
import java.util.Arrays;

/**
 * A code scanner (core.Scope.ScanCode): Camera2's preview of the back camera full screen in a dialog, with a close button the system words, sending the luminance of a few frames a second to Go, which reads them (internal/platform/codescan) and stops the scanner once one holds a code. Android reads no codes itself without Google Play services, which the runtime does not link. It needs CAMERA in the app's manifest, which it asks for once.
 */
final class Scanner {
    static final int PERMISSION_REQUEST = 0x6e79;
    /** The longest side of the frames Go reads: codes a phone's camera frames from a hand's length read at it, and larger ones only cost the pipe. */
    private static final int MAX_SIDE = 960;
    /** The least time between frames sent to Go. */
    private static final long FRAME_INTERVAL_MS = 200;

    private final NsgoView view;
    private final int surface, count;
    private final Runnable gone;
    private Dialog dialog;
    private TextureView preview;
    private HandlerThread thread;
    private Handler camera;
    private CameraDevice device;
    private CameraCaptureSession session;
    private ImageReader reader;
    private Size size;
    private int sensorOrientation;
    private volatile long lastFrame;
    private boolean ended;

    Scanner(NsgoView view, int surface, int count, Runnable gone) {
        this.view = view;
        this.surface = surface;
        this.count = count;
        this.gone = gone;
    }

    /** Whether the device has a camera and the app's manifest asks for CAMERA, which reading its frames needs (Environment.ScanCode). */
    static boolean available(Context context) {
        return Services.hasCamera(context) && declaresCamera(context);
    }

    private static boolean declaresCamera(Context context) {
        try {
            String[] asked = context.getPackageManager()
                    .getPackageInfo(context.getPackageName(), PackageManager.GET_PERMISSIONS)
                    .requestedPermissions;
            return asked != null && Arrays.asList(asked).contains(Services.CAMERA_PERMISSION);
        } catch (PackageManager.NameNotFoundException e) {
            return false;
        }
    }

    boolean answers(int surface, int count) {
        return this.surface == surface && this.count == count;
    }

    /** Shows the scanner over activity once the user allowed the camera, asking first where they have not. */
    void start(Activity activity) {
        if (activity == null) {
            fail("no activity shows the camera");
            return;
        }
        if (!declaresCamera(activity)) {
            fail("the app's manifest asks for no CAMERA (nsgo.yaml's android.permissions)");
            return;
        }
        if (activity.checkSelfPermission(Services.CAMERA_PERMISSION) != PackageManager.PERMISSION_GRANTED) {
            activity.requestPermissions(new String[] {Services.CAMERA_PERMISSION}, PERMISSION_REQUEST);
            return;
        }
        show(activity);
    }

    void permissionResult(boolean granted) {
        if (ended) return;
        Activity activity = view.activity();
        if (granted && activity != null) {
            show(activity);
        } else {
            fail("camera access denied");
        }
    }

    private void show(Activity activity) {
        float density = activity.getResources().getDisplayMetrics().density;
        FrameLayout root = new FrameLayout(activity);
        root.setBackgroundColor(0xff000000);
        preview = new TextureView(activity);
        root.addView(preview, new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
        ImageButton close = new ImageButton(activity);
        close.setImageResource(android.R.drawable.ic_menu_close_clear_cancel);
        close.setContentDescription(activity.getString(android.R.string.cancel));
        close.setBackground(new ColorDrawable(0x66000000));
        int pad = Math.round(12 * density);
        close.setPadding(pad, pad, pad, pad);
        FrameLayout.LayoutParams at = new FrameLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT, Gravity.BOTTOM | Gravity.CENTER_HORIZONTAL);
        at.bottomMargin = Math.round(48 * density);
        root.addView(close, at);
        close.setOnClickListener(v -> close());
        dialog = new Dialog(activity, android.R.style.Theme_Black_NoTitleBar_Fullscreen);
        dialog.setContentView(root);
        // Back closes it, as the button does.
        dialog.setOnCancelListener(d -> close());
        if (dialog.getWindow() != null) dialog.getWindow().addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);
        dialog.show();
        thread = new HandlerThread("nsgo-scan");
        thread.start();
        camera = new Handler(thread.getLooper());
        preview.setSurfaceTextureListener(new TextureView.SurfaceTextureListener() {
            @Override public void onSurfaceTextureAvailable(SurfaceTexture texture, int width, int height) { open(activity, texture); }
            @Override public void onSurfaceTextureSizeChanged(SurfaceTexture texture, int width, int height) { fit(activity); }
            @Override public boolean onSurfaceTextureDestroyed(SurfaceTexture texture) { return true; }
            @Override public void onSurfaceTextureUpdated(SurfaceTexture texture) {}
        });
    }

    /** Opens the back camera, else the first, into the preview and a reader of its frames. */
    private void open(Activity activity, SurfaceTexture texture) {
        if (ended) return;
        CameraManager manager = (CameraManager) activity.getSystemService(Context.CAMERA_SERVICE);
        try {
            String chosen = null;
            for (String id : manager.getCameraIdList()) {
                Integer facing = manager.getCameraCharacteristics(id).get(CameraCharacteristics.LENS_FACING);
                if (chosen == null || (facing != null && facing == CameraCharacteristics.LENS_FACING_BACK)) chosen = id;
                if (facing != null && facing == CameraCharacteristics.LENS_FACING_BACK) break;
            }
            if (chosen == null) {
                fail("this device has no camera");
                return;
            }
            CameraCharacteristics c = manager.getCameraCharacteristics(chosen);
            Integer orientation = c.get(CameraCharacteristics.SENSOR_ORIENTATION);
            sensorOrientation = orientation == null ? 90 : orientation;
            StreamConfigurationMap map = c.get(CameraCharacteristics.SCALER_STREAM_CONFIGURATION_MAP);
            size = frameSize(map.getOutputSizes(ImageFormat.YUV_420_888));
            texture.setDefaultBufferSize(size.getWidth(), size.getHeight());
            fit(activity);
            reader = ImageReader.newInstance(size.getWidth(), size.getHeight(), ImageFormat.YUV_420_888, 2);
            reader.setOnImageAvailableListener(this::frame, camera);
            manager.openCamera(chosen, new CameraDevice.StateCallback() {
                @Override public void onOpened(CameraDevice d) {
                    if (ended) {
                        d.close();
                        return;
                    }
                    device = d;
                    capture(new Surface(texture));
                }
                @Override public void onDisconnected(CameraDevice d) {
                    d.close();
                    view.post(() -> fail("the camera was disconnected"));
                }
                @Override public void onError(CameraDevice d, int error) {
                    d.close();
                    view.post(() -> fail("the camera failed (" + error + ")"));
                }
            }, camera);
        } catch (CameraAccessException | SecurityException | IllegalArgumentException e) {
            fail(String.valueOf(e.getMessage()));
        }
    }

    /** The largest frame size whose longer side is at most MAX_SIDE, else the smallest. */
    private static Size frameSize(Size[] sizes) {
        Size best = null, smallest = null;
        for (Size s : sizes) {
            long area = (long) s.getWidth() * s.getHeight();
            if (smallest == null || area < (long) smallest.getWidth() * smallest.getHeight()) smallest = s;
            if (Math.max(s.getWidth(), s.getHeight()) > MAX_SIDE) continue;
            if (best == null || area > (long) best.getWidth() * best.getHeight()) best = s;
        }
        return best != null ? best : smallest;
    }

    /** Fills the preview with the camera's picture, upright and cropped rather than stretched. */
    private void fit(Activity activity) {
        if (preview == null || size == null) return;
        preview.post(() -> {
            float w = preview.getWidth(), h = preview.getHeight();
            if (w == 0 || h == 0) return;
            int rotation = activity.getWindowManager().getDefaultDisplay().getRotation();
            boolean upright = rotation == Surface.ROTATION_0 || rotation == Surface.ROTATION_180;
            // The sensor lies on its side: upright, its picture is as wide as the buffer is tall.
            boolean turned = upright == (sensorOrientation % 180 != 0);
            float contentW = turned ? size.getHeight() : size.getWidth(), contentH = turned ? size.getWidth() : size.getHeight();
            float scale = Math.max(w / contentW, h / contentH);
            Matrix m = new Matrix();
            m.setScale(contentW * scale / w, contentH * scale / h, w / 2, h / 2);
            preview.setTransform(m);
        });
    }

    private void capture(Surface shown) {
        try {
            CaptureRequest.Builder request = device.createCaptureRequest(CameraDevice.TEMPLATE_PREVIEW);
            request.addTarget(shown);
            request.addTarget(reader.getSurface());
            request.set(CaptureRequest.CONTROL_AF_MODE, CaptureRequest.CONTROL_AF_MODE_CONTINUOUS_PICTURE);
            device.createCaptureSession(Arrays.asList(shown, reader.getSurface()), new CameraCaptureSession.StateCallback() {
                @Override public void onConfigured(CameraCaptureSession s) {
                    if (ended) {
                        s.close();
                        return;
                    }
                    session = s;
                    try {
                        s.setRepeatingRequest(request.build(), null, camera);
                    } catch (CameraAccessException | IllegalStateException e) {
                        view.post(() -> fail(String.valueOf(e.getMessage())));
                    }
                }
                @Override public void onConfigureFailed(CameraCaptureSession s) {
                    view.post(() -> fail("the camera could not be configured"));
                }
            }, camera);
        } catch (CameraAccessException | IllegalStateException e) {
            view.post(() -> fail(String.valueOf(e.getMessage())));
        }
    }

    /** Sends a frame's luminance to Go, a few a second, on the camera's thread. */
    private void frame(ImageReader r) {
        Image image = r.acquireLatestImage();
        if (image == null) return;
        try {
            long now = SystemClock.uptimeMillis();
            if (ended || now - lastFrame < FRAME_INTERVAL_MS) return;
            lastFrame = now;
            int width = image.getWidth(), height = image.getHeight();
            Image.Plane y = image.getPlanes()[0];
            ByteBuffer buffer = y.getBuffer();
            int rowStride = y.getRowStride(), pixelStride = y.getPixelStride();
            byte[] lum = new byte[width * height];
            if (pixelStride == 1 && rowStride == width) {
                buffer.get(lum, 0, Math.min(lum.length, buffer.remaining()));
            } else {
                byte[] row = new byte[rowStride];
                for (int line = 0; line < height; line++) {
                    buffer.position(line * rowStride);
                    int n = Math.min(rowStride, buffer.remaining());
                    buffer.get(row, 0, n);
                    for (int x = 0; x < width && x * pixelStride < n; x++) lum[line * width + x] = row[x * pixelStride];
                }
            }
            view.scanReport(surface, count, Protocol.SCAN_FRAME, out -> {
                out.writeInt(width);
                out.writeInt(height);
                out.writeInt(lum.length);
                out.write(lum);
            });
        } finally {
            image.close();
        }
    }

    /** Closes the scanner as the user does, reporting it to Go. */
    void close() {
        if (end()) view.scanReport(surface, count, Protocol.SCAN_CLOSED, null);
    }

    /** Closes the scanner once Go read a code, reporting nothing. */
    void stop() {
        end();
    }

    private void fail(String why) {
        if (end()) view.scanReport(surface, count, Protocol.SCAN_FAILED, out -> Protocol.writeString(out, why));
    }

    /** Lets go of the camera and the dialog, on the UI thread, and reports whether the scanner showed until now. */
    private boolean end() {
        if (ended) return false;
        ended = true;
        Handler h = camera;
        CameraCaptureSession s = session;
        CameraDevice d = device;
        ImageReader r = reader;
        HandlerThread t = thread;
        session = null;
        device = null;
        reader = null;
        if (h != null) {
            h.post(() -> {
                if (s != null) s.close();
                if (d != null) d.close();
                if (r != null) r.close();
                if (t != null) t.quitSafely();
            });
        }
        if (dialog != null) {
            dialog.setOnCancelListener(null);
            dialog.dismiss();
            dialog = null;
        }
        gone.run();
        return true;
    }
}

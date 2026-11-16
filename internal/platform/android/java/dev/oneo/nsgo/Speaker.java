package dev.oneo.nsgo;

import android.content.Context;
import android.content.Intent;
import android.media.AudioAttributes;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.speech.tts.TextToSpeech;
import android.speech.tts.UtteranceProgressListener;
import android.speech.tts.Voice;
import java.util.ArrayList;
import java.util.Locale;
import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

/**
 * Speaking text through TextToSpeech, as core's Speak: one engine for the process, made at the first request, which binds to the system's engine asynchronously, so the requests made before it is ready wait for it. One utterance speaks at a time, a new one stopping the one before; each reports its end to the view that asked once, as finished, stopped or failed with one of driver's Speak errors. Its state is the main thread's.
 */
final class Speaker {
    private Speaker() {}

    // What ITEM_SPEAK asks, as driver.SpeakCommand.
    static final int START = 0, STOP = 1, VOICES = 2;

    private static TextToSpeech tts;
    /** Whether the engine bound, failed to, or is still binding. */
    private static boolean ready, failed;
    private static final ArrayList<Runnable> waiting = new ArrayList<>();
    private static final Handler main = new Handler(Looper.getMainLooper());

    /** The utterance speaking: its view and request, and the ID TextToSpeech knows it by. */
    private static NsgoView owner;
    private static long current;
    private static String utterance;
    private static int sequence;
    /** The START waiting for the engine that is to speak, a later one or a STOP of it cancelling it. */
    private static NsgoView pendingView;
    private static long pending;

    /**
     * Whether the device has a speech engine, which Environment.Speak reports; an app targeting API 30 or later sees one only with the TTS_SERVICE query the runtime's manifest declares.
     */
    static boolean available(Context context) {
        return !context.getPackageManager().queryIntentServices(new Intent(TextToSpeech.Engine.INTENT_ACTION_TTS_SERVICE), 0).isEmpty();
    }

    /** Carries out ITEM_SPEAK count of view, on the main thread. */
    static void handle(NsgoView view, long count, int command, String text, String lang, String voice, float rate, float pitch) {
        if (command == STOP) {
            if (owner == view && current == count) stop();
            if (pendingView == view && pending == count) pendingView = null;
            return;
        }
        if (command == START) {
            stop();
            pendingView = view;
            pending = count;
        }
        withEngine(view.getContext(), () -> {
            if (command == START) {
                // Stopped, or another spoke, while the engine bound.
                if (pendingView != view || pending != count) return;
                pendingView = null;
            }
            if (failed) {
                view.answerSpeak(count, false, "synthesis-unavailable", "");
            } else if (command == VOICES) {
                view.answerSpeak(count, false, "", voices());
            } else {
                start(view, count, text, lang, voice, rate, pitch);
            }
        });
    }

    /** Runs then once the engine bound or failed to. */
    private static void withEngine(Context context, Runnable then) {
        if (ready || failed) {
            then.run();
            return;
        }
        waiting.add(then);
        if (tts != null) return;
        tts = new TextToSpeech(context.getApplicationContext(), status -> main.post(() -> {
            ready = status == TextToSpeech.SUCCESS;
            failed = !ready;
            if (ready) {
                tts.setAudioAttributes(new AudioAttributes.Builder().setUsage(AudioAttributes.USAGE_MEDIA)
                        .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH).build());
                tts.setOnUtteranceProgressListener(listener);
            }
            ArrayList<Runnable> run = new ArrayList<>(waiting);
            waiting.clear();
            for (Runnable r : run) r.run();
        }));
    }

    private static void start(NsgoView view, long count, String text, String lang, String id, float rate, float pitch) {
        stop();
        if (!id.isEmpty()) {
            Voice found = null;
            if (tts.getVoices() != null) {
                for (Voice v : tts.getVoices()) {
                    if (v.getName().equals(id)) found = v;
                }
            }
            if (found == null || tts.setVoice(found) != TextToSpeech.SUCCESS) {
                view.answerSpeak(count, false, "voice-unavailable", "");
                return;
            }
        } else {
            int r = tts.setLanguage(Locale.forLanguageTag(lang));
            if (r == TextToSpeech.LANG_MISSING_DATA || r == TextToSpeech.LANG_NOT_SUPPORTED) {
                view.answerSpeak(count, false, "language-unavailable", "");
                return;
            }
        }
        if (text.length() > TextToSpeech.getMaxSpeechInputLength()) {
            view.answerSpeak(count, false, "text-too-long", "");
            return;
        }
        tts.setSpeechRate(rate);
        tts.setPitch(pitch);
        String uid = "nsgo-" + (++sequence);
        owner = view;
        current = count;
        utterance = uid;
        if (tts.speak(text, TextToSpeech.QUEUE_FLUSH, new Bundle(), uid) != TextToSpeech.SUCCESS) {
            end(uid, false, "synthesis-failed");
        }
    }

    /** Stops the utterance speaking, which reports itself stopped. */
    private static void stop() {
        if (utterance == null) return;
        String uid = utterance;
        end(uid, true, "");
        tts.stop();
    }

    /** Ends utterance uid, if it is the one speaking, reporting how. */
    private static void end(String uid, boolean stopped, String error) {
        if (!uid.equals(utterance)) return;
        NsgoView view = owner;
        long count = current;
        owner = null;
        utterance = null;
        view.answerSpeak(count, stopped, error, "");
    }

    private static final UtteranceProgressListener listener = new UtteranceProgressListener() {
        @Override public void onStart(String uid) {}

        @Override public void onDone(String uid) {
            main.post(() -> end(uid, false, ""));
        }

        @Override public void onStop(String uid, boolean interrupted) {
            main.post(() -> end(uid, true, ""));
        }

        @Override @Deprecated public void onError(String uid) {
            main.post(() -> end(uid, false, "synthesis-failed"));
        }

        @Override public void onError(String uid, int code) {
            String error;
            switch (code) {
                case TextToSpeech.ERROR_NOT_INSTALLED_YET:
                    error = "language-unavailable";
                    break;
                case TextToSpeech.ERROR_OUTPUT:
                    error = "audio-busy";
                    break;
                default:
                    error = "synthesis-failed";
            }
            main.post(() -> end(uid, false, error));
        }
    };

    /** The voices installed, as JSON driver.Voice reads: their names as IDs and names, and their locales' tags. */
    private static String voices() {
        JSONArray out = new JSONArray();
        if (tts.getVoices() == null) return out.toString();
        try {
            for (Voice v : tts.getVoices()) {
                if (v.getFeatures() != null && v.getFeatures().contains(TextToSpeech.Engine.KEY_FEATURE_NOT_INSTALLED)) continue;
                out.put(new JSONObject().put("id", v.getName()).put("name", v.getName()).put("lang", v.getLocale().toLanguageTag()));
            }
        } catch (JSONException impossible) {
            throw new AssertionError(impossible);
        }
        return out.toString();
    }
}

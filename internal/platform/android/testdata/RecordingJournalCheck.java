package dev.oneo.nsgo;

import java.io.File;
import java.io.IOException;
import java.nio.charset.StandardCharsets;

/**
 * Runs one step of RecordingJournal on the JVM for recordingjournal_test.go, each in a process of its own as RecordingService's processes are: put keeps a record, take lets one go, and pending prints the records a process that connects hears, one a line.
 */
final class RecordingJournalCheck {
    public static void main(String[] args) throws IOException {
        File dir = new File(args[0]);
        switch (args[1]) {
            case "put":
                RecordingJournal.put(dir, args[2], args[3].getBytes(StandardCharsets.UTF_8));
                break;
            case "take":
                RecordingJournal.remove(dir, args[2]);
                break;
            case "pending":
                for (byte[] record : RecordingJournal.pending(dir)) {
                    System.out.println(new String(record, StandardCharsets.UTF_8));
                }
                break;
            default:
                throw new IllegalArgumentException(args[1]);
        }
    }
}

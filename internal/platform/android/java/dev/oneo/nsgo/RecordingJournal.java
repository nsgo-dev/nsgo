package dev.oneo.nsgo;

import java.io.BufferedInputStream;
import java.io.BufferedOutputStream;
import java.io.DataInputStream;
import java.io.DataOutputStream;
import java.io.EOFException;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileNotFoundException;
import java.io.FileOutputStream;
import java.io.IOException;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * The terminal records of earlier recordings that package microphone's Earlier has not taken yet, kept in a file of the recordings directory so that a record outlives the processes it was reported to: RecordingService adds the record of each file it repairs and of each recording whose Go process ended, every Go process that connects hears those of its directory, and a record goes once the app takes it. A record is keyed by its file's path, so a newer record of the same file replaces the older. It uses java.* alone, so RecordingJournalCheck runs it on the JVM; its callers hold RecordingService.class.
 */
final class RecordingJournal {
    private RecordingJournal() {}

    /** The journal's name in the recordings directory: a dot name, which no recording has and the repair of temporary files skips. */
    static final String NAME = ".earlier";
    private static final int VERSION = 1;

    /** Keeps record, the record of the file at path, in dir's journal, replacing an older one of the same file. */
    static void put(File dir, String path, byte[] record) throws IOException {
        Map<String, byte[]> records = read(dir);
        records.remove(path);
        records.put(path, record);
        write(dir, records);
    }

    /** Lets go of the record of the file at path, which the app took; nothing if dir's journal has none. */
    static void remove(File dir, String path) throws IOException {
        Map<String, byte[]> records = read(dir);
        if (records.remove(path) != null) write(dir, records);
    }

    /** The records dir's journal keeps, oldest first, without those whose file is gone, which the app deleted. */
    static List<byte[]> pending(File dir) throws IOException {
        Map<String, byte[]> records = read(dir);
        boolean dropped = records.keySet().removeIf(path -> !new File(path).exists());
        if (dropped) write(dir, records);
        return new ArrayList<>(records.values());
    }

    private static Map<String, byte[]> read(File dir) throws IOException {
        Map<String, byte[]> records = new LinkedHashMap<>();
        DataInputStream in;
        try {
            in = new DataInputStream(new BufferedInputStream(new FileInputStream(new File(dir, NAME))));
        } catch (FileNotFoundException e) {
            return records;
        }
        try (DataInputStream r = in) {
            if (r.readInt() != VERSION) return records;
            for (int n = r.readInt(); n > 0; n--) {
                String path = r.readUTF();
                byte[] record = new byte[r.readInt()];
                r.readFully(record);
                records.put(path, record);
            }
        } catch (EOFException e) {
            // A journal cut short keeps the records before the cut; write replaces it whole, so only a damaged disk cuts one.
        }
        return records;
    }

    /** Replaces dir's journal with records, through a temporary file, so that a kill leaves the old journal or the new one. */
    private static void write(File dir, Map<String, byte[]> records) throws IOException {
        File journal = new File(dir, NAME), next = new File(dir, NAME + ".next");
        if (records.isEmpty()) {
            if (journal.exists() && !journal.delete()) throw new IOException("cannot delete " + journal);
            return;
        }
        try (FileOutputStream f = new FileOutputStream(next);
                DataOutputStream out = new DataOutputStream(new BufferedOutputStream(f))) {
            out.writeInt(VERSION);
            out.writeInt(records.size());
            for (Map.Entry<String, byte[]> e : records.entrySet()) {
                out.writeUTF(e.getKey());
                out.writeInt(e.getValue().length);
                out.write(e.getValue());
            }
            out.flush();
            f.getFD().sync();
        }
        if (!next.renameTo(journal)) throw new IOException("cannot replace " + journal);
    }
}

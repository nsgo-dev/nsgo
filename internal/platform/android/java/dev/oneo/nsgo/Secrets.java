package dev.oneo.nsgo;

import android.content.Context;
import android.security.keystore.KeyGenParameterSpec;
import android.security.keystore.KeyProperties;
import android.util.AtomicFile;

import java.io.ByteArrayOutputStream;
import java.io.DataInputStream;
import java.io.DataOutputStream;
import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;
import java.security.GeneralSecurityException;
import java.security.KeyStore;
import java.security.MessageDigest;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

import javax.crypto.Cipher;
import javax.crypto.KeyGenerator;
import javax.crypto.SecretKey;
import javax.crypto.spec.GCMParameterSpec;

/**
 * The app's secrets (secrets.go): each value encrypted with AES-GCM under one key of the app's in AndroidKeyStore, which never leaves the system's keystore, its service and account as the associated data, and kept in a file of the app's no-backup files named after their SHA-256, which neither other apps nor backups reach. One thread serves the requests in order, off the reader thread.
 */
final class Secrets {
    private Secrets() {}

    /** The key's alias in AndroidKeyStore. */
    private static final String KEY = "dev.oneo.nsgo.secrets";
    /** The directory of the ciphertexts in the app's no-backup files. */
    private static final String DIR = "nsgo-secrets";
    /** The first byte of a file: its format, then the IV's length, the IV and the ciphertext with its tag. */
    private static final int FORMAT = 1;
    private static final int TAG_BITS = 128;

    private static final ExecutorService worker = Executors.newSingleThreadExecutor();

    /** Sends an answer to Go. */
    interface Reply {
        void send(byte[] payload);
    }

    /** Answers a SECRET message on the worker: its number, an error, whether a value was found and the value. */
    static void answer(Context context, DataInputStream in, Reply reply) {
        worker.execute(() -> reply.send(handle(context, in)));
    }

    private static byte[] handle(Context context, DataInputStream in) {
        ByteArrayOutputStream bytes = new ByteArrayOutputStream();
        DataOutputStream out = new DataOutputStream(bytes);
        try {
            int id = in.readInt();
            int op = in.readUnsignedByte();
            byte[] service = read(in), account = read(in), value = read(in);
            String error = "";
            boolean found = false;
            byte[] result = new byte[0];
            try {
                File file = file(context, service, account);
                byte[] aad = associated(service, account);
                switch (op) {
                    case Protocol.SECRET_READ:
                        if (file.exists()) {
                            result = decrypt(new AtomicFile(file).readFully(), aad);
                            found = true;
                        }
                        break;
                    case Protocol.SECRET_WRITE:
                        write(file, encrypt(value, aad));
                        break;
                    case Protocol.SECRET_DELETE:
                        new AtomicFile(file).delete();
                        break;
                    default:
                        error = "no secret request " + op;
                }
            } catch (IOException | GeneralSecurityException | RuntimeException e) {
                error = e.getClass().getSimpleName() + ": " + e.getMessage();
                found = false;
                result = new byte[0];
            }
            out.writeInt(id);
            Protocol.writeString(out, error);
            out.writeByte(found ? 1 : 0);
            out.writeInt(result.length);
            out.write(result);
        } catch (IOException impossible) {
            throw new AssertionError(impossible);
        }
        return bytes.toByteArray();
    }

    private static byte[] read(DataInputStream in) throws IOException {
        byte[] b = new byte[in.readInt()];
        in.readFully(b);
        return b;
    }

    /** The service's length in four bytes, the service and the account, so no other service and account make the same bytes. */
    private static byte[] associated(byte[] service, byte[] account) {
        byte[] b = new byte[4 + service.length + account.length];
        int n = service.length;
        b[0] = (byte) (n >>> 24);
        b[1] = (byte) (n >>> 16);
        b[2] = (byte) (n >>> 8);
        b[3] = (byte) n;
        System.arraycopy(service, 0, b, 4, n);
        System.arraycopy(account, 0, b, 4 + n, account.length);
        return b;
    }

    private static File file(Context context, byte[] service, byte[] account) throws GeneralSecurityException {
        byte[] digest = MessageDigest.getInstance("SHA-256").digest(associated(service, account));
        StringBuilder name = new StringBuilder(digest.length * 2);
        for (byte b : digest) name.append(String.format("%02x", b));
        return new File(new File(context.getNoBackupFilesDir(), DIR), name.toString());
    }

    private static void write(File file, byte[] data) throws IOException {
        File dir = file.getParentFile();
        if (!dir.isDirectory() && !dir.mkdirs()) throw new IOException("cannot make " + dir);
        AtomicFile atomic = new AtomicFile(file);
        FileOutputStream stream = atomic.startWrite();
        try {
            stream.write(data);
            atomic.finishWrite(stream);
        } catch (IOException e) {
            atomic.failWrite(stream);
            throw e;
        }
    }

    /** The app's key, made the first time. */
    private static SecretKey key() throws GeneralSecurityException, IOException {
        KeyStore store = KeyStore.getInstance("AndroidKeyStore");
        store.load(null);
        if (store.getKey(KEY, null) instanceof SecretKey key) return key;
        KeyGenerator generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore");
        generator.init(new KeyGenParameterSpec.Builder(KEY, KeyProperties.PURPOSE_ENCRYPT | KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .build());
        return generator.generateKey();
    }

    private static byte[] encrypt(byte[] value, byte[] aad) throws GeneralSecurityException, IOException {
        Cipher cipher = Cipher.getInstance("AES/GCM/NoPadding");
        // The keystore picks the IV, as it refuses one the caller gives.
        cipher.init(Cipher.ENCRYPT_MODE, key());
        cipher.updateAAD(aad);
        byte[] sealed = cipher.doFinal(value);
        byte[] iv = cipher.getIV();
        ByteArrayOutputStream out = new ByteArrayOutputStream(2 + iv.length + sealed.length);
        out.write(FORMAT);
        out.write(iv.length);
        out.write(iv);
        out.write(sealed);
        return out.toByteArray();
    }

    private static byte[] decrypt(byte[] data, byte[] aad) throws GeneralSecurityException, IOException {
        if (data.length < 2 || data[0] != FORMAT || data.length < 2 + (data[1] & 0xff)) {
            throw new IOException("the secret's file is not one this runtime wrote");
        }
        int ivLength = data[1] & 0xff;
        Cipher cipher = Cipher.getInstance("AES/GCM/NoPadding");
        cipher.init(Cipher.DECRYPT_MODE, key(), new GCMParameterSpec(TAG_BITS, data, 2, ivLength));
        cipher.updateAAD(aad);
        return cipher.doFinal(data, 2 + ivLength, data.length - 2 - ivLength);
    }
}

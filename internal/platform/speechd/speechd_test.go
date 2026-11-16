package speechd

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// daemon is a fake speech-dispatcher: it answers SSIP's commands as the daemon does, records them, and ends a message when the test says.
type daemon struct {
	mu       sync.Mutex
	commands []string
	texts    []string
	w        *bufio.Writer
	msg      int
}

func (d *daemon) send(lines ...string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, l := range lines {
		d.w.WriteString(l + "\r\n")
	}
	d.w.Flush()
}

func (d *daemon) serve(t *testing.T, nc net.Conn) {
	r := bufio.NewReader(nc)
	d.mu.Lock()
	d.w = bufio.NewWriter(nc)
	d.mu.Unlock()
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.TrimRight(line, "\r\n")
		d.mu.Lock()
		d.commands = append(d.commands, cmd)
		d.mu.Unlock()
		switch {
		case cmd == "SPEAK":
			d.send("230 OK RECEIVING DATA")
			var text []string
			for {
				l, _ := r.ReadString('\n')
				l = strings.TrimRight(l, "\r\n")
				if l == "." {
					break
				}
				text = append(text, l)
			}
			d.mu.Lock()
			d.msg++
			msg := d.msg
			d.texts = append(d.texts, strings.Join(text, "\n"))
			d.mu.Unlock()
			d.send(fmt.Sprintf("225-%d", msg), "225 OK MESSAGE QUEUED")
		case cmd == "CANCEL self":
			d.mu.Lock()
			msg := d.msg
			d.mu.Unlock()
			d.send("210 OK CANCELED", fmt.Sprintf("703-%d", msg), "703-1", "703 CANCELED")
		case cmd == "LIST SYNTHESIS_VOICES":
			d.send("249-Alan\ten-GB\tnone", "249-kal\ten_US\tnone", "249 OK VOICE LIST SENT")
		case cmd == "SET self SYNTHESIS_VOICE nobody":
			d.send("409 ERR VOICE NOT FOUND")
		default:
			d.send("208 OK SET")
		}
	}
}

func TestSpeechDispatcher(t *testing.T) {
	// A socket's path is short: macOS's temporary directory is too long for one.
	dir, err := os.MkdirTemp("/tmp", "speechd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "s")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Skip("no unix sockets here:", err)
	}
	defer l.Close()
	t.Setenv("SPEECHD_ADDRESS", "unix_socket:"+sock)
	d := &daemon{}
	go func() {
		nc, err := l.Accept()
		if err == nil {
			d.serve(t, nc)
		}
	}()
	if !Available() {
		t.Fatal("a listening daemon is not available")
	}
	events := make(chan driver.Event, 8)
	dispatch := func(ev driver.Event) { events <- ev }
	next := func() (uint32, driver.SpeakEvent) {
		t.Helper()
		select {
		case ev := <-events:
			return ev.Count, *ev.Data.(*driver.SpeakEvent)
		case <-time.After(5 * time.Second):
			t.Fatal("no report")
		}
		return 0, driver.SpeakEvent{}
	}

	Handle(1, driver.SpeakRequest{Command: driver.SpeakVoices}, dispatch)
	if id, e := next(); id != 1 || !slices.Equal(e.Voices, []driver.Voice{{ID: "Alan", Name: "Alan", Lang: "en-GB"}, {ID: "kal", Name: "kal", Lang: "en-US"}}) {
		t.Fatalf("voices %d %+v", id, e)
	}

	// An utterance ends as the daemon says; one line of a dot is doubled.
	Handle(2, driver.SpeakRequest{Command: driver.SpeakStart, Text: "Hello\n.\nworld", Lang: "en-US", Rate: 2, Pitch: 0.5}, dispatch)
	waitFor(t, d, "SPEAK")
	d.send("701-1", "701-1", "701 BEGIN", "702-1", "702-1", "702 END")
	if id, e := next(); id != 2 || e.Stopped || e.Error != "" {
		t.Fatalf("the first ended %d %+v", id, e)
	}
	d.mu.Lock()
	text, cmds := d.texts[0], slices.Clone(d.commands)
	d.mu.Unlock()
	if text != "Hello\n..\nworld" {
		t.Errorf("text %q", text)
	}
	for _, want := range []string{"SET self NOTIFICATION END on", "SET self LANGUAGE en-us", "SET self RATE 100", "SET self PITCH -100"} {
		if !slices.Contains(cmds, want) {
			t.Errorf("no %q among %q", want, cmds)
		}
	}

	// A second utterance stops the first, reported once, and a stop the second.
	Handle(3, driver.SpeakRequest{Command: driver.SpeakStart, Text: "one", Lang: "ja"}, dispatch)
	waitFor(t, d, "SPEAK")
	Handle(4, driver.SpeakRequest{Command: driver.SpeakStart, Text: "two", Lang: "ja"}, dispatch)
	if id, e := next(); id != 3 || !e.Stopped {
		t.Fatalf("the second ended %d %+v", id, e)
	}
	Handle(4, driver.SpeakRequest{Command: driver.SpeakStop}, dispatch)
	if id, e := next(); id != 4 || !e.Stopped {
		t.Fatalf("the third ended %d %+v", id, e)
	}

	Handle(5, driver.SpeakRequest{Command: driver.SpeakStart, Text: "x", Voice: "nobody"}, dispatch)
	if id, e := next(); id != 5 || e.Error != driver.SpeakVoiceUnavailable {
		t.Fatalf("an unknown voice ended %d %+v", id, e)
	}
	select {
	case ev := <-events:
		t.Fatalf("a late report %+v", ev)
	case <-time.After(300 * time.Millisecond):
	}
}

// waitFor waits for the daemon to hear cmd, which it marks heard.
func waitFor(t *testing.T, d *daemon, cmd string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		d.mu.Lock()
		i := slices.Index(d.commands, cmd)
		if i >= 0 {
			d.commands[i] = "(" + cmd + ")"
		}
		d.mu.Unlock()
		if i >= 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("the daemon heard no %s", cmd)
}

func TestAddress(t *testing.T) {
	t.Setenv("SPEECHD_ADDRESS", "inet_socket:localhost:7000")
	if n, a := address(); n != "tcp" || a != "localhost:7000" {
		t.Errorf("inet_socket: %s %s", n, a)
	}
	t.Setenv("SPEECHD_ADDRESS", "")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	if n, a := address(); n != "unix" || a != filepath.Join("/run/user/1000", "speech-dispatcher", "speechd.sock") {
		t.Errorf("runtime directory: %s %s", n, a)
	}
}

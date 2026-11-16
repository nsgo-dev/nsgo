// Package speechd speaks text through speech-dispatcher, Linux's speech service, for core's Speak: a client of its SSIP protocol over the socket the daemon listens on, written in Go as the protocol is lines of text, so the backend loads no libspeechd. One connection serves the process, as the daemon speaks one message of a client at a time, and a worker goroutine sends the requests in order, so the main thread never waits for the daemon.
package speechd

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// address is where the daemon listens: SPEECHD_ADDRESS's unix_socket:path or inet_socket:host:port, else the socket in the user's runtime directory, as libspeechd finds it.
func address() (network, addr string) {
	if a := os.Getenv("SPEECHD_ADDRESS"); a != "" {
		method, rest, _ := strings.Cut(a, ":")
		switch method {
		case "unix_socket":
			if rest != "" {
				return "unix", rest
			}
		case "inet_socket":
			host, port, ok := strings.Cut(rest, ":")
			if !ok {
				port = "6560"
			}
			return "tcp", net.JoinHostPort(cmpOr(host, "127.0.0.1"), port)
		}
	}
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return "unix", filepath.Join(dir, "speech-dispatcher", "speechd.sock")
	}
	home, _ := os.UserHomeDir()
	return "unix", filepath.Join(home, ".cache", "speech-dispatcher", "speechd.sock")
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// Available is Environment.Speak: the daemon listens, or can be started, as libspeechd starts it.
func Available() bool {
	network, addr := address()
	if network == "unix" {
		if _, err := os.Stat(addr); err == nil {
			return true
		}
	}
	_, err := exec.LookPath("speech-dispatcher")
	return err == nil || network == "tcp"
}

// client is the process's connection, made at the first request.
var client = &conn{jobs: make(chan func(), 64)}

// Handle carries out an ItemSpeak of utterance or request id, reporting EventSpeak through dispatch, on any goroutine.
func Handle(id uint32, r driver.SpeakRequest, dispatch func(driver.Event)) {
	client.once.Do(func() { go client.work() })
	client.jobs <- func() { client.handle(id, r, dispatch) }
}

// conn is a connection to the daemon and the utterances it speaks.
type conn struct {
	once sync.Once
	jobs chan func()
	// The worker's: the connection, and what it reads.
	c       net.Conn
	w       *bufio.Writer
	replies chan reply
	// mu guards what the reader reports to: the utterance speaking, by the daemon's message ID, and the ends of messages it heard before their IDs came back.
	mu       sync.Mutex
	speaking *utterance
	early    map[int]string
}

// utterance is a message the daemon speaks for utterance id.
type utterance struct {
	id       uint32
	msg      int
	dispatch func(driver.Event)
}

// reply is what the daemon answered a command: its code and its lines.
type reply struct {
	code  int
	lines []string
}

func (c *conn) work() {
	for job := range c.jobs {
		job()
	}
}

func (c *conn) handle(id uint32, r driver.SpeakRequest, dispatch func(driver.Event)) {
	report := func(e driver.SpeakEvent) { dispatch(driver.Event{Kind: driver.EventSpeak, Count: id, Data: &e}) }
	if r.Command == driver.SpeakStop {
		c.mu.Lock()
		u := c.speaking
		c.mu.Unlock()
		if u != nil && u.id == id {
			c.stop()
		}
		return
	}
	if err := c.connect(); err != nil {
		log.Printf("nsgo: speak: %v", err)
		report(driver.SpeakEvent{Error: driver.SpeakUnavailable})
		return
	}
	switch r.Command {
	case driver.SpeakStart:
		c.stop()
		code := c.speak(id, r, dispatch)
		if code != "" {
			report(driver.SpeakEvent{Error: code})
		}
	case driver.SpeakVoices:
		voices, err := c.voices()
		if err != nil {
			log.Printf("nsgo: speak: %v", err)
		}
		report(driver.SpeakEvent{Voices: voices})
	}
}

// connect connects to the daemon, starting it where it does not listen, once, and again after the daemon went.
func (c *conn) connect() error {
	if c.c != nil {
		return nil
	}
	network, addr := address()
	nc, err := net.Dial(network, addr)
	if err != nil && network == "unix" {
		// As libspeechd: the daemon starts itself in the background and exits once idle.
		if path, lerr := exec.LookPath("speech-dispatcher"); lerr == nil {
			if out, serr := exec.Command(path, "--spawn", "--communication-method", "unix_socket", "--socket-path", addr).CombinedOutput(); serr != nil {
				log.Printf("nsgo: speak: speech-dispatcher --spawn: %v: %s", serr, out)
			}
			for range 20 {
				if nc, err = net.Dial(network, addr); err == nil {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
	if err != nil {
		return fmt.Errorf("cannot reach speech-dispatcher: %w", err)
	}
	c.c, c.w, c.replies = nc, bufio.NewWriter(nc), make(chan reply, 1)
	go c.read(nc, c.replies)
	for _, cmd := range []string{"SET self CLIENT_NAME " + clientName(), "SET self NOTIFICATION END on", "SET self NOTIFICATION CANCEL on"} {
		if r, err := c.command(cmd); err != nil || r.code/100 != 2 {
			c.close()
			return fmt.Errorf("speech-dispatcher refused %q: %v %v", cmd, r.lines, err)
		}
	}
	return nil
}

// clientName names the app to the daemon, as user:application:component.
func clientName() string {
	user := os.Getenv("USER")
	app := filepath.Base(os.Args[0])
	clean := func(s string) string {
		return strings.Map(func(r rune) rune {
			if r <= ' ' || r == ':' {
				return '_'
			}
			return r
		}, cmpOr(s, "nsgo"))
	}
	return clean(user) + ":" + clean(app) + ":main"
}

// close drops the connection, which the next request makes again.
func (c *conn) close() {
	if c.c != nil {
		c.c.Close()
	}
	c.c, c.w = nil, nil
}

// command sends one command and waits for its reply.
func (c *conn) command(line string, data ...string) (reply, error) {
	if c.c == nil {
		return reply{}, errors.New("not connected")
	}
	c.w.WriteString(line + "\r\n")
	if err := c.w.Flush(); err != nil {
		c.close()
		return reply{}, err
	}
	select {
	case r, ok := <-c.replies:
		if !ok {
			c.close()
			return reply{}, errors.New("speech-dispatcher closed the connection")
		}
		return r, nil
	case <-time.After(5 * time.Second):
		c.close()
		return reply{}, errors.New("speech-dispatcher did not answer")
	}
}

// speak has the daemon speak r, reporting its end once the daemon does, or answers why it cannot, a driver Speak error.
func (c *conn) speak(id uint32, r driver.SpeakRequest, dispatch func(driver.Event)) string {
	settings := []string{
		// The daemon reads a language as its primary subtag and, from 0.11, a region.
		"SET self LANGUAGE " + strings.ToLower(driver.LanguageTag(r.Lang)),
		"SET self RATE " + strconv.Itoa(int(driver.SpeakScale(r.Rate, -100, 0, 100))),
		"SET self PITCH " + strconv.Itoa(int(driver.SpeakScale(r.Pitch, -100, 0, 100))),
	}
	if r.Voice != "" {
		settings = append(settings, "SET self SYNTHESIS_VOICE "+r.Voice)
	}
	for _, cmd := range settings {
		rep, err := c.command(cmd)
		switch {
		case err != nil:
			log.Printf("nsgo: speak: %v", err)
			return driver.SpeakFailed
		case rep.code/100 == 2:
		case strings.HasPrefix(cmd, "SET self SYNTHESIS_VOICE"):
			return driver.SpeakVoiceUnavailable
		case strings.HasPrefix(cmd, "SET self LANGUAGE"):
			return driver.SpeakLanguageUnavailable
		}
	}
	rep, err := c.command("SPEAK")
	if err != nil || rep.code != 230 {
		return driver.SpeakFailed
	}
	// The text's lines, a line of one dot doubled, then a line of one dot.
	var b strings.Builder
	for _, line := range strings.Split(strings.ReplaceAll(r.Text, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, ".") {
			line = "." + line
		}
		b.WriteString(line + "\r\n")
	}
	b.WriteString(".")
	rep, err = c.command(b.String())
	if err != nil || rep.code != 225 || len(rep.lines) < 2 {
		return driver.SpeakFailed
	}
	msg, err := strconv.Atoi(rep.lines[0])
	if err != nil {
		return driver.SpeakFailed
	}
	u := &utterance{id: id, msg: msg, dispatch: dispatch}
	c.mu.Lock()
	c.speaking = u
	ended, early := c.early[msg]
	// What came early of the messages before is of no one now.
	c.early = nil
	c.mu.Unlock()
	if early {
		c.ended(msg, ended)
	}
	return ""
}

// stop cancels the utterance speaking, which reports itself stopped.
func (c *conn) stop() {
	c.mu.Lock()
	u := c.speaking
	c.speaking = nil
	c.mu.Unlock()
	if u == nil {
		return
	}
	u.dispatch(driver.Event{Kind: driver.EventSpeak, Count: u.id, Data: &driver.SpeakEvent{Stopped: true}})
	if _, err := c.command("CANCEL self"); err != nil {
		log.Printf("nsgo: speak: %v", err)
	}
}

// voices lists the output module's voices: a name, a language and a variant to a line.
func (c *conn) voices() ([]driver.Voice, error) {
	rep, err := c.command("LIST SYNTHESIS_VOICES")
	if err != nil || rep.code/100 != 2 {
		return nil, fmt.Errorf("speech-dispatcher lists no voices: %v %v", rep.lines, err)
	}
	var voices []driver.Voice
	for _, line := range rep.lines[:len(rep.lines)-1] {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			fields = strings.Fields(line)
		}
		if len(fields) < 2 || fields[0] == "" {
			continue
		}
		voices = append(voices, driver.Voice{ID: fields[0], Name: fields[0], Lang: driver.LanguageTag(fields[1])})
	}
	return voices, nil
}

// read reads the daemon's lines: a reply's to replies, and an event's, numbered 7xx, to ended, as the daemon sends events between replies, never within one.
func (c *conn) read(nc net.Conn, replies chan<- reply) {
	defer close(replies)
	r := bufio.NewReader(nc)
	var lines []string
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			c.mu.Lock()
			u := c.speaking
			c.speaking = nil
			c.mu.Unlock()
			if u != nil {
				u.dispatch(driver.Event{Kind: driver.EventSpeak, Count: u.id, Data: &driver.SpeakEvent{Error: driver.SpeakFailed}})
			}
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if len(line) < 4 {
			continue
		}
		code, err := strconv.Atoi(line[:3])
		if err != nil {
			continue
		}
		lines = append(lines, line[4:])
		if line[3] == '-' {
			continue
		}
		block := lines
		lines = nil
		if code/100 == 7 {
			// The message's ID, the client's, then the event's name.
			if msg, err := strconv.Atoi(block[0]); err == nil && len(block) == 3 {
				switch code {
				case 702: // END
					c.ended(msg, "end")
				case 703: // CANCELED
					c.ended(msg, "cancel")
				}
			}
			continue
		}
		replies <- reply{code: code, lines: block}
	}
}

// ended reports that message msg ended, as how says: "end" or "cancel".
func (c *conn) ended(msg int, how string) {
	c.mu.Lock()
	u := c.speaking
	if u == nil || u.msg != msg {
		if u == nil {
			// Its ID may not have come back yet.
			if c.early == nil {
				c.early = map[int]string{}
			}
			c.early[msg] = how
		}
		c.mu.Unlock()
		return
	}
	c.speaking = nil
	c.mu.Unlock()
	u.dispatch(driver.Event{Kind: driver.EventSpeak, Count: u.id, Data: &driver.SpeakEvent{Stopped: how == "cancel"}})
}

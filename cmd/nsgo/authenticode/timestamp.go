package authenticode

import (
	"bytes"
	"encoding/asn1"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Timestamp is a timestamp server, RFC 3161's: it signs the digest of a
// signature's value with the current time, so that Windows trusts the
// signature past the certificate's expiration.
type Timestamp struct {
	// URL is the server's address, such as http://timestamp.digicert.com.
	URL string
	// Client makes the request; http.DefaultClient without one.
	Client *http.Client
}

// Token asks the server to sign digest and returns its token, the
// TimeStampResp's TimeStampToken.
func (t *Timestamp) Token(digest []byte) ([]byte, error) {
	req := timeStampRequest(digest, time.Now())
	body, err := t.roundTrip(req)
	if err != nil {
		return nil, err
	}
	return timeStampToken(body)
}

func (t *Timestamp) roundTrip(req []byte) ([]byte, error) {
	client := t.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Post(t.URL, "application/timestamp-query", bytes.NewReader(req))
	if err != nil {
		return nil, fmt.Errorf("timestamp %s: %w", t.URL, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("timestamp %s: %w", t.URL, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("timestamp %s: %s", t.URL, resp.Status)
	}
	return body, nil
}

// timeStampRequest is RFC 3161's TimeStampReq: version 1, the message
// imprint, a nonce, and the server's certificate asked for.
func timeStampRequest(digest []byte, now time.Time) []byte {
	imprint := sequence(algorithmIdentifier(oidSHA256), octetString(digest))
	nonce := smallInt(int(now.UnixNano() & 0x7fffffff))
	return sequence(smallInt(1), imprint, nonce, boolean(true))
}

// timeStampToken is the TimeStampToken of a TimeStampResp, which holds the
// PKCS #7 ContentInfo alone before its status.
func timeStampToken(resp []byte) ([]byte, error) {
	var answer struct {
		Status struct {
			Status  int
			Failure asn1.RawValue `asn1:"optional"`
			Text    string        `asn1:"optional"`
		}
		Token asn1.RawValue `asn1:"optional"`
	}
	if _, err := asn1.Unmarshal(resp, &answer); err != nil {
		return nil, fmt.Errorf("timestamp: the response is not a TimeStampResp: %w", err)
	}
	// granted(0) and grantedWithMods(1) carry a token; the others say why
	// not, 2 being a rejection and 3 to 5 the server's failures.
	if answer.Status.Status > 1 {
		text := answer.Status.Text
		if text == "" {
			text = fmt.Sprintf("status %d", answer.Status.Status)
		}
		return nil, errors.New("timestamp: " + text)
	}
	if len(answer.Token.FullBytes) == 0 {
		return nil, errors.New("timestamp: the response holds no token")
	}
	return answer.Token.FullBytes, nil
}

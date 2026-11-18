package cookies

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testSigner(t *testing.T) *Signer {
	t.Helper()

	signer, err := NewSigner("session", []string{strings.Repeat("k", 32)})
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	return signer
}

func signPayload(t *testing.T, signer *Signer, payload signedPayload) string {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	body := signedVersion + "." + encode(raw)
	return body + "." + encode(sign(signer.keys[0], body))
}

func signBody(signer *Signer, body string) string {
	return body + "." + encode(sign(signer.keys[0], body))
}

func requestWith(t *testing.T, value string) *http.Request {
	t.Helper()

	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Cookie", "session="+value)
	return request
}

func TestSignerRejectsExpiredPayload(t *testing.T) {
	signer := testSigner(t)
	now := time.Now()
	value := signPayload(t, signer, signedPayload{
		Data:    map[string]any{"user": "42"},
		Issued:  now.Add(-2 * time.Hour).Unix(),
		Expires: now.Add(-time.Hour).Unix(),
	})

	if _, ok := signer.Get(requestWith(t, value)); ok {
		t.Error("Get() accepted an expired payload")
	}
}

func TestSignerRejectsWrongVersion(t *testing.T) {
	signer := testSigner(t)
	raw, err := json.Marshal(signedPayload{Data: map[string]any{"user": "42"}, Issued: time.Now().Unix(), Expires: time.Now().Add(time.Hour).Unix()})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	value := signBody(signer, "v2."+encode(raw))

	if _, ok := signer.Get(requestWith(t, value)); ok {
		t.Error("Get() accepted an unknown version")
	}
}

func TestSignerRejectsNonJSONPayload(t *testing.T) {
	signer := testSigner(t)
	value := signBody(signer, signedVersion+"."+encode([]byte("not json")))

	if _, ok := signer.Get(requestWith(t, value)); ok {
		t.Error("Get() accepted a non-JSON payload")
	}
}

func TestSignerRejectsNullData(t *testing.T) {
	signer := testSigner(t)
	value := signPayload(t, signer, signedPayload{
		Data:    nil,
		Issued:  time.Now().Unix(),
		Expires: time.Now().Add(time.Hour).Unix(),
	})

	if _, ok := signer.Get(requestWith(t, value)); ok {
		t.Error("Get() accepted a payload without data")
	}
}

func TestSignerRejectsNonASCIIValue(t *testing.T) {
	signer := testSigner(t)
	value := signPayload(t, signer, signedPayload{
		Data:    map[string]any{"user": "42"},
		Issued:  time.Now().Unix(),
		Expires: time.Now().Add(time.Hour).Unix(),
	})

	if _, ok := signer.Get(requestWith(t, value+"é")); ok {
		t.Error("Get() accepted a non-ASCII value")
	}
}

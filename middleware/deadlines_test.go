package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A body sent slower than the default read deadline fails on an ordinary
// route and succeeds on a LongTransfer route.
func TestDeadlines(t *testing.T) {
	prevR, prevW, prevT := DefaultReadDeadline, DefaultWriteDeadline, TransferDeadline
	DefaultReadDeadline, DefaultWriteDeadline, TransferDeadline = 300*time.Millisecond, 300*time.Millisecond, 10*time.Second
	t.Cleanup(func() { DefaultReadDeadline, DefaultWriteDeadline, TransferDeadline = prevR, prevW, prevT })

	read := func(w http.ResponseWriter, r *http.Request) {
		n, err := io.Copy(io.Discard, r.Body)
		if err != nil {
			w.WriteHeader(http.StatusRequestTimeout)
			return
		}
		w.Write([]byte(strings.Repeat("x", int(n))))
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/short", read)
	mux.HandleFunc("/long", LongTransfer(read))
	srv := httptest.NewServer(Deadlines(mux))
	defer srv.Close()

	slowBody := func() io.Reader {
		pr, pw := io.Pipe()
		go func() {
			for i := 0; i < 8; i++ { // ~800 ms in total
				pw.Write([]byte("ab"))
				time.Sleep(100 * time.Millisecond)
			}
			pw.Close()
		}()
		return pr
	}
	send := func(path string) (int, error) {
		resp, err := http.Post(srv.URL+path, "text/plain", slowBody())
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)
		return resp.StatusCode, nil
	}

	if code, err := send("/short"); err == nil && code == http.StatusOK {
		t.Fatal("slow body on a default route should hit the read deadline")
	}
	if code, err := send("/long"); err != nil || code != http.StatusOK {
		t.Fatalf("LongTransfer route: %d, %v", code, err)
	}
}

func TestMaxBody(t *testing.T) {
	big := bytes.Repeat([]byte("x"), MaxBodyBytes+10)
	h := MaxBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
		}
	}))
	for _, tt := range []struct {
		method, path string
		want         int
	}{
		{http.MethodPut, "/core/v1/12/object", http.StatusOK}, // upload: own cap
		{http.MethodPost, "/core/v1/12/object/head", http.StatusRequestEntityTooLarge},
		{http.MethodPut, "/core/v1/12/object/rename", http.StatusRequestEntityTooLarge},
		{http.MethodPut, "/auth/self", http.StatusRequestEntityTooLarge},
	} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(tt.method, tt.path, bytes.NewReader(big)))
		if rr.Code != tt.want {
			t.Errorf("%s %s: %d, want %d", tt.method, tt.path, rr.Code, tt.want)
		}
	}
}

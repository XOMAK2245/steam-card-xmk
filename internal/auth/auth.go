// Package auth implements Steam login via QR code (IAuthenticationService),
// access token renewal and cookie helpers.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/encoding/protowire"

	"github.com/XOMAK2245/steam-card-xmk/internal/config"
)

const apiBase = "https://api.steampowered.com/IAuthenticationService/"

var httpc = &http.Client{Timeout: 30 * time.Second}

func call(ctx context.Context, method string, body []byte) ([]byte, error) {
	form := url.Values{"input_protobuf_encoded": {base64.StdEncoding.EncodeToString(body)}}
	req, _ := http.NewRequestWithContext(ctx, "POST", apiBase+method+"/v1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 steam-card-xmk")
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s: HTTP %d (eresult %s)", method, resp.StatusCode, resp.Header.Get("X-eresult"))
	}
	return b, nil
}

// fields decodes a flat protobuf message into field-number -> raw value maps.
func fields(b []byte) (bytesF map[int][]byte, varints map[int]uint64) {
	bytesF, varints = map[int][]byte{}, map[int]uint64{}
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return
		}
		b = b[n:]
		switch typ {
		case protowire.VarintType:
			v, m := protowire.ConsumeVarint(b)
			if m < 0 {
				return
			}
			varints[int(num)] = v
			b = b[m:]
		case protowire.BytesType:
			v, m := protowire.ConsumeBytes(b)
			if m < 0 {
				return
			}
			bytesF[int(num)] = v
			b = b[m:]
		default:
			m := protowire.ConsumeFieldValue(num, typ, b)
			if m < 0 {
				return
			}
			b = b[m:]
		}
	}
	return
}

// QRState is the public state of an ongoing QR login.
type QRState struct {
	Active bool   `json:"active"`
	URL    string `json:"url,omitempty"`
	Status string `json:"status"` // idle | waiting | scanned | done | error
	Error  string `json:"error,omitempty"`
}

// QRLogin runs one QR login at a time.
type QRLogin struct {
	mu     sync.Mutex
	state  QRState
	cancel context.CancelFunc
}

func (q *QRLogin) State() QRState {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.state.Status == "" {
		return QRState{Status: "idle"}
	}
	return q.state
}

func (q *QRLogin) set(f func(*QRState)) {
	q.mu.Lock()
	f(&q.state)
	q.mu.Unlock()
}

// Start begins a new QR login; onDone is called with the resulting session.
func (q *QRLogin) Start(onDone func(config.Session)) error {
	q.mu.Lock()
	if q.cancel != nil {
		q.cancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	q.cancel = cancel
	q.state = QRState{Status: "waiting"}
	q.mu.Unlock()

	// CAuthentication_BeginAuthSessionViaQR_Request
	var req []byte
	req = protowire.AppendTag(req, 1, protowire.BytesType)
	req = protowire.AppendString(req, "steam-card-xmk")
	req = protowire.AppendTag(req, 2, protowire.VarintType) // platform_type: WebBrowser
	req = protowire.AppendVarint(req, 2)

	out, err := call(ctx, "BeginAuthSessionViaQR", req)
	if err != nil {
		cancel()
		q.set(func(s *QRState) { s.Status, s.Error = "error", err.Error() })
		return err
	}
	bf, vf := fields(out)
	clientID, requestID, challenge := vf[1], bf[3], string(bf[2])
	if clientID == 0 || challenge == "" {
		cancel()
		err := errors.New("unexpected response from Steam")
		q.set(func(s *QRState) { s.Status, s.Error = "error", err.Error() })
		return err
	}
	q.set(func(s *QRState) { s.Active, s.URL = true, challenge })

	go func() {
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				q.set(func(s *QRState) {
					if s.Status != "done" {
						s.Active, s.Status, s.Error = false, "error", "QR code expired"
					}
				})
				return
			case <-time.After(3 * time.Second):
			}
			var p []byte
			p = protowire.AppendTag(p, 1, protowire.VarintType)
			p = protowire.AppendVarint(p, clientID)
			p = protowire.AppendTag(p, 2, protowire.BytesType)
			p = protowire.AppendBytes(p, requestID)
			out, err := call(ctx, "PollAuthSessionStatus", p)
			if err != nil {
				continue // transient
			}
			bf, vf := fields(out)
			if v := vf[1]; v != 0 {
				clientID = v
			}
			if u := string(bf[2]); u != "" {
				q.set(func(s *QRState) { s.URL = u }) // challenge rotates
			}
			if vf[5] != 0 {
				q.set(func(s *QRState) { s.Status = "scanned" })
			}
			if rt := string(bf[3]); rt != "" {
				sess, err := SessionFromTokens(ctx, rt, string(bf[4]))
				if err != nil {
					q.set(func(s *QRState) { s.Active, s.Status, s.Error = false, "error", err.Error() })
					return
				}
				q.set(func(s *QRState) { s.Active, s.Status, s.URL = false, "done", "" })
				onDone(sess)
				return
			}
		}
	}()
	return nil
}

func (q *QRLogin) Cancel() {
	q.mu.Lock()
	if q.cancel != nil {
		q.cancel()
	}
	q.mu.Unlock()
}

// SessionFromTokens builds a session from a refresh token (and optional access token).
func SessionFromTokens(ctx context.Context, refresh, access string) (config.Session, error) {
	sid, err := jwtSub(refresh)
	if err != nil {
		return config.Session{}, err
	}
	s := config.Session{SteamID: sid, RefreshToken: refresh, AccessToken: access, SessionID: NewSessionID()}
	if access == "" {
		if err := Renew(ctx, &s); err != nil {
			return s, err
		}
	}
	return s, nil
}

// Renew obtains a fresh access token using the refresh token.
func Renew(ctx context.Context, s *config.Session) error {
	if s.RefreshToken == "" {
		return errors.New("no refresh token; please log in again")
	}
	var sid uint64
	fmt.Sscan(s.SteamID, &sid)
	var p []byte
	p = protowire.AppendTag(p, 1, protowire.BytesType)
	p = protowire.AppendString(p, s.RefreshToken)
	p = protowire.AppendTag(p, 2, protowire.Fixed64Type)
	p = protowire.AppendFixed64(p, sid)
	out, err := call(ctx, "GenerateAccessTokenForApp", p)
	if err != nil {
		return err
	}
	bf, _ := fields(out)
	if len(bf[1]) == 0 {
		return errors.New("Steam did not return an access token (refresh token revoked?)")
	}
	s.AccessToken = string(bf[1])
	return nil
}

func NewSessionID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func jwtSub(tok string) (string, error) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return "", errors.New("malformed token")
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	var c struct {
		Sub string `json:"sub"`
		Exp int64  `json:"exp"`
	}
	if err := json.Unmarshal(b, &c); err != nil || c.Sub == "" {
		return "", errors.New("token has no subject")
	}
	return c.Sub, nil
}

// TokenExpiresSoon reports whether the JWT expires within d (or is unparsable).
func TokenExpiresSoon(tok string, d time.Duration) bool {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return true
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return true
	}
	var c struct {
		Exp int64 `json:"exp"`
	}
	json.Unmarshal(b, &c)
	return time.Until(time.Unix(c.Exp, 0)) < d
}

// ParseManual builds a session from manually entered cookies. The loginSecure
// value may be URL-encoded ("7656...%7C%7Ctoken") or plain ("7656...||token").
func ParseManual(loginSecure, sessionID string) (config.Session, error) {
	ls, err := url.QueryUnescape(strings.TrimSpace(loginSecure))
	if err != nil {
		ls = loginSecure
	}
	i := strings.Index(ls, "||")
	if i < 1 {
		return config.Session{}, errors.New("steamLoginSecure must look like 7656119...||token")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		sessionID = NewSessionID()
	}
	return config.Session{SteamID: ls[:i], LoginSecure: ls, SessionID: sessionID}, nil
}

// Cookies returns the cookies to send to steamcommunity.com.
func Cookies(s config.Session) []*http.Cookie {
	ls := s.LoginSecure
	if s.AccessToken != "" {
		ls = s.SteamID + "||" + s.AccessToken
	}
	return []*http.Cookie{
		{Name: "steamLoginSecure", Value: url.QueryEscape(ls), Path: "/", Domain: "steamcommunity.com"},
		{Name: "sessionid", Value: s.SessionID, Path: "/", Domain: "steamcommunity.com"},
		{Name: "Steam_Language", Value: "english", Path: "/", Domain: "steamcommunity.com"},
	}
}

package eventstoreui

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const sessionCookieName = "goes_ui_session"

type sessionClaims struct {
	Username string `json:"u"`
	Expires  int64  `json:"e"`
}

type authenticator struct {
	username      string
	password      string
	secret        []byte
	ttl           time.Duration
	secureCookies bool
	now           func() time.Time
}

func newAuthenticator(cfg Config) *authenticator {
	return &authenticator{
		username:      cfg.Username,
		password:      cfg.Password,
		secret:        cfg.SessionSecret,
		ttl:           cfg.SessionTTL,
		secureCookies: cfg.SecureCookies,
		now:           time.Now,
	}
}

func (auth *authenticator) enabled() bool {
	return auth.username != "" && auth.password != ""
}

func (auth *authenticator) validCredentials(username, password string) bool {
	if !auth.enabled() {
		return false
	}
	userMatch := subtle.ConstantTimeCompare([]byte(username), []byte(auth.username))
	passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(auth.password))
	return userMatch&passwordMatch == 1
}

func (auth *authenticator) setSession(w http.ResponseWriter, r *http.Request) error {
	claims := sessionClaims{Username: auth.username, Expires: auth.now().Add(auth.ttl).Unix()}
	payload, err := json.Marshal(claims)
	if err != nil {
		return err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	signature := auth.sign(encoded)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    encoded + "." + signature,
		Path:     "/",
		Expires:  time.Unix(claims.Expires, 0),
		MaxAge:   int(auth.ttl.Seconds()),
		HttpOnly: true,
		Secure:   auth.secureCookies || r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteStrictMode,
	})
	return nil
}

func (auth *authenticator) clearSession(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   auth.secureCookies || r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteStrictMode,
	})
}

func (auth *authenticator) authenticated(r *http.Request) bool {
	if !auth.enabled() {
		return true
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	var encoded, signature string
	for i := range cookie.Value {
		if cookie.Value[i] == '.' {
			encoded, signature = cookie.Value[:i], cookie.Value[i+1:]
			break
		}
	}
	if encoded == "" || signature == "" || !hmac.Equal([]byte(signature), []byte(auth.sign(encoded))) {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return false
	}
	var claims sessionClaims
	if json.Unmarshal(payload, &claims) != nil {
		return false
	}
	return claims.Username == auth.username && claims.Expires > auth.now().Unix()
}

func (auth *authenticator) sign(payload string) string {
	mac := hmac.New(sha256.New, auth.secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func requireSameOrigin(r *http.Request) error {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return nil
	}
	parsed, err := url.Parse(origin)
	if err != nil || !strings.EqualFold(parsed.Host, r.Host) {
		return fmt.Errorf("origin does not match request host")
	}
	return nil
}

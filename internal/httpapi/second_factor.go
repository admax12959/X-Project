package httpapi

import (
	"net/http"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
)

// SecondFactorCookie carries the short-lived token of a login whose password was right but
// whose second step is still open. It is not a session: only POST /api/session/second-factor
// reads it, and the browser only sends it there (Path).
const SecondFactorCookie = "xp_second_factor"

const secondFactorPath = "/api/session/second-factor"

func (s *server) secondFactorRoutes() {
	s.publicRoute("POST "+secondFactorPath, s.submitSecondFactor)
	s.route("GET /api/me/second-factor", s.secondFactorStatus)
	s.route("POST /api/me/second-factor/totp", s.startTOTP)
	s.route("POST /api/me/second-factor/totp/confirm", s.confirmTOTP)
	s.route("POST /api/me/second-factor/recovery-codes", s.regenerateRecoveryCodes)
	s.route("POST /api/me/second-factor/disable", s.disableSecondFactor)
}

func (s *server) setSecondFactorCookie(w http.ResponseWriter, token string, expires time.Time) {
	c := &http.Cookie{
		Name: SecondFactorCookie, Value: token, Path: secondFactorPath,
		HttpOnly: true, Secure: s.opts.CookieSecure, SameSite: http.SameSiteLaxMode,
	}
	if token == "" {
		c.MaxAge = -1
	} else {
		c.Expires = expires
	}
	http.SetCookie(w, c)
}

// secondFactorRequiredJSON answers a login whose first factor passed and that now needs a code.
type secondFactorRequiredJSON struct {
	SecondFactorRequired bool   `json:"secondFactorRequired"`
	ExpiresAt            string `json:"expiresAt"`
}

// loginResponse sets the cookie of whichever step the login reached and writes the answer.
func (s *server) loginResponse(w http.ResponseWriter, res domain.LoginResult) {
	if res.SecondFactorToken != "" {
		s.setSecondFactorCookie(w, res.SecondFactorToken, res.SecondFactorExpiresAt)
		writeJSON(w, http.StatusOK, secondFactorRequiredJSON{SecondFactorRequired: true, ExpiresAt: formatTime(res.SecondFactorExpiresAt)})
		return
	}
	s.setSessionCookie(w, res.Token, res.ExpiresAt)
	writeJSON(w, http.StatusOK, sessionJSON{User: toUserJSON(res.User), MustChangePassword: res.MustChangePassword, SecondFactorSetupRequired: res.SecondFactorSetupRequired})
}

func (s *server) submitSecondFactor(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &body) {
		return
	}
	token := ""
	if c, err := r.Cookie(SecondFactorCookie); err == nil {
		token = c.Value
	}
	res, err := s.svc.SubmitSecondFactor(r.Context(), token, body.Code)
	if err != nil {
		writeError(w, r, err)
		return
	}
	s.setSecondFactorCookie(w, "", time.Time{}) // spent
	s.loginResponse(w, res)
}

type authenticatorJSON struct {
	Type      string `json:"type"`
	EnabledAt string `json:"enabledAt"`
}

type secondFactorStatusJSON struct {
	Enabled        bool                `json:"enabled"`
	Authenticators []authenticatorJSON `json:"authenticators"`
	// RecoveryCodesRemaining counts the unspent recovery codes. The codes themselves are only
	// in the answers that make them (enabling, regenerating).
	RecoveryCodesRemaining int `json:"recoveryCodesRemaining"`
}

func (s *server) secondFactorStatus(w http.ResponseWriter, r *http.Request) {
	s.writeSecondFactorStatus(w, r, nil)
}

// writeSecondFactorStatus answers with the status, plus the fresh recovery codes when codes
// is not nil.
func (s *server) writeSecondFactorStatus(w http.ResponseWriter, r *http.Request, codes []string) {
	st, err := s.svc.SecondFactorStatus(r.Context(), actorOf(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := secondFactorStatusJSON{Enabled: st.Enabled, Authenticators: []authenticatorJSON{}, RecoveryCodesRemaining: st.RecoveryCodesRemaining}
	for _, a := range st.Authenticators {
		out.Authenticators = append(out.Authenticators, authenticatorJSON{Type: string(a.Type), EnabledAt: formatTime(a.EnabledAt)})
	}
	if codes == nil {
		writeJSON(w, http.StatusOK, out)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		secondFactorStatusJSON
		RecoveryCodes []string `json:"recoveryCodes"`
	}{out, codes})
}

func (s *server) startTOTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	setup, err := s.svc.StartTOTPSetup(r.Context(), actorOf(r), body.Password)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"secret": setup.Secret, "otpauthUri": setup.OtpauthURI, "qrSvg": setup.QrSVG})
}

func (s *server) confirmTOTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &body) {
		return
	}
	codes, err := s.svc.ConfirmTOTP(r.Context(), actorOf(r), body.Code)
	if err != nil {
		writeError(w, r, err)
		return
	}
	s.writeSecondFactorStatus(w, r, codes)
}

func (s *server) regenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &body) {
		return
	}
	codes, err := s.svc.RegenerateRecoveryCodes(r.Context(), actorOf(r), body.Code)
	if err != nil {
		writeError(w, r, err)
		return
	}
	s.writeSecondFactorStatus(w, r, codes)
}

func (s *server) disableSecondFactor(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &body) {
		return
	}
	if err := s.svc.DisableSecondFactor(r.Context(), actorOf(r), body.Code); err != nil {
		writeError(w, r, err)
		return
	}
	s.writeSecondFactorStatus(w, r, nil)
}

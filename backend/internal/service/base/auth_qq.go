package base

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/example/aichat/backend/internal/biz/base/loginprovider"
	"github.com/example/aichat/backend/models/generator/model"
	"github.com/go-kratos/kratos/v2/log"
)

func (s *AuthService) HandleQQLogin(w http.ResponseWriter, r *http.Request) {
	redirectURL := strings.TrimSpace(r.URL.Query().Get("redirect"))
	loginURL, err := s.user.BuildQQLoginURL(r.Context(), redirectURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, loginURL, http.StatusFound)
}

func (s *AuthService) HandleQQCallback(w http.ResponseWriter, r *http.Request) {
	redirectURL := s.user.PopQQRedirectURL(strings.TrimSpace(r.URL.Query().Get("state")))
	if redirectURL == "" {
		redirectURL = s.user.QQDefaultRedirectURL()
	}
	if redirectURL == "" {
		redirectURL = "/"
	}

	if oauthErr := strings.TrimSpace(r.URL.Query().Get("error")); oauthErr != "" {
		http.Redirect(w, r, appendQuery(redirectURL, "qq_error", oauthErr), http.StatusFound)
		return
	}

	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		http.Redirect(w, r, appendQuery(redirectURL, "qq_error", "missing_code"), http.StatusFound)
		return
	}

	result, err := s.user.Login(r.Context(), &loginprovider.LoginRequest{
		AuthType: model.AuthTypeQQ,
		AuthCode: code,
		State:    strings.TrimSpace(r.URL.Query().Get("state")),
	})
	if err != nil {
		log.Errorf("qq login failed: %v", err)
		http.Redirect(w, r, appendQuery(redirectURL, "qq_error", "qq_login_failed"), http.StatusFound)
		return
	}

	http.Redirect(w, r, appendQuery(redirectURL, "qq_token", result.Token), http.StatusFound)
}

func appendQuery(rawURL, key, value string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u == nil {
		u = &url.URL{Path: "/"}
	}
	q := u.Query()
	q.Set(key, value)
	u.RawQuery = q.Encode()
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}



package loginprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/example/aichat/backend/internal/conf"
	"github.com/example/aichat/backend/internal/consts"
	"github.com/example/aichat/backend/models"
	"github.com/example/aichat/backend/models/generator/model"
	"github.com/example/aichat/backend/pkg/auth"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	redislib "github.com/redis/go-redis/v9"
)

type qqOAuthConfig struct {
	AppID            string
	AppKey           string
	CallbackURL      string
	FrontendRedirect string
	Scope            string
}

var qqHTTPClient = &http.Client{Timeout: 10 * time.Second}

type QQProvider struct {
	userRepo UserRepo
	authRepo auth.AuthRepo
	redis    StateCache
	qqConf   *conf.Auth_QQ
}

func NewQQProvider(userRepo UserRepo, authRepo auth.AuthRepo, redisRepo StateCache, qqConf *conf.Auth_QQ) *QQProvider {
	return &QQProvider{
		userRepo: userRepo,
		authRepo: authRepo,
		redis:    redisRepo,
		qqConf:   qqConf,
	}
}

func (p *QQProvider) Type() model.AuthType {
	return model.AuthTypeQQ
}

func (p *QQProvider) BuildLoginURL(ctx context.Context, redirectURL string) (string, error) {
	cfg, err := p.loadQQOAuthConfig()
	if err != nil {
		return "", err
	}

	redirectURL = strings.TrimSpace(redirectURL)
	if redirectURL == "" {
		redirectURL = cfg.FrontendRedirect
	}
	if redirectURL == "" {
		redirectURL = "/"
	}

	state, err := RandomHex(16)
	if err != nil {
		return "", err
	}
	if err = p.putState(ctx, state, redirectURL); err != nil {
		return "", err
	}

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", cfg.AppID)
	params.Set("redirect_uri", cfg.CallbackURL)
	params.Set("state", state)
	params.Set("scope", cfg.Scope)

	return "https://graph.qq.com/oauth2.0/authorize?" + params.Encode(), nil
}

func (p *QQProvider) DefaultRedirectURL() string {
	cfg, err := p.loadQQOAuthConfig()
	if err != nil {
		return "/"
	}
	if strings.TrimSpace(cfg.FrontendRedirect) == "" {
		return "/"
	}
	return strings.TrimSpace(cfg.FrontendRedirect)
}

func (p *QQProvider) PopRedirectURL(state string) string {
	if strings.TrimSpace(state) == "" {
		return ""
	}
	if p.redis == nil {
		return ""
	}

	value, err := p.redis.GetDel(context.Background(), fmt.Sprintf(consts.RedisKeyQQState, state))
	if err != nil {
		if errors.Is(err, redislib.Nil) {
			return ""
		}
		return ""
	}
	return strings.TrimSpace(value)
}

func (p *QQProvider) Login(ctx context.Context, req *LoginRequest) (*LoginResult, error) {
	code := strings.TrimSpace(req.AuthCode)
	if code == "" {
		return nil, kerrors.BadRequest("INVALID_ARGUMENT", "qq auth code is empty")
	}

	cfg, err := p.loadQQOAuthConfig()
	if err != nil {
		return nil, err
	}

	accessToken, err := exchangeQQAccessToken(ctx, cfg, code)
	if err != nil {
		return nil, err
	}

	openID, err := fetchQQOpenID(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	nickname, avatar, err := fetchQQUserInfo(ctx, cfg.AppID, accessToken, openID)
	if err != nil {
		log.Warnf("qq get user info failed, continue with openid only: %v", err)
	}

	record, err := p.userRepo.GetUserAuthByTypeAndIdentifier(ctx, model.AuthTypeQQ, openID)
	if err != nil {
		return nil, err
	}

	var user *model.SysUser
	if record != nil {
		user, err = p.userRepo.GetById(ctx, record.UserID)
		if err != nil {
			return nil, err
		}
	}

	if user == nil {
		account, err := BuildQQAccount(ctx, p.userRepo, openID)
		if err != nil {
			return nil, err
		}

		name := strings.TrimSpace(nickname)
		if name == "" {
			name = account
		}

		user = &model.SysUser{
			Type:    model.SysUserType_Guest,
			Account: account,
			Name:    name,
			Avatar:  strings.TrimSpace(avatar),
			Status:  models.Status_Enabled,
		}
		user.New()
		if err = p.userRepo.Create(ctx, user); err != nil {
			return nil, err
		}

		record = &model.SysUserAuth{
			UserID:     user.ID,
			AuthType:   model.AuthTypeQQ,
			Identifier: openID,
			Secret:     "",
		}
		record.New()
		if err = p.userRepo.CreateUserAuth(ctx, record); err != nil {
			return nil, err
		}
	}

	token, err := IssueToken(ctx, p.authRepo, user)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Token: token, User: user}, nil
}

func (p *QQProvider) putState(ctx context.Context, state, redirectURL string) error {
	if p.redis == nil {
		return fmt.Errorf("redis is not configured")
	}
	return p.redis.Set(ctx, fmt.Sprintf(consts.RedisKeyQQState, state), redirectURL, 10*time.Minute)
}

func (p *QQProvider) loadQQOAuthConfig() (*qqOAuthConfig, error) {
	if p.qqConf == nil {
		return nil, fmt.Errorf("qq oauth config missing")
	}
	cfg := &qqOAuthConfig{
		AppID:            strings.TrimSpace(p.qqConf.GetAppId()),
		AppKey:           strings.TrimSpace(p.qqConf.GetAppKey()),
		CallbackURL:      strings.TrimSpace(p.qqConf.GetCallbackUrl()),
		FrontendRedirect: strings.TrimSpace(p.qqConf.GetFrontendRedirect()),
		Scope:            strings.TrimSpace(p.qqConf.GetScope()),
	}
	if cfg.Scope == "" {
		cfg.Scope = "get_user_info"
	}
	if cfg.AppID == "" || cfg.AppKey == "" || cfg.CallbackURL == "" {
		return nil, fmt.Errorf("qq oauth config missing, require auth.qq.app_id/app_key/callback_url")
	}
	return cfg, nil
}

func exchangeQQAccessToken(ctx context.Context, cfg *qqOAuthConfig, code string) (string, error) {
	params := url.Values{}
	params.Set("grant_type", "authorization_code")
	params.Set("client_id", cfg.AppID)
	params.Set("client_secret", cfg.AppKey)
	params.Set("code", code)
	params.Set("redirect_uri", cfg.CallbackURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://graph.qq.com/oauth2.0/token?"+params.Encode(), nil)
	if err != nil {
		return "", err
	}

	resp, err := qqHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("qq token http status=%d body=%s", resp.StatusCode, truncateForLog(string(body)))
	}

	bodyText := strings.TrimSpace(string(body))
	parsed, err := url.ParseQuery(bodyText)
	if err == nil {
		if accessToken := strings.TrimSpace(parsed.Get("access_token")); accessToken != "" {
			return accessToken, nil
		}
	}

	return "", fmt.Errorf("qq token response invalid: %s", truncateForLog(bodyText))
}

func fetchQQOpenID(ctx context.Context, accessToken string) (string, error) {
	params := url.Values{}
	params.Set("access_token", accessToken)
	params.Set("fmt", "json")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://graph.qq.com/oauth2.0/me?"+params.Encode(), nil)
	if err != nil {
		return "", err
	}

	resp, err := qqHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("qq me http status=%d body=%s", resp.StatusCode, truncateForLog(string(body)))
	}

	var payload struct {
		OpenID string `json:"openid"`
		Error  int    `json:"error"`
		Msg    string `json:"error_description"`
	}
	if err = json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.Error != 0 {
		return "", fmt.Errorf("qq me error=%d msg=%s", payload.Error, payload.Msg)
	}
	if strings.TrimSpace(payload.OpenID) == "" {
		return "", fmt.Errorf("qq openid missing")
	}
	return payload.OpenID, nil
}

func fetchQQUserInfo(ctx context.Context, appID, accessToken, openID string) (nickname, avatar string, err error) {
	params := url.Values{}
	params.Set("access_token", accessToken)
	params.Set("oauth_consumer_key", appID)
	params.Set("openid", openID)
	params.Set("format", "json")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://graph.qq.com/user/get_user_info?"+params.Encode(), nil)
	if err != nil {
		return "", "", err
	}

	resp, err := qqHTTPClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("qq user info http status=%d body=%s", resp.StatusCode, truncateForLog(string(body)))
	}

	var payload struct {
		Ret          int    `json:"ret"`
		Msg          string `json:"msg"`
		Nickname     string `json:"nickname"`
		FigureurlQQ  string `json:"figureurl_qq_1"`
		FigureurlQQ2 string `json:"figureurl_qq_2"`
	}
	if err = json.Unmarshal(body, &payload); err != nil {
		return "", "", err
	}
	if payload.Ret != 0 {
		return "", "", fmt.Errorf("qq user info ret=%d msg=%s", payload.Ret, payload.Msg)
	}

	avatar = strings.TrimSpace(payload.FigureurlQQ2)
	if avatar == "" {
		avatar = strings.TrimSpace(payload.FigureurlQQ)
	}
	return strings.TrimSpace(payload.Nickname), avatar, nil
}

func truncateForLog(s string) string {
	const maxLen = 240
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
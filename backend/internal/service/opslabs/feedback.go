package opslabs

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// FeedbackHandler 用户反馈收集端点(不走 proto,避免为一个纯副作用 API 跑 proto regen)
//
// 端点:POST /v1/feedback
// 请求:
//
//	{
//	  "text":         "...",       // required, 1..2000 chars,去掉首尾空白
//	  "scenarioSlug": "...",       // optional,当前所在场景
//	  "attemptId":    "...",       // optional,当前 attempt(排障上下文)
//	  "rating":       1..5         // optional,星级
//	}
//
// 响应:
//
//	200 {"ok":true}
//	400 {"ok":false,"error":"reason"}
//
// 存储策略(V1):直接 log.Info 进服务日志,用户反馈走结构化 zap 字段便于 grep。
// 内测收集量小、保留时间短,不做 DB 表 / registry,等量上来再建模型。
// clientID 从 middleware 注入的 X-Client-ID 读,匿名但能做聚合统计。
func FeedbackHandler(log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeFeedbackErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
		if err != nil {
			writeFeedbackErr(w, http.StatusBadRequest, "read body failed")
			return
		}

		var req struct {
			Text         string `json:"text"`
			ScenarioSlug string `json:"scenarioSlug"`
			AttemptID    string `json:"attemptId"`
			Rating       int    `json:"rating"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeFeedbackErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		req.Text = strings.TrimSpace(req.Text)
		if req.Text == "" {
			writeFeedbackErr(w, http.StatusBadRequest, "text required")
			return
		}
		if len([]rune(req.Text)) > 2000 {
			writeFeedbackErr(w, http.StatusBadRequest, "text too long (max 2000 chars)")
			return
		}
		if req.Rating < 0 || req.Rating > 5 {
			writeFeedbackErr(w, http.StatusBadRequest, "rating must be 0..5")
			return
		}

		clientID := ""
		if cid := r.Header.Get("X-Client-ID"); cid != "" {
			clientID = cid
		}

		log.Info("user feedback",
			zap.String("client_id", clientID),
			zap.String("scenario_slug", req.ScenarioSlug),
			zap.String("attempt_id", req.AttemptID),
			zap.Int("rating", req.Rating),
			zap.String("text", req.Text),
			zap.String("ua", r.UserAgent()),
			zap.Time("at", time.Now()),
		)

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}
}

func writeFeedbackErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	// msg 都是本文件 hard-coded 的 ASCII 文案,不会含反斜杠 / 引号 / 控制字符,
	// 直接拼接比 json.Marshal 一个 map 轻很多,又绝对安全
	_, _ = w.Write([]byte(`{"ok":false,"error":"` + msg + `"}`))
}

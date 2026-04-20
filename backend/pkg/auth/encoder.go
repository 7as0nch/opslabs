// Package auth @author <chengjiang@buffalo-robot.com>
// @date 2023/1/10
// @note
package auth

import (
	"encoding/json"
	"errors"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"reflect"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/transport/http"
)

func DefaultResponseEncoder(w http.ResponseWriter, r *http.Request, v interface{}) error {
	var response struct {
		Code     int         `json:"code"`
		Data     interface{} `json:"data"`
		Msg      string      `json:"msg"`
		NewToken string      `json:"newToken"`
		DateTime string      `json:"datetime"`
	}

	response.Code = 200
	response.DateTime = time.Now().Format(time.DateTime)
	response.Msg = "成功啦，宝子，你可真棒！"
	if v == nil {
		return nil
	}
	if !reflect.ValueOf(v).IsNil() {
		codec, _ := http.CodecForRequest(r, "Accept")
		data, err := codec.Marshal(v)
		if err != nil {
			return err
		}
		response.Data = json.RawMessage(data)
	}
	codec, _ := http.CodecForRequest(r, "Accept")
	data, err := codec.Marshal(&response)

	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", ContentType(codec.Name()))
	_, err = w.Write(data)
	if err != nil {
		return err
	}
	return nil
}

func DefaultErrorEncoder(w http.ResponseWriter, r *http.Request, err error) {
	var response struct {
		Msg      string `json:"msg"`
		Code     int    `json:"code"`
		DateTime int64  `json:"datetime"`
		ClientID string `json:"clientID"`
	}

	response.Msg = err.Error()
	response.DateTime = time.Now().UnixMilli()
	response.Code = 500
	if se := new(kerrors.Error); errors.As(err, &se) {
		se = kerrors.FromError(err)
		w.WriteHeader(int(se.Code))
		response.Code = int(se.Code)
		response.Msg = se.Message
	} else {
		w.WriteHeader(200)
	}

	codec, _ := http.CodecForRequest(r, "Accept")
	body, err := json.Marshal(&response)
	if err != nil {
		panic(err)
	}
	w.Header().Set("Content-Type", ContentType(codec.Name()))
	// w.WriteHeader(200)
	w.Write(body)
}

const (
	baseContentType = "application"
)

func ContentType(subtype string) string {
	return strings.Join([]string{baseContentType, subtype}, "/")
}

package javaApi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

)

// GetJavaRpcServ 获取java的rpc服务地址
// e.g.: requestUrl: /pc/screen/v1/realTimeFiled?radarId=&field=
// author: cheng jiang
func GetJavaRpcServ(requestUrl string) (string, error) {
	getUrl := requestUrl
	println("the url:", getUrl)
	resp, err := http.Get(getUrl)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

type RespBody struct {
	Code string                    `json:"code"`
	Msg  string                    `json:"message"`
	Data map[string]map[string]any `json:"data"`
}

func PostJavaRpcServ(requestUrl string, postBody PostBody) (string, error) {
	postUrl := requestUrl
	println("the url:", postUrl)
	postData, err := json.Marshal(postBody)
	if err != nil {
		return "", err
	}
	resp, err := http.Post(postUrl, "application/json", bytes.NewBuffer(postData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return fmt.Sprintf("%v", resp.StatusCode), nil
}

// PostBody post请求体结构体：
type PostBody struct {
	// id
	Id *string `json:"id,omitempty"`
	//endDataTime: "2023-04-13 16:04:57"
	EndDataTime string `json:"endDataTime"`
	//radarId: "47ad1a046ff949708c7909989e975d34"
	RadarId string `json:"radarId"`
	//radarTypeId: "72a4063f888f48408edf2bbf592808da"
	RadarTypeId string `json:"radarTypeId"`
	//simulationDataType: 2
	SimulationDataType int32 `json:"simulationDataType"`
	//startDataTime: "2023-04-12 16:04:52"
	StartDataTime string `json:"startDataTime"`
	//state: 1
	State int32 `json:"state"`
}

func GetJavaRpcServ4Simulate(requestUrl string) (*RespBody4Simulate, error) {
	getUrl :=  requestUrl
	println("the url:", getUrl)
	resp, err := http.Get(getUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// RespBody4Simulate
	var respBody4Simulate RespBody4Simulate
	err = json.Unmarshal(body, &respBody4Simulate)
	if err != nil {
		return nil, err
	}
	return &respBody4Simulate, nil
}

type RespBody4Simulate struct {
	Code string           `json:"code"`
	Msg  string           `json:"message"`
	Data []map[string]any `json:"data"`
}

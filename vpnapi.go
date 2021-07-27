package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
	"github.com/thoas/go-funk"
)

type jsonrpcMessage struct {
	Version string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *jsonError      `json:"error,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

type jsonError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (err *jsonError) Error() string {
	if err.Message == "" {
		return fmt.Sprintf("json-rpc error %d", err.Code)
	}
	return err.Message
}

func (err *jsonError) ErrorCode() int {
	return err.Code
}

func (err *jsonError) ErrorData() interface{} {
	return err.Data
}

type SoftEtherAPIClient struct {
	c                *resty.Client
	PrintRawResponse bool
}

func NewSoftEtherAPIClient(endpoint string) *SoftEtherAPIClient {
	c := resty.New()
	c.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	c.SetHostURL(endpoint)
	c.SetBasicAuth("administrator", "")

	return &SoftEtherAPIClient{c: c}
}

func (c *SoftEtherAPIClient) Call(method string, params interface{}, result interface{}) error {
	var p json.RawMessage
	if params == nil {
		p = []byte("{}")
	} else {
		var err error
		p, err = json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal params: %w", err)
		}
	}

	msg := jsonrpcMessage{
		Version: "2.0",
		ID:      []byte("0"),
		Method:  method,
		Params:  p,
	}

	res, err := c.c.R().SetBody(msg).SetResult(jsonrpcMessage{}).Post("")
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	if c.PrintRawResponse {
		log.Infof("%+v", res)
	}

	rpcResponse := res.Result().(*jsonrpcMessage)
	if rpcResponse.Error != nil {
		return rpcResponse.Error
	}

	if result != nil {
		if err := json.Unmarshal(rpcResponse.Result, result); err != nil {
			return fmt.Errorf("unmarshal result: %w", err)
		}
	}

	return nil
}

type Session struct {
	Name        string    `json:"Name_str"`
	Username    string    `json:"Username_str"`
	Hostname    string    `json:"Hostname_str"`
	IP          string    `json:"Ip_ip"`
	CreatedTime time.Time `json:"CreatedTime_dt"`
}

func (c *SoftEtherAPIClient) EnumSession(hub string) ([]*Session, error) {
	p := map[string]interface{}{
		"HubName_str": hub,
	}

	var res struct {
		SessionList []*Session
	}
	if err := c.Call("EnumSession", p, &res); err != nil {
		return nil, err
	}

	ss := funk.Filter(res.SessionList, func(s *Session) bool {
		return s.Username != "SecureNAT"
	}).([]*Session)

	return ss, nil
}

package main

import (
	"errors"
	"net/url"
	"strings"

	"github.com/go-resty/resty/v2"
)

const APIBaseURL = "https://oms.every8d.com/API21/HTTP/sendSMS.ashx"

type SMSClient struct {
	username      string
	password      string
	notifyNumbers []string
	client        *resty.Client
}

func NewSMSClient(username, password string, notifyNumbers []string) *SMSClient {
	c := resty.New()

	return &SMSClient{
		username:      username,
		password:      password,
		notifyNumbers: notifyNumbers,
		client:        c,
	}
}

func (c *SMSClient) Send(msg string) error {
	params := url.Values{}
	params.Set("UID", c.username)
	params.Set("PWD", c.password)
	params.Set("SB", "")
	params.Set("MSG", msg)
	params.Set("DEST", strings.Join(c.notifyNumbers, ","))
	params.Set("ST", "")
	params.Set("RETRYTIME", "5")

	res, err := c.client.R().SetQueryParamsFromValues(params).Get(APIBaseURL)
	if err != nil {
		return err
	}

	resFields := strings.Split(res.String(), ",")

	if resFields[0] == "-99" {
		return errors.New("send sms: unknown error")
	}

	return nil
}

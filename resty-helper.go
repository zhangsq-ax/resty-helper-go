package resty_helper

import (
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/zhangsq-ax/logs"
	"go.uber.org/zap"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	restyClients = sync.Map{}
)

type BasicAuth struct {
	Username string
	Password string
}

type RestyClientOptions struct {
	BasicAuth *BasicAuth
	BaseUrl   string
	Headers   map[string]string
}

func GetRestyClient(opts *RestyClientOptions) *resty.Client {
	var (
		client *resty.Client
		ok     bool
		c      any
	)
	if c, ok = restyClients.Load(opts.BaseUrl); !ok {
		client = resty.New().
			SetTransport(&http.Transport{
				MaxIdleConnsPerHost: 10,
			}).
			SetRetryCount(3).
			SetRetryWaitTime(5 * time.Second).
			OnBeforeRequest(func(c *resty.Client, r *resty.Request) error {
				logs.Debugw("send-http-request", zap.String("method", r.Method), zap.String("url", opts.BaseUrl+r.URL), zap.Reflect("headers", r.Header), zap.Reflect("body", r.Body))
				return nil
			}).
			OnAfterResponse(func(c *resty.Client, r *resty.Response) error {
				logs.Debugw("receive-http-response", zap.String("url", r.Request.URL), zap.Int("statusCode", r.StatusCode()), zap.ByteString("body", r.Body()))
				return nil
			})
		if opts.BaseUrl != "" {
			client.SetBaseURL(opts.BaseUrl)
		}
		if opts.Headers != nil {
			client.SetHeaders(opts.Headers)
		}
		if opts.BasicAuth != nil {
			client.SetBasicAuth(opts.BasicAuth.Username, opts.BasicAuth.Password)
		}
		restyClients.Store(opts.BaseUrl, client)
		c = client
	}
	client = c.(*resty.Client)
	return client
}

func Request(client *resty.Client, method string, url string, body ...any) ([]byte, error) {
	req := client.R()
	if len(body) > 0 {
		req.SetBody(body[0])
	}
	var request func(url string) (*resty.Response, error)
	method = strings.ToUpper(method)
	switch method {
	case "POST":
		request = req.Post
	case "GET":
		request = req.Get
	case "PUT":
		request = req.Put
	case "PATCH":
		request = req.Patch
	case "DELETE":
		request = req.Delete
	default:
		return nil, fmt.Errorf("unknown request method: %s", method)
	}
	res, err := request(url)
	if err != nil {
		return nil, err
	}
	if res.StatusCode()/100 != 2 {
		return nil, fmt.Errorf("%d: %s", res.StatusCode(), res.Status())
	}
	return res.Body(), nil
}

func RequestWithProcess(client *resty.Client, method string, url string, processor func(body []byte) (any, error), body ...any) (any, error) {
	res, err := Request(client, method, url, body...)
	if err != nil {
		return nil, err
	}

	return processor(res)
}

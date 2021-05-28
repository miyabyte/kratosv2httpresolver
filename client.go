package kratosv2httpresolver

import (
	"context"
	"fmt"
	"github.com/go-kratos/kratos/v2/encoding"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport"
	http2 "github.com/go-kratos/kratos/v2/transport/http"
	"google.golang.org/grpc/resolver"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// ClientOption is HTTP client option.
type ClientOption func(*clientOptions)

// WithTimeout with client request timeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(o *clientOptions) {
		o.timeout = d
	}
}

// WithUserAgent with client user agent.
func WithUserAgent(ua string) ClientOption {
	return func(o *clientOptions) {
		o.userAgent = ua
	}
}

// WithTransport with client transport.
func WithTransport(trans http.RoundTripper) ClientOption {
	return func(o *clientOptions) {
		o.transport = trans
	}
}

// WithMiddleware with client middleware.
func WithMiddleware(m middleware.Middleware) ClientOption {
	return func(o *clientOptions) {
		o.middleware = m
	}
}

// WithEndpoint with client endpoint.
//func WithEndpoint(endpoint string) ClientOption {
//	return func(o *clientOptions) {
//		o.endpoint = endpoint
//	}
//}

type Builder interface {
	Build() error
	Schema() string
	//UpdateState(resolver.State)
	GetState() resolver.State
}

// Client is a HTTP transport client.
type clientOptions struct {
	endpoint   string
	ctx        context.Context
	timeout    time.Duration
	userAgent  string
	transport  http.RoundTripper
	middleware middleware.Middleware
}

type Client struct {
	hc        *http.Client
	resolvers map[string]Builder
}

// NewClient returns an HTTP client.
func NewClient(ctx context.Context, opts ...ClientOption) (*Client, error) {
	trans, err := NewTransport(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{
		hc:        &http.Client{Transport: trans},
		resolvers: make(map[string]Builder),
	}, nil
}

func (c *Client) AddDiscovery(d registry.Discovery, target string) error {
	b := NewBuilder(d, target)
	e := b.Build()
	if e != nil {
		return e
	}
	c.resolvers[target] = b
	return nil
}

func parseTarget(target string) (string, string, bool) {
	spl := strings.SplitN(target, "://", 2)
	if len(spl) < 2 {
		return "", "", false
	}
	return strings.ToLower(spl[0]), spl[1], true
}

// NewTransport creates an http.RoundTripper.
func NewTransport(ctx context.Context, opts ...ClientOption) (http.RoundTripper, error) {
	options := &clientOptions{
		ctx:       ctx,
		timeout:   500 * time.Millisecond,
		transport: http.DefaultTransport,
	}
	for _, o := range opts {
		o(options)
	}
	return &baseTransport{
		middleware: options.middleware,
		userAgent:  options.userAgent,
		timeout:    options.timeout,
		base:       options.transport,
	}, nil
}

type baseTransport struct {
	userAgent  string
	timeout    time.Duration
	base       http.RoundTripper
	middleware middleware.Middleware
}

func (t *baseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.userAgent != "" && req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", t.userAgent)
	}
	ctx := transport.NewContext(req.Context(), transport.Transport{Kind: transport.KindHTTP})
	ctx = http2.NewClientContext(ctx, http2.ClientInfo{Request: req})
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	h := func(ctx context.Context, in interface{}) (interface{}, error) {
		return t.base.RoundTrip(in.(*http.Request))
	}
	if t.middleware != nil {
		h = t.middleware(h)
	}
	res, err := h(ctx, req)
	if err != nil {
		return nil, err
	}
	return res.(*http.Response), nil
}

func (c *Client) NewRequest(method, target, path string, body io.Reader) (*http.Request, error) {
	addr, ok := c.getAddr(target)
	if !ok || addr == nil {
		return nil, fmt.Errorf("getResolver %v notfound", target)
	}
	return http.NewRequest(method, "http://"+addr.Addr+path, body)
}

func (c *Client) getAddr(target string) (*resolver.Address, bool) {
	r, ok := c.resolvers[target]
	if !ok {
		return nil, false
	}
	state := r.GetState()
	if len(state.Addresses) == 0 {
		return nil, false
	}
	rand.Seed(time.Now().Unix())
	addr := state.Addresses[rand.Intn(len(state.Addresses))]
	return &addr, ok
}

// Do send an HTTP request and decodes the body of response into target.
// returns an error (of type *Error) if the response status code is not 2xx.
func (c *Client) Do(req *http.Request, target interface{}) error {
	res, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		se := &errors.Error{Code: 2}
		if err := decodeResponse(res, se); err != nil {
			return err
		}
		return se
	}
	return decodeResponse(res, target)
}

func (c *Client) Send(req *http.Request, target interface{}) (*http.Response, error) {
	return c.hc.Do(req)
}

func decodeResponse(res *http.Response, target interface{}) error {
	subtype := contentSubtype(res.Header.Get(contentTypeHeader))
	codec := encoding.GetCodec(subtype)
	if codec == nil {
		codec = encoding.GetCodec("json")
	}
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	return codec.Unmarshal(data, target)
}

//--todo mr del
const (
	// SupportPackageIsVersion1 These constants should not be referenced from any other code.
	SupportPackageIsVersion1 = true

	baseContentType = "application"
)

var (
	acceptHeader      = http.CanonicalHeaderKey("Accept")
	contentTypeHeader = http.CanonicalHeaderKey("Content-Type")
)

func contentSubtype(contentType string) string {
	if contentType == baseContentType {
		return ""
	}
	if !strings.HasPrefix(contentType, baseContentType) {
		return ""
	}
	switch contentType[len(baseContentType)] {
	case '/', ';':
		if i := strings.Index(contentType, ";"); i != -1 {
			return contentType[len(baseContentType)+1 : i]
		}
		return contentType[len(baseContentType)+1:]
	default:
		return ""
	}
}

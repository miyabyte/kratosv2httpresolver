package kratosv2httpresolver

import (
	"context"
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/registry"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/resolver"
	"net/url"
)

const name = "discovery"

// Option is builder option.
type Option func(o *builder)

// WithLogger with builder logger.
func WithLogger(logger log.Logger) Option {
	return func(o *builder) {
		o.logger = logger
	}
}

type builder struct {
	discoverer registry.Discovery
	logger     log.Logger
	state      resolver.State
	target     string
}

// NewBuilder creates a builder which is used to factory registry resolvers.
func NewBuilder(d registry.Discovery, target string, opts ...Option) Builder {
	b := &builder{
		target:     target,
		discoverer: d,
		logger:     log.DefaultLogger,
	}
	for _, o := range opts {
		o(b)
	}
	return b
}

func (b *builder) Build() error {
	i, e := b.discoverer.GetService(context.Background(), b.target)
	if e != nil {
		return fmt.Errorf("discoverer.GetService %v err: %v", b.target, e)
	}
	b.updateStates(i)

	w, err := b.discoverer.Watch(context.Background(), b.target)
	if err != nil {
		return err
	}

	go func() {
		ins, err := w.Next()
		if err != nil {
			b.updateStates(ins)
		}
	}()

	return nil
}

func (b *builder) updateStates(ins []*registry.ServiceInstance) {
	var addrs []resolver.Address
	for _, in := range ins {
		endpoint, err := parseEndpoint(in.Endpoints)
		if err != nil {
			b.logger.Print("Failed to parse discovery endpoint: %v", err)
			continue
		}
		if endpoint == "" {
			continue
		}
		addr := resolver.Address{
			ServerName: in.Name,
			Attributes: parseAttributes(in.Metadata),
			Addr:       endpoint,
		}
		addrs = append(addrs, addr)
	}
	b.state = resolver.State{Addresses: addrs}
}

func (b *builder) GetState() resolver.State {
	return b.state
}

func (b *builder) Schema() string {
	return name
}

func parseEndpoint(endpoints []string) (string, error) {
	for _, e := range endpoints {
		u, err := url.Parse(e)
		if err != nil {
			return "", err
		}
		if u.Scheme == "http" {
			return u.Host, nil
		}
	}
	//return "", fmt.Errorf("http endpoint not found: %v", endpoints)
	return "", nil
}

func parseAttributes(md map[string]string) *attributes.Attributes {
	var pairs []interface{}
	for k, v := range md {
		pairs = append(pairs, k, v)
	}
	return attributes.New(pairs...)
}

package http_resolver

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-kratos/nacos/registry"
	"github.com/miyabyte/kratosv2httpresolver"
)

func NewResolverCli(r *registry.Registry) (*kratosv2httpresolver.Client, error) {
	cli, err := kratosv2httpresolver.NewClient(context.Background())
	if err != nil {
		return nil, err
	}
	if err := cli.AddDiscovery(r, "golang-sms"); err != nil {
		return nil, errors.New(fmt.Sprintf("resolverCli.AddDiscovery: golang-sms , %v", err))
	}
	return cli, nil
}

package tests

import (
	"context"
	"github.com/go-kratos/nacos/registry"
	"github.com/miyabyte/kratosv2httpresolver"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"io/ioutil"
	"log"
	"testing"
)

func TestNacosDiscoveryHttp(t *testing.T) {
	dev := "192.168.88.164"
	local := "127.0.0.1"
	log.Println(dev, local)

	sc := []constant.ServerConfig{
		*constant.NewServerConfig(dev, 8848),
	}

	cc := constant.ClientConfig{
		NamespaceId:         "public", //namespace id
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogDir:              "/tmp/nacos/log",
		CacheDir:            "/tmp/nacos/cache",
		RotateTime:          "1h",
		MaxAge:              3,
		LogLevel:            "debug",
	}

	// a more graceful way to create naming client
	ctl, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  &cc,
			ServerConfigs: sc,
		},
	)

	if err != nil {
		log.Panic(err)
	}

	r := registry.New(ctl)

	ep := "golang-sms"

	//service
	client, err := kratosv2httpresolver.NewClient(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	err = client.AddDiscovery(r, ep)
	if err != nil {
		t.Fatal(err)
	}

	//use
	req, err := client.NewRequest("POST", ep, "/api/v1/aaa", nil)
	if err != nil {
		t.Fatal(err)
	}

	var res struct {
		Path string
	}

	httpResponse, err := client.Send(req, &res)

	defer httpResponse.Body.Close()
	data, err := ioutil.ReadAll(httpResponse.Body)
	log.Println(data, err)

	select {}
}

package eth

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/polynetwork/bridge-common/chains"
	"github.com/polynetwork/bridge-common/log"
)


func TestWs(t *testing.T) {
	sdk := new(Client)
	err := sdk.Listen("")
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan uint64)
	sdk.Subscribe(ch)
	for h := range ch {
		t.Log(h)
	}
}

func TestClients(t *testing.T) {
	log.Init(&log.LogConfig{Path: "test"})
	p, err := chains.NewChainListRpcProvider()
	if err != nil {
		t.Fatal(err)
	}
	opt := &chains.Options {
		ChainID: 0,
		NativeID: 137,
		Providers: []chains.RpcProvider{p},
	}
	c, err := WithProviders(opt)
	if err != nil {
		t.Fatal(err)
	}
	for i:=0; i < 10000000; i++ {
		h, err := c.GetLatestHeight(context.Background())
		log.Info("Tick", "height", h, "err", err)
	}
}

type A struct {
	Value int `json:"value"`
}

func TestJson(t *testing.T) {
	f := func (s string, a interface{}) {
		err := json.Unmarshal([]byte(s), &a)
		if err != nil {
			t.Fatal(err)
		}
	}
	{
		s := `{"value": 3}`
		r := new(Raw)
		f(s, r)
		var a A
		f(string([]byte(*r)), &a)
		c := make(chan int, 1)
		close(c)
		c <- 1
		t.Logf("%s %+v", *r, a)
	}
	{
		var a A
		f(`{"value": 3}`, a)
		t.Logf("%+v", a)
	}
	{
		var a *A
		f("{}", a)
		t.Logf("%+v", *a)
	}
}
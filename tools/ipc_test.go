package tools

import (
	"testing"

	"github.com/polynetwork/bridge-common/log"
)


type apiHandler struct {
	IPCBase
}

func (h *apiHandler) Get(i int, s string) (v int, err error) {
	ret, err := h.HandleCall("get", &v, i, s)
	if ret {
		return v, err
	}
		
	log.Info("Serving for client", "i", i, "s", s)
	return i, nil
}

func TestIPC(t *testing.T) {
	url := "../ipc.test"
	h := new(apiHandler)
	h.AsServer("test")
	srv := NewIPCServer(url)
	srv.Start([]API{{
		Namespace: h.Namespace(),
		Service: h,
	}})

	c := new(apiHandler)
	c.AsClient("test", url)
	v, err := c.Get(3, "test")
	t.Log(v, err)
}
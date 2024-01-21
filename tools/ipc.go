package tools

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/polynetwork/bridge-common/log"
)

type IpcServer struct {
	endpoint string

	mu       sync.Mutex
	listener net.Listener
	srv      *rpc.Server
}

func NewIPCServer(endpoint string) *IpcServer {
	return &IpcServer{endpoint: endpoint}
}

type API = rpc.API
// Start starts the httpServer's http.Server
func (is *IpcServer) Start(apis []rpc.API) error {
	is.mu.Lock()
	defer is.mu.Unlock()

	if is.listener != nil {
		return nil // already running
	}
	listener, srv, err := rpc.StartIPCEndpoint(is.endpoint, apis)
	if err != nil {
		log.Warn("IPC opening failed", "url", is.endpoint, "error", err)
		return err
	}
	log.Info("IPC endpoint opened", "url", is.endpoint)
	is.listener, is.srv = listener, srv
	return nil
}

func (is *IpcServer) Stop() error {
	is.mu.Lock()
	defer is.mu.Unlock()

	if is.listener == nil {
		return nil // not running
	}
	err := is.listener.Close()
	is.srv.Stop()
	is.listener, is.srv = nil, nil
	log.Info("IPC endpoint closed", "url", is.endpoint)
	return err
}


type IPCBase struct {
	namespace string
	isClient bool
	url string
	c atomic.Value
}

func (ib *IPCBase) Namespace() string {
	return ib.namespace
}

func (ib *IPCBase) AsClient(ns, url string) {
	ib.namespace = ns
	ib.url = url
	ib.isClient = true
}

func (ib *IPCBase) AsServer(ns string) {
	ib.namespace = ns
	ib.isClient = false
}

func (ib *IPCBase) HandleCall(method string, res interface{}, args ...interface{}) (bool, error) {
	if ib.isClient {
		c, err := ib.GetClient()
		if err != nil {
			return true, err
		}
		return true, c.CallContext(context.Background(), res, fmt.Sprintf("%s_%s", ib.namespace, method), args...)
	}
	return false, nil
}

func (ib *IPCBase) GetClient() (*rpc.Client, error) {
	_c := ib.c.Load()
	if _c == nil {
		c, err := rpc.DialIPC(context.Background(), ib.url)
		if err != nil {
			return nil, err
		}
		ib.c.Store(c)
		return c, nil
	}
	return _c.(*rpc.Client), nil
}

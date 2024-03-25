/*
 * Copyright (C) 2021 The poly network Authors
 * This file is part of The poly network library.
 *
 * The  poly network  is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The  poly network  is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 * You should have received a copy of the GNU Lesser General Public License
 * along with The poly network .  If not, see <http://www.gnu.org/licenses/>.
 */

package wallet

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/polynetwork/bridge-common/chains/eth"
	"github.com/polynetwork/bridge-common/log"
)

type NonceProvider interface {
	Acquire() (uint64, error)
	Update(bool) error
	Fill(uint64, bool)
}

func NewRemoteNonceProvider(sdk eth.NodeProvider, address common.Address) *RemoteNonceProvider {
	return &RemoteNonceProvider{sdk, address, false}
}

type RemoteNonceProvider struct {
	sdk     eth.NodeProvider
	address common.Address
	UsePending bool
}

func (p *RemoteNonceProvider) Acquire() (uint64, error) {
	if p.UsePending {
		p.sdk.Node().PendingNonceAt(context.Background(), p.address)
	}
	return p.sdk.Node().NonceAt(context.Background(), p.address, nil)
}

func (p *RemoteNonceProvider) Update(success bool) error {
	return nil
}

func (p *RemoteNonceProvider) Fill(nonce uint64, force bool) {
	return
}

type DummyNonceProvider uint64
func (p *DummyNonceProvider) Acquire() (uint64, error) { return uint64(*p), nil }
func (p *DummyNonceProvider) Update(_success bool) (err error) { return nil }
func (p *DummyNonceProvider) Fill(nonce uint64, force bool) {
	if force {
		*p = DummyNonceProvider(nonce)
	}
	return
}


func WrapNonce(nonce NonceProvider) (DummyNonceProvider, error) {
	n, err := nonce.Acquire()
	if err != nil {
		return DummyNonceProvider(0), err
	}
	return DummyNonceProvider(n), nil
}

func NewCacheNonceProvider(sdk eth.NodeProvider, address common.Address) *CacheNonceProvider {
	p := &CacheNonceProvider{sdk: sdk, address: address, sig: make(chan struct{}, 1), interval: time.Minute * 2, nonce: new(atomic.Pointer[uint64])}
	p.Update(true)
	go p.loop()
	return p
}

type CacheNonceProvider struct {
	sdk     eth.NodeProvider
	address common.Address
	nonce *atomic.Pointer[uint64]
	interval time.Duration
	sig chan struct{}
	UsePending bool
}

func (p *CacheNonceProvider) Acquire() (uint64, error) {
	nonce := p.nonce.Swap(nil)
	if nonce != nil {
		return *nonce, nil
	}
	if p.UsePending {
		return p.sdk.Node().PendingNonceAt(context.Background(), p.address)
	}
	return p.sdk.Node().NonceAt(context.Background(), p.address, nil)
}

func (p *CacheNonceProvider) loop() {
	for {
		select {
		case <- p.sig:
		case <-time.After(p.interval):
		}
		p.update()
	}
}

func(p *CacheNonceProvider) SetInterval(interval time.Duration) {
	p.interval = interval
}

func (p *CacheNonceProvider) Update(_success bool) (err error) {
	select {
	case p.sig <- struct{}{}:
	default:
	}
	return
}

func (p *CacheNonceProvider) update() (err error) {
	var nonce uint64
	if p.UsePending {
		nonce, err = p.sdk.Node().PendingNonceAt(context.Background(), p.address)
	} else {
		nonce, err = p.sdk.Node().NonceAt(context.Background(), p.address, nil)
	}
	if err != nil {
		log.Error("Failed to fetch nonce for account", "err", err)
	} else {
		p.nonce.Store(&nonce)
		log.Info("Updated account nonce cache", "account", p.address, "nonce", nonce)
	}
	return
}

func (p *CacheNonceProvider) Fill(nonce uint64, force bool) {
	if force {
		p.nonce.Store(&nonce)
	} else {
		p.nonce.CompareAndSwap(nil, &nonce)
	}
	return
}
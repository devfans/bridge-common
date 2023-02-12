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

package chains

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/polynetwork/bridge-common/log"
)

type Options struct {
	ChainID  uint64
	Nodes    []string
	Interval time.Duration
	MaxGap   uint64
}

func (o *Options) Key() string {
	return fmt.Sprintf("SDK:%v:%s", o.ChainID, strings.Join(o.Nodes, ":"))
}

type SDK interface {
	GetLatestHeight() (uint64, error)
	Address() string
}

type Nodes interface {
	Height() uint64
	WaitTillHeight(context.Context, uint64, time.Duration) (uint64, bool)
	Available() bool
	Node() SDK
}

type ChainSDK struct {
	sdk      SDK
	ChainID  uint64
	nodes    []SDK
	state    []bool
	index    int
	cursor   int
	status   int // SDK nodes status: 1. available, 0. all down
	height   uint64
	interval time.Duration
	maxGap   uint64
	sync.RWMutex
	exit chan struct{}
}

func (s *ChainSDK) Key() string {
	nodes := make([]string, len(s.nodes))
	for i, node := range s.nodes {
		nodes[i] = node.Address()
	}
	return fmt.Sprintf("SDK:%v:%s", s.ChainID, strings.Join(nodes, ":"))
}

func (s *ChainSDK) Nodes() (nodes []int) {
	s.RLock()
	defer s.RUnlock()
	nodes = append(nodes, s.index)
	for idx, active := range s.state {
		if active && idx != s.index {
			nodes = append(nodes, idx)
		}
	}
	return
}

func (s *ChainSDK) Height() uint64 {
	s.RLock()
	defer s.RUnlock()
	return s.height
}

func (s *ChainSDK) WaitTillHeight(ctx context.Context, height uint64, interval time.Duration) (uint64, bool) {
	if interval == 0 {
		interval = s.interval
	}
	for {
		h, err := s.Node().GetLatestHeight()
		if err != nil {
			log.Error("Failed to get chain latest height err ", "chain", s.ChainID, "err", err)
		} else if h >= height {
			return h, true
		}
		select {
		case <-ctx.Done():
			return h, false
		case <-time.After(interval):
		}
	}
}

func (s *ChainSDK) updateSelection() {
	var height uint64
	var sdk SDK
	var index int
	var perf, best time.Duration
	state := make([]uint64, len(s.nodes))
	timer := time.After(time.Second * 10)
	ch := make(chan [2]uint64, len(s.nodes))
	target := time.Now()
	for i, s := range s.nodes {
		go func (index int, node SDK) {
			h, err := node.GetLatestHeight()
			if err != nil {
				log.Error("Ping node error", "url", node.Address(), "err", err)
			}
			ch <- [2]uint64{uint64(index), h}
		} (i, s)
	}

	count := len(s.nodes)
	LOOP:
	for {
		select {
		case <- timer:
			break LOOP
		case res := <- ch:
			elapse := time.Since(target)
			if res[1] > 0 && best == 0 {
				best = elapse
			}
			count--
			state[int(res[0])] = res[1]
			if res[1] > height {
				index = int(res[0])
				height = res[1]
				sdk = s.nodes[index]
				perf = elapse
			}
			if count == 0 {
				break LOOP
			}
		}
	}

	status := 1
	if sdk == nil {
		status = 0
		log.Warn("Temp unavailabitlity for all node", "chain", s.ChainID)
		if len(s.nodes) > 0 {
			sdk = s.nodes[0]
		}
	}
	var changed bool
	s.Lock()
	s.sdk = sdk
	s.status = status
	s.height = height
	changed = s.index != index
	s.index = index
	for i, h := range state {
		s.state[i] = h >= height-s.maxGap
	}
	s.Unlock()
	if changed && status == 1 {
		log.Info("Changing best node", "chain_id", s.ChainID, "height", height, "elapse", perf, "delta", perf-best, "addr", sdk.Address())
	}
}

func (s *ChainSDK) Available() bool {
	s.RLock()
	defer s.RUnlock()
	return s.status > 0
}

func (s *ChainSDK) Index() int {
	s.RLock()
	defer s.RUnlock()
	return s.index
}

func (s *ChainSDK) Select() int {
	s.Lock()
	defer s.Unlock()
	cursor := s.cursor % len(s.nodes)
	c := (cursor + 1) % len(s.nodes)
	for c != cursor {
		if s.state[c] {
			break
		}
		c = (c + 1) % len(s.nodes)
	}
	s.cursor = c
	return c
}

func (s *ChainSDK) Node() SDK {
	s.RLock()
	defer s.RUnlock()
	return s.sdk
}

func (s *ChainSDK) Stop() {
	close(s.exit)
}

func (s *ChainSDK) monitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		select {
		case <-s.exit:
			log.Info("Exiting nodes monitoring", "chainID", s.ChainID)
			return
		default:
			s.updateSelection()
		}
	}
}

func (s *ChainSDK) Init() error {
	log.Info("Initializing chain sdk", "chainID", s.ChainID)
	s.updateSelection()
	if !s.Available() {
		return fmt.Errorf("all the nodes are unavailable for chain %v", s.ChainID)
	}
	return nil
}

func New(chainID uint64, urls []string, interval time.Duration, maxGap uint64, f func(string) SDK) (sdk *ChainSDK, err error) {
	nodes := make([]SDK, len(urls))
	for i, url := range urls {
		nodes[i] = f(url)
	}
	return NewChainSDK(chainID, nodes, interval, maxGap)
}

func NewChainSDK(chainID uint64, nodes []SDK, interval time.Duration, maxGap uint64) (sdk *ChainSDK, err error) {
	var s SDK
	sdk = &ChainSDK{
		sdk:      s,
		ChainID:  chainID,
		nodes:    nodes,
		interval: interval,
		maxGap:   maxGap,
		state:    make([]bool, len(nodes)),
		exit:     make(chan struct{}),
	}
	err = sdk.Init()
	if err == nil {
		go sdk.monitor(interval)
	}
	return
}

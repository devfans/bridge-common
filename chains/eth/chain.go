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

package eth

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/polynetwork/bridge-common/log"
	"github.com/polynetwork/bridge-common/util"
)

type Node interface {
	ChainID(context.Context) (*big.Int, error)
	LatestHeight(context.Context) (uint64, error)
	Address() string
}

type Nodes interface {
	Height() uint64
	WaitTillHeight(context.Context, uint64, time.Duration) (uint64, bool)
	Available() bool
	Node() Node
}

type NodeState byte

const (
	NodeUninitialized NodeState = 0
	NodeHidden        NodeState = 1

	NodeRateLimited   NodeState = 10
	NodeUnavailable   NodeState = 15
	NodeAvailable     NodeState = 20
)

type ChainSDK struct {
	node     Node
	ChainID  uint64
	NativeID uint64
	nodes    []Node
	state    []NodeState
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
	for idx, state := range s.state {
		if state == NodeAvailable && idx != s.index {
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
		h, err := s.Node().LatestHeight(ctx)
		if err != nil {
			log.Error("Failed to get chain latest height err ", "chain", s.ChainID, "err", util.CompactError(err, util.RateLimitErrors))
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

func (s *ChainSDK) dial(indices []int) {
	for _, i := range indices {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second * 3)
		chainID, err := s.nodes[i].ChainID(ctx)
		if err != nil || chainID == nil || chainID.Uint64() != s.NativeID {
			log.Error("Failed to verify chain id", "chainID", chainID, "expected", s.NativeID, "addr", s.nodes[i].Address(), "err", util.CompactError(err, util.RateLimitErrors))
		} else {
			s.Lock()
			switch s.state[i] {
			case NodeRateLimited, NodeUninitialized:
				s.state[i] = NodeUnavailable
			}
			s.Unlock()
			log.Info("Validated node", "chainID", s.NativeID, "addr", s.nodes[i].Address())
		}
		cancel()
	}
}

func (s *ChainSDK) startDialing() {
	{
		var indices []int
		for i := range s.nodes {
			indices = append(indices, i)
		}
		s.dial(indices)
	}
	go func() {
		schedule := make(map[int]int64)
		ticker := time.NewTicker(time.Second * 2)
		defer ticker.Stop()
		for {
			{
				now := time.Now().Unix()
				s.RLock()
				for i, status := range s.state {
					_, included := schedule[i]
					switch status {
					case NodeUninitialized, NodeRateLimited:
						if !included {
							schedule[i] = now + 120
						}
					case NodeHidden:
						if included {
							delete(schedule, i)
						}
					}
				}
				s.RUnlock()
			}
			select {
			case <- s.exit:
				return
			case <- ticker.C:
			}
			now := time.Now().Unix()
			var indices []int
			for i, line := range schedule {
				if line <= now {
					indices = append(indices, i)
					delete(schedule, i)
				}
			}
			s.dial(indices)
		}
	} ()
}

func (s *ChainSDK) UpdateNodeStatus(index int, status NodeState) (available bool) {
	switch status {
	case NodeAvailable:
		return
	}

	available = true
	s.Lock()
	defer s.Unlock()
	addr := s.nodes[index].Address()
	s.state[index] = status

	if s.index == index  {
		for i, state := range s.state {
			if i != index && state == NodeAvailable {
				s.index = i
				s.node = s.nodes[i]
				break
			}
		}
	}
	if s.index == index {
		s.status = 0
		available = false
	}
	best := s.node.Address()
	log.Warn("Node status updated", "status", status, "addr", addr, "best", best)
	return
}

func (s *ChainSDK) filterNode(f func(NodeState) bool) ([]Node, int) {
	list := make([]Node, len(s.nodes))
	total := 0
	s.RLock()
	for i, node := range s.nodes {
		if f(s.state[i]) {
			list[i] = node
			total++
		}
	}
	s.RUnlock()
	return list, total
}

func (s *ChainSDK) updateSelection() {
	var height uint64
	var node Node
	var index int
	var perf, best time.Duration

	candidates, total := s.filterNode(func(status NodeState) bool { return status >= NodeUnavailable })
	state := make([]uint64, len(candidates))
	ch := make(chan [2]uint64, total)
	target := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 5)
	defer cancel()
	for i, s := range candidates {
		if s == nil { continue }
		go func (index int, node Node) {
			h, err := node.LatestHeight(ctx)
			if err != nil {
				log.Error("Ping node error", "url", node.Address(), "err", util.CompactError(err, util.RateLimitErrors))
			}
			ch <- [2]uint64{uint64(index), h}
		} (i, s)
	}

	LOOP:
	for {
		select {
		case <-s.exit:
			return
		case <- ctx.Done():
			break LOOP
		case res := <- ch:
			elapse := time.Since(target)
			if res[1] > 0 && best == 0 {
				best = elapse
			}
			total--
			state[int(res[0])] = res[1]
			if res[1] > height {
				index = int(res[0])
				height = res[1]
				node = s.nodes[index]
				perf = elapse
			}
			if total == 0 {
				break LOOP
			}
		}
	}

	status := 1
	if node == nil {
		status = 0
		log.Warn("Temp unavailabitlity for all node", "chain", s.ChainID)
		if len(s.nodes) > 0 {
			node = s.nodes[0]
		}
	}
	var changed bool

	s.Lock()
	s.node = node
	s.status = status
	s.height = height
	changed = s.index != index
	s.index = index
	for i, h := range state {
		nodeStatus := NodeAvailable
		if h + s.maxGap < height {
			if h > 0 {
				nodeStatus = NodeUnavailable
			} else {
				nodeStatus = NodeRateLimited
			}
		}
		switch s.state[i] {
		case NodeAvailable, NodeUnavailable, NodeRateLimited:
			s.state[i] = nodeStatus
		}
	}
	s.Unlock()

	if changed && status == 1 {
		log.Info("Changing best node", "chain_id", s.ChainID, "height", height, "elapse", perf, "delta", perf-best, "addr", node.Address())
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

func (s *ChainSDK) Best() (int, bool) {
	s.RLock()
	defer s.RUnlock()
	return s.index, s.state[s.index] == NodeAvailable
}

func (s *ChainSDK) Next(cur int) (int, bool) {
	s.RLock()
	defer s.RUnlock()
	cur = cur % len(s.state)
	c := (cur + 1) % len(s.state)
	for c != cur {
		if s.state[c] == NodeAvailable {
			return c, true
		}
		c = (c + 1) % len(s.state)
	}
	return c, s.state[c] == NodeAvailable
}

func (s *ChainSDK) Select() int {
	s.Lock()
	defer s.Unlock()
	cursor := s.cursor % len(s.nodes)
	c := (cursor + 1) % len(s.nodes)
	for c != cursor {
		if s.state[c] == NodeAvailable {
			break
		}
		c = (c + 1) % len(s.nodes)
	}
	s.cursor = c
	return c
}

func (s *ChainSDK) Node() Node {
	s.RLock()
	defer s.RUnlock()
	return s.node
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
	log.Info("Initializing chain sdk", "chainID", s.ChainID, "size", len(s.nodes))
	s.startDialing()
	s.updateSelection()
	if !s.Available() {
		s.Stop()
		return fmt.Errorf("all the nodes are unavailable for chain %v", s.ChainID)
	} else {
		go s.monitor(s.interval)
	}
	return nil
}

func NewChainSDK(chainID, nativeID uint64, nodes []Node, interval time.Duration, maxGap uint64) (sdk *ChainSDK, err error) {
	sdk = &ChainSDK{
		ChainID:  chainID,
		NativeID: nativeID,
		nodes:    nodes,
		interval: interval,
		maxGap:   maxGap,
		state:    make([]NodeState, len(nodes)),
		exit:     make(chan struct{}),
	}
	err = sdk.Init()
	return
}

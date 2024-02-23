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

package base

import "fmt"

var (
	PRICE_PRECISION = int64(100000000)
	FEE_PRECISION   = int64(100000000)
)

var (
	MARKET_COINMARKETCAP = "coinmarketcap"
	MARKET_BINANCE       = "binance"
	MARKET_HUOBI         = "huobi"
	MARKET_SELF          = "self"
)

const (
	STATE_FINISHED = iota
	STATE_PENDDING
	STATE_SOURCE_DONE
	STATE_SOURCE_CONFIRMED
	STATE_POLY_CONFIRMED
	STATE_DESTINATION_DONE

	STATE_WAIT = 100
	STATE_SKIP = 101
)

func GetStateName(state int) string {
	switch state {
	case STATE_FINISHED:
		return "Finished"
	case STATE_PENDDING:
		return "Pending"
	case STATE_SOURCE_DONE:
		return "SrcDone"
	case STATE_SOURCE_CONFIRMED:
		return "SrcConfirmed"
	case STATE_POLY_CONFIRMED:
		return "PolyConfirmed"
	case STATE_DESTINATION_DONE:
		return "DestDone"
	case STATE_WAIT:
		return "WAIT"
	case STATE_SKIP:
		return "SKIP"
	default:
		return fmt.Sprintf("Unknown(%d)", state)
	}
}

type ChainID uint64

func (id ChainID) String() string { return ChainName(uint64(id)) }

var chainNames = map[uint64]string{
	POLY:       "Poly",
	ETH:        "Ethereum",
	RINKEBY:    "Ethereum-Rinkeby",
	ONT:        "Ontology",
	NEO:        "Neo",
	BSC:        "Bsc",
	HECO:       "Heco",
	O3:         "O3",
	OK:         "OK",
	MATIC:      "Polygon",
	HEIMDALL:   "Heimdall",
	NEO3:       "NEO3",
	SWITCHEO:   "Switcheo",
	PLT:        "Palette",
	PLT2:       "Palette2",
	ARBITRUM:   "Arbitrum",
	ZILLIQA:    "Zilliqa",
	XDAI:       "Xdai",
	OPTIMISM:   "Optimism",
	FANTOM:     "Fantom",
	METIS:      "Metis",
	AVA:        "Avalanche",
	BOBA:       "Boba",
	PIXIE:      "Pixie",
	OASIS:      "Oasis",
	HSC:        "Hsc",
	HARMONY:    "Harmony",
	BYTOM:      "Bytom",
	BCSPALETTE: "BCS Palette",
	KCC:        "KCC",
	STARCOIN:   "Starcoin",
	BINANCE:    "Binance",
	OKX:        "Okx",
	GATE:       "Gate",
	KUCOIN:     "Kucoin",
	ONTEVM:     "ONTEVM",
	MILKO:      "Milkomeda",
	CUBE:       "Cube",
	KAVA:       "Kava",
	ZKSYNC:     "zkSync",
	CELO:       "Celo",
	CLOVER:     "CLV P-Chain",
	CONFLUX:    "Conflux",
	ASTAR:      "Astar",
	APTOS:      "Aptos",
	BRISE:      "Bitgert",
	CORE:       "CoreDao",
	XDC:        "XDC",
	POLYGONZK:  "PolygonZK",
	FEVM:       "FIL",
	ETHW:       "ETHW",
	ETHF:       "ETHF",
	ZKFAIR:     "ZKFair",
	MERLIN:     "Merlin",
	B2:         "B2",
	MANTLE:     "Mantle",
	SCROLL:     "Sroll",
	ZORA:       "Zora",
	MANTA:      "Manta",
	STARK:      "Stark",
	ZETA:       "Zeta",
}

func ChainName(id uint64) string {
	name, ok := chainNames[id]
	if ok {
		return name
	}
	return fmt.Sprintf("Unknown(%d)", id)
}

func BlocksToSkip(chainId uint64) uint64 {
	switch chainId {
	case MATIC:
		return 120
	case ETH:
		return 8
	case BSC, HECO, HSC, BYTOM, KCC:
		return 20
	case O3:
		return 8
	case PLT, PLT2, BCSPALETTE:
		return 5
	case ONT:
		return 0
	case PIXIE:
		return 2
	case STARCOIN:
		return 70
	default:
		return 1
	}
}

func BlocksToWait(chainId uint64) uint64 {
	switch chainId {
	case ETH:
		return 12
	case BSC, HECO, HSC, BYTOM, KCC:
		return 21
	case ONT, NEO, NEO3, OK, SWITCHEO:
		return 1
	case HARMONY, ASTAR:
		return 2
	case PLT, PLT2, BCSPALETTE:
		return 4
	case O3:
		return 12
	case MATIC:
		return 128
	case PIXIE:
		return 3
	case STARCOIN:
		return 72
	default:
		return 100000000
	}
}

func SameAsETH(chainId uint64) bool {
	for _, chain := range ETH_CHAINS {
		if chain == chainId {
			return true
		}
	}
	return false
}

func UseDynamicFeeTx(chainId uint64) bool {
	switch chainId {
	case ETH, MATIC, FEVM, ZETA:
		return true
	default:
		return false
	}
}

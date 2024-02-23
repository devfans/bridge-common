//go:build mainnet
// +build mainnet

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

const (
	POLY     uint64 = 0
	BTC      uint64 = 1
	ETH      uint64 = 2
	ONT      uint64 = 3
	NEO      uint64 = 4
	SWITCHEO uint64 = 5
	BSC      uint64 = 6
	HECO     uint64 = 7
	PLT      uint64 = 8
	O3       uint64 = 10
	OK       uint64 = 12
	NEO3     uint64 = 14
	HEIMDALL uint64 = 15
	MATIC    uint64 = 17
	ZILLIQA  uint64 = 18
	ARBITRUM uint64 = 19
	XDAI     uint64 = 20
	AVA      uint64 = 21
	FANTOM   uint64 = 22
	OPTIMISM uint64 = 23
	METIS    uint64 = 24
	BOBA     uint64 = 25
	OASIS    uint64 = 26
	HARMONY  uint64 = 27
	HSC      uint64 = 28
	BYTOM    uint64 = 29
	KCC      uint64 = 30
	STARCOIN uint64 = 31
	KAVA     uint64 = 32
	MILKO    uint64 = 34
	CUBE     uint64 = 35
	CELO     uint64 = 36
	CLOVER   uint64 = 37
	CONFLUX  uint64 = 38
	ASTAR    uint64 = 40
	APTOS    uint64 = 41
	BRISE    uint64 = 42
	DEXIT    uint64 = 43
	CLOUDTX  uint64 = 44
	ZKSYNC   uint64 = 45

	// Invalid chain IDs
	RINKEBY    uint64 = 1000000
	PIXIE      uint64 = 2000000
	BCSPALETTE uint64 = 1001001
	ONTEVM     uint64 = 1001333
	FLOW       uint64 = 1000444
	PLT2       uint64 = 1000108

	// CEX
	BINANCE uint64 = 9001
	OKX     uint64 = 9002
	GATE    uint64 = 9003
	KUCOIN  uint64 = 9004

	// Others
	CORE   uint64 = 101
	KLAY   uint64 = 102
	CANTO  uint64 = 103
	ZKFAIR uint64 = 104
	MERLIN uint64 = 105
	B2     uint64 = 106
	MANTLE uint64 = 107
	SCROLL uint64 = 108
	ZORA   uint64 = 109
	MANTA  uint64 = 110
	STARK  uint64 = 111
	ZETA   uint64 = 112

	XDC       uint64 = 201
	POLYGONZK uint64 = 202
	FEVM      uint64 = 203
	BASE      uint64 = 204
	ETHW      uint64 = 205
	ETHF      uint64 = 206

	ENV = "mainnet"
)

var CHAINS = []uint64{
	POLY, ETH, BSC, HECO, OK, ONT, NEO, NEO3, HEIMDALL, MATIC, SWITCHEO, O3, PLT, ARBITRUM, XDAI, OPTIMISM, FANTOM, AVA,
	METIS, BOBA, PIXIE, OASIS, HSC, HARMONY, BYTOM, BCSPALETTE, STARCOIN, ONTEVM, KCC, MILKO, CUBE, KAVA, FLOW, ZKSYNC,
	CELO, CLOVER, CONFLUX, ASTAR, APTOS, BRISE,
}

var ETH_CHAINS = []uint64{
	ETH, BSC, HECO, OK, MATIC, O3, PLT, ARBITRUM, XDAI, OPTIMISM, FANTOM, AVA, METIS, BOBA, PIXIE, OASIS, HSC, HARMONY,
	BYTOM, BCSPALETTE, KCC, ONTEVM, MILKO, CUBE, KAVA, ZKSYNC, CELO, CLOVER, CONFLUX, ASTAR, BRISE, CORE, KLAY, CANTO, XDC,
	DEXIT, CLOUDTX, ZKSYNC, POLYGONZK, FEVM, ETHW, ETHF, ZKFAIR, MERLIN, B2, MANTLE, SCROLL, ZORA, MANTA, STARK, ZETA,
}

func NativeID(uint64 uint64) int64 {
	switch uint64 {
	case ETH:
		return 1
	case BSC:
		return 56
	case CORE:
		return 1116
	case METIS:
		return 1088
	case MATIC:
		return 137
	case POLYGONZK:
		return 1101
	case ARBITRUM:
		return 42161
	case OPTIMISM:
		return 10
	case HECO:
		return 128
	case OK:
		return 66
	case FANTOM:
		return 250
	case XDAI:
		return 100
	case AVA:
		return 43114
	case KCC:
		return 321
	case CONFLUX:
		return 1030
	case ZKSYNC:
		return 324
	case FEVM:
		return 314
	case XDC:
		return 50
	case BASE:
		return 8453
	case BRISE:
		return 32520
	case CUBE:
		return 1818
	case ETHW:
		return 10001
	case ETHF:
		return 513100
	case ZKFAIR:
		return 42766
	case MERLIN:
		return 4200
	case B2:
		return 0
	case MANTLE:
		return 5000
	case SCROLL:
		return 534352
	case ZORA:
		return 7777777
	case MANTA:
		return 169
	case STARK:
		return 0x534e5f4d41494e
	case ZETA:
		return 7000
	default:
		panic(fmt.Sprintf("no native id found for %v", uint64))
	}
}

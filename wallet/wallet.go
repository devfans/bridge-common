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
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/polynetwork/bridge-common/base"
	"github.com/polynetwork/bridge-common/chains/eth"
	"github.com/polynetwork/bridge-common/log"
	"github.com/polynetwork/bridge-common/util"
)

type Config struct {
	ReadFile func(string) ([]byte, error) `json:"-"`

	NativeID          int64
	KeyStoreProviders []*KeyStoreProviderConfig
	KeyProviders      []string
	Nodes             []string

	// NEO wallet
	Path     string
	Password string
	SysFee   float64
	NetFee   float64

	// ONT wallet
	GasPrice uint64
	GasLimit uint64

	// FLOW/Aptos wallet
	Address    string
	PrivateKey string
}

type IWallet interface {
	Init() error
	Send(addr common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, gasPriceX *big.Float, data []byte) (hash string, err error)
	SendWithAccount(account accounts.Account, addr common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, gasPriceX *big.Float, data []byte) (hash string, err error)
	EstimateWithAccount(account accounts.Account, addr common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, gasPriceX *big.Float, data []byte) (hash string, err error)
	Accounts() []accounts.Account
	GetAccount(account accounts.Account) (provider Provider, nonces NonceProvider)
	Select() (accounts.Account, Provider, NonceProvider)
	GetBalance(common.Address) (*big.Int, error)
	SetNonceProvider(account accounts.Account, provider NonceProvider) (err error)
	GetNonceProvider(account accounts.Account) NonceProvider
	SetCacheNonces(accounts ...accounts.Account) (err error)
	SendLight(ctx context.Context, provider Provider, nonces NonceProvider, account accounts.Account, addr common.Address, data []byte, amount *big.Int, price GasPriceOracle, limit uint64) (hash string, err error)
	EstimateGasWithAccount(account accounts.Account, addr common.Address, amount *big.Int, data []byte) (gasPrice *big.Int, gasLimit uint64, err error)
	SendWithMaxLimit(chainId uint64, account accounts.Account, addr common.Address, amount *big.Int, maxLimit *big.Int, gasPrice *big.Int, gasPriceX *big.Float, data []byte) (hash string, err error)
	QuickSendWithAccount(account accounts.Account, addr common.Address, amount *big.Int, gasLimit uint64, price GasPriceOracle, gasPriceX *big.Float, data []byte) (hash string, err error)
	QuickSendWithMaxLimit(chainId uint64, account accounts.Account, addr common.Address, amount *big.Int, maxLimit *big.Int, price GasPriceOracle, gasPriceX *big.Float, data []byte) (hash string, err error)
}

type Wallet struct {
	sync.RWMutex
	nativeID  int64
	providers map[accounts.Account]Provider
	provider  Provider                           // active account provider
	account   accounts.Account                   // active account
	nonces    map[accounts.Account]NonceProvider // account nonces
	sdk       eth.NodeProvider
	accounts  []accounts.Account
	cursor    int
	config    *Config
	Broadcast bool
}

type Provider interface {
	SignTx(account accounts.Account, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error)
	Init(accounts.Account) error
	Accounts() []accounts.Account
	SignHash(accounts.Account, []byte) ([]byte, error)
}

func New(config *Config, sdk eth.NodeProvider) *Wallet {
	w := &Wallet{
		config: config, sdk: sdk, nativeID: config.NativeID, providers: map[accounts.Account]Provider{},
		nonces:   map[accounts.Account]NonceProvider{},
		accounts: []accounts.Account{},
	}
	for _, c := range config.KeyStoreProviders {
		w.AddProvider(NewKeyStoreProvider(c))
	}
	return w
}

func (w *Wallet) GetBalance(address common.Address) (balance *big.Int, err error) {
	return w.sdk.Node().BalanceAt(context.Background(), address, nil)
}

func (w *Wallet) AddProvider(p Provider) {
	w.Lock()
	defer w.Unlock()

	for _, a := range p.Accounts() {
		w.providers[a] = p
	}
}

func (w *Wallet) Accounts() []accounts.Account {
	w.RLock()
	defer w.RUnlock()
	return w.accounts
}

// Should be called after Init and before uses
func (w *Wallet) SetCacheNonces(accounts ...accounts.Account) (err error) {
	w.Lock()
	defer w.Unlock()
	for _, account := range accounts {
		_, ok := w.nonces[account]
		if !ok {
			return fmt.Errorf("Missing account in wallet %s", account.Address)
		}
		w.nonces[account] = NewCacheNonceProvider(w.sdk, account.Address)
	}
	return
}

func (w *Wallet) GetNonceProvider(account accounts.Account) NonceProvider {
	w.RLock()
	defer w.RUnlock()
	return w.nonces[account]
}

func (w *Wallet) SetNonceProvider(account accounts.Account, provider NonceProvider) (err error) {
	w.Lock()
	defer w.Unlock()
	_, ok := w.nonces[account]
	if !ok {
		return fmt.Errorf("Missing account in wallet %s", account.Address)
	}
	w.nonces[account] = provider
	return
}

func (w *Wallet) Init() (err error) {
	{
		for _, k := range w.config.KeyProviders {
			p, err := NewKeyProvider(k)
			if err != nil {
				return fmt.Errorf("create KeyProvider failure, %w", err)
			}
			w.AddProvider(p)
		}
	}
	w.updateAccounts()
	w.Lock()
	defer w.Unlock()
	for a, p := range w.providers {
		err = p.Init(a)
		if err != nil {
			return
		}
	}
	if len(w.accounts) == 0 {
		return fmt.Errorf("No valid account provided")
	}
	w.account = w.accounts[0]
	w.provider = w.providers[w.account]
	if w.sdk != nil {
		w.VerifyChainId()
	}
	return
}

func (w *Wallet) VerifyChainId() {
	id := base.NativeID(w.sdk.ChainID())
	if id == 0 || (w.nativeID != 0 && w.nativeID != id) {
		util.Fatal("ChainID does not match specified %v, on chain: %v", w.nativeID, id)
	}
	w.nativeID = id
}

func (w *Wallet) GetAccount(account accounts.Account) (provider Provider, nonces NonceProvider) {
	w.RLock()
	defer w.RUnlock()
	return w.providers[account], w.nonces[account]
}

func (w *Wallet) Send(addr common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, gasPriceX *big.Float, data []byte) (hash string, err error) {
	account, _, _ := w.Select()
	return w.sendWithAccount(false, true, account, addr, amount, gasLimit, gasPrice, gasPriceX, data)
}

func (w *Wallet) SendWithAccount(account accounts.Account, addr common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, gasPriceX *big.Float, data []byte) (hash string, err error) {
	return w.sendWithAccount(false, true, account, addr, amount, gasLimit, gasPrice, gasPriceX, data)
}

func (w *Wallet) QuickSendWithAccount(account accounts.Account, addr common.Address, amount *big.Int, gasLimit uint64, price GasPriceOracle, gasPriceX *big.Float, data []byte) (hash string, err error) {
	return w.sendWithAccount(false, true, account, addr, amount, gasLimit, price.Price(), gasPriceX, data)
}

func (w *Wallet) SendWithAccountFix(account accounts.Account, addr common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, gasPriceX *big.Float, data []byte) (hash string, err error) {
	return w.sendWithAccount(false, false, account, addr, amount, gasLimit, gasPrice, gasPriceX, data)
}

func (w *Wallet) EstimateWithAccount(account accounts.Account, addr common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, gasPriceX *big.Float, data []byte) (hash string, err error) {
	return w.sendWithAccount(true, true, account, addr, amount, gasLimit, gasPrice, gasPriceX, data)
}

func (w *Wallet) sendWithAccount(dry bool, estimateWithGas bool, account accounts.Account, addr common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, gasPriceX *big.Float, data []byte) (hash string, err error) {
	if gasPrice == nil || gasPrice.Sign() <= 0 {
		gasPrice, err = w.GasPrice()
		if err != nil {
			err = fmt.Errorf("Get gas price error %v", err)
			return
		}
		if gasPriceX != nil {
			gasPrice, _ = new(big.Float).Mul(new(big.Float).SetInt(gasPrice), gasPriceX).Int(nil)
		}
	}

	if gasLimit == 0 {
		msg := ethereum.CallMsg{From: account.Address, To: &addr, Value: amount, Data: data}
		if estimateWithGas {
			msg.GasPrice = gasPrice
		}
		gasLimit, err = w.sdk.Node().EstimateGas(context.Background(), msg)
		if err != nil {
			if strings.Contains(err.Error(), "has been executed") {
				log.Info("Transaction already executed")
				return "", nil
			}

			err = fmt.Errorf("Estimate gas limit error %v, account %s", err, account.Address)
			return
		}
	}

	if dry {
		fmt.Printf(
			"Estimated tx successfully account %s gas_price %s gas_limit %v target %s amount %s data %x\n",
			account.Address, gasPrice, gasLimit, addr, amount, data)
		return
	}

	gasLimit = uint64(1.3 * float32(gasLimit))
	limit := GetChainGasLimit(w.sdk.ChainID(), gasLimit)
	if limit < gasLimit {
		err = fmt.Errorf("Send tx estimated gas limit(%v) higher than max %v", gasLimit, limit)
		return
	}
	provider, nonces := w.GetAccount(account)
	nonce, err := nonces.Acquire()
	if err != nil {
		return
	}
	tx := types.NewTransaction(nonce, addr, amount, limit, gasPrice, data)
	tx, err = provider.SignTx(account, tx, big.NewInt(w.nativeID))
	if err != nil {
		nonces.Update(false)
		err = fmt.Errorf("Sign tx error %v", err)
		return
	}

	if w.Broadcast {
		_, err = w.sdk.Broadcast(context.Background(), tx)
	} else {
		err = w.sdk.Node().SendTransaction(context.Background(), tx)
	}
	//TODO: Check err here before update nonces
	nonces.Update(err == nil)
	log.Info("Sending tx", "hash", tx.Hash(), "account", account.Address, "nonce", tx.Nonce(), "limit", tx.Gas(), "gasPrice", tx.GasPrice(), "err", err)
	return tx.Hash().String(), err
}

func (w *Wallet) SendLight(ctx context.Context, provider Provider, nonces NonceProvider, account accounts.Account, addr common.Address, data []byte, amount *big.Int, price GasPriceOracle, limit uint64) (hash string, err error) {
	nonce, err := nonces.Acquire()
	if err != nil {
		return
	}
	tx := types.NewTransaction(nonce, addr, amount, limit, price.Price(), data)
	tx, err = provider.SignTx(account, tx, big.NewInt(w.nativeID))
	if err != nil {
		nonces.Update(false)
		err = fmt.Errorf("Sign tx error %v", err)
		return
	}

	if w.Broadcast {
		_, err = w.sdk.Broadcast(ctx, tx)
	} else {
		err = w.sdk.Node().SendTransaction(ctx, tx)
	}
	nonces.Update(err == nil)
	//TODO: Check err here before update nonces
	log.Info("Sending tx", "hash", tx.Hash(), "account", account.Address, "nonce", tx.Nonce(), "limit", tx.Gas(), "gasPrice", tx.GasPrice(), "err", err)
	return tx.Hash().String(), err
}

func (w *Wallet) Account() (accounts.Account, Provider, NonceProvider) {
	w.RLock()
	defer w.RUnlock()
	return w.account, w.provider, w.nonces[w.account]
}

func (w *Wallet) GasPrice() (price *big.Int, err error) {
	return w.sdk.Node().SuggestGasPrice(context.Background())
}

func (w *Wallet) GasTip() (price *big.Int, err error) {
	return w.sdk.Node().SuggestGasTipCap(context.Background())
}

func (w *Wallet) updateAccounts() {
	w.Lock()
	defer w.Unlock()
	accounts := []accounts.Account{}
	for a := range w.providers {
		accounts = append(accounts, a)
		w.nonces[a] = NewRemoteNonceProvider(w.sdk, a.Address)
	}
	w.accounts = accounts
	w.cursor = 0
}

// Round robin
func (w *Wallet) Select() (accounts.Account, Provider, NonceProvider) {
	w.Lock()
	defer w.Unlock()
	account := w.accounts[w.cursor]
	w.cursor = (w.cursor + 1) % len(w.accounts)
	return account, w.providers[account], w.nonces[account]
}

func (w *Wallet) Upgrade() *EthWallet {
	return &EthWallet{w}
}

func (w *Wallet) EstimateGasWithAccount(account accounts.Account, addr common.Address, amount *big.Int, data []byte) (gasPrice *big.Int, gasLimit uint64, err error) {
	gasPrice, err = w.GasPrice()
	if err != nil {
		err = fmt.Errorf("Get gas price error %v", err)
		return
	}
	msg := ethereum.CallMsg{From: account.Address, To: &addr, Value: amount, Data: data}
	gasLimit, err = w.sdk.Node().EstimateGas(context.Background(), msg)
	if err != nil {
		err = fmt.Errorf("Estimate gas limit error %v, account %s", err, account.Address)
		return
	}
	return
}

func (w *Wallet) QuickSendWithMaxLimit(chainId uint64, account accounts.Account, addr common.Address, amount *big.Int, maxLimit *big.Int, price GasPriceOracle, gasPriceX *big.Float, data []byte) (hash string, err error) {
	return w.SendWithMaxLimit(chainId, account, addr, amount, maxLimit, price.Price(), gasPriceX, data)
}

func (w *Wallet) SendWithMaxLimit(chainId uint64, account accounts.Account, addr common.Address, amount *big.Int, maxLimit *big.Int, gasPrice *big.Int, gasPriceX *big.Float, data []byte) (hash string, err error) {
	if maxLimit == nil || maxLimit.Sign() <= 0 {
		err = fmt.Errorf("max limit is zero or missing")
		return
	}
	if gasPrice == nil || gasPrice.Sign() <= 0 {
		gasPrice, err = w.GasPrice()
		if err != nil {
			err = fmt.Errorf("Get gas price error %v", err)
			return
		}
	}

	if gasPriceX != nil {
		gasPrice, _ = new(big.Float).Mul(new(big.Float).SetInt(gasPrice), gasPriceX).Int(nil)
	}

	var gasLimit uint64
	msg := ethereum.CallMsg{From: account.Address, To: &addr, Value: amount, Data: data}
	gasLimit, err = w.sdk.Node().EstimateGas(context.Background(), msg)
	if err != nil {
		if strings.Contains(err.Error(), "has been executed") {
			log.Info("Transaction already executed")
			return "", nil
		}

		err = fmt.Errorf("Estimate gas limit error %v, account %s", err, account.Address)
		return
	}

	if chainId == base.OPTIMISM {
		gasLimit = uint64(2 * float32(gasLimit))
	} else {
		gasLimit = uint64(1.2 * float32(gasLimit))
	}

	limit := GetChainGasLimit(w.sdk.ChainID(), gasLimit)
	if limit < gasLimit {
		err = fmt.Errorf("Send tx estimated gas limit(%v) higher than chain max limit %v", gasLimit, limit)
		return
	}

	delta := new(big.Int).Sub(maxLimit, new(big.Int).Mul(big.NewInt(int64(gasLimit)), gasPrice))
	if delta.Sign() < 0 {
		err = fmt.Errorf("Send tx estimated gas (limit %v, price %s) higher than max limit %s", gasLimit, gasPrice, maxLimit)
		return
	}

	provider, nonces := w.GetAccount(account)
	nonce, err := nonces.Acquire()
	if err != nil {
		return
	}
	tx := types.NewTransaction(nonce, addr, amount, limit, gasPrice, data)
	tx, err = provider.SignTx(account, tx, big.NewInt(w.nativeID))
	if err != nil {
		nonces.Update(false)
		err = fmt.Errorf("Sign tx error %v", err)
		return
	}
	if w.Broadcast {
		_, err = w.sdk.Broadcast(context.Background(), tx)
	} else {
		err = w.sdk.Node().SendTransaction(context.Background(), tx)
	}
	//TODO: Check err here before update nonces
	nonces.Update(err == nil)
	hash = tx.Hash().String()
	log.Info("Compose dst chain tx", "hash", hash, "account", account.Address, "nonce", tx.Nonce(), "limit", tx.Gas(), "gasPrice", tx.GasPrice())
	log.Info("Sent tx with limit", "chainId", chainId, "hash", hash, "delta", delta, "err", err)
	return hash, err
}

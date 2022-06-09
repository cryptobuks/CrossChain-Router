package stellar

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	hProtocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/txnbuild"
)

// BuildRawTransaction build raw tx
//nolint:funlen,gocyclo // ok
func (b *Bridge) BuildRawTransaction(args *tokens.BuildTxArgs) (rawTx interface{}, err error) {
	if !params.IsTestMode && args.ToChainID.String() != b.ChainConfig.ChainID {
		return nil, tokens.ErrToChainIDMismatch
	}
	if args.From == "" {
		return nil, fmt.Errorf("forbid empty sender")
	}
	routerMPC, err := router.GetRouterMPC(args.GetTokenID(), b.ChainConfig.ChainID)
	if err != nil {
		return nil, err
	}
	if !common.IsEqualIgnoreCase(args.From, routerMPC) {
		log.Error("build tx mpc mismatch", "have", args.From, "want", routerMPC)
		return nil, tokens.ErrSenderMismatch
	}

	switch args.SwapType {
	case tokens.ERC20SwapType:
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}

	mpcPubkey := router.GetMPCPublicKey(args.From)
	if mpcPubkey == "" {
		return nil, tokens.ErrMissMPCPublicKey
	}

	erc20SwapInfo := args.ERC20SwapInfo
	multichainToken := router.GetCachedMultichainToken(erc20SwapInfo.TokenID, args.ToChainID.String())
	if multichainToken == "" {
		log.Warn("get multichain token failed", "tokenID", erc20SwapInfo.TokenID, "chainID", args.ToChainID)
		return nil, tokens.ErrMissTokenConfig
	}

	token := b.GetTokenConfig(multichainToken)
	if token == nil {
		return nil, tokens.ErrMissTokenConfig
	}

	assetI, exist := assetMap.Load(token.ContractAddress)
	if !exist {
		return nil, fmt.Errorf("non exist asset %v", token.ContractAddress)
	}
	asset := assetI.(txnbuild.Asset)

	receiver, amount, err := b.getReceiverAndAmount(args, multichainToken)
	if err != nil {
		return nil, err
	}
	amt := getPaymentAmount(amount, token)
	var fee string

	extra := args.Extra
	if extra == nil {
		extra, err = b.swapoutDefaultArgs(args, multichainToken)
		if err != nil {
			return nil, err
		}
		args.Extra = extra
		fee = *extra.Fee
	} else {
		if extra.Fee != nil {
			fee = *extra.Fee
		}
	}
	fromAccount, err := b.GetAccount(args.From)
	if err != nil {
		return nil, err
	}
	// check XLM
	if !b.checkXlmBalanceEnough(fromAccount) {
		return nil, tokens.ErrMissTokenConfig
	}

	memo, err := buildMemo(args.FromChainID.String(), args.SwapID, strconv.Itoa(args.LogIndex))
	if err != nil {
		return nil, err
	}
	return NewUnsignedPaymentTransaction(fromAccount, b.NetworkStr, receiver, amt, fee, memo, asset)
}

// 判断swapID isHex 是的话写入swapID
func buildMemo(fromChainID, swapID, logIndex string) (*txnbuild.MemoHash, error) {
	rtn := new(txnbuild.MemoHash)

	b, err := hex.DecodeString(swapID)
	if err != nil || len(b) > 32 {
		return rtn, nil
	}
	for i := 0; i < len(b); i++ {
		rtn[i] = b[i]
	}
	return rtn, nil
}

func getPaymentAmount(amount *big.Int, token *tokens.TokenConfig) string {
	value := float64(amount.Int64()) / math.Pow10(int(token.Decimals))
	return fmt.Sprintf("%.7f", value)
}

func (b *Bridge) getReceiverAndAmount(args *tokens.BuildTxArgs, multichainToken string) (receiver string, amount *big.Int, err error) {
	erc20SwapInfo := args.ERC20SwapInfo
	receiver = args.Bind
	if receiver == "" || !b.IsValidAddress(args.Bind) {
		log.Warn("swapout to wrong receiver", "receiver", args.Bind)
		return receiver, amount, errors.New("can not swapout to empty or invalid receiver")
	}
	fromBridge := router.GetBridgeByChainID(args.FromChainID.String())
	if fromBridge == nil {
		return receiver, amount, tokens.ErrNoBridgeForChainID
	}
	fromTokenCfg := fromBridge.GetTokenConfig(erc20SwapInfo.Token)
	if fromTokenCfg == nil {
		log.Warn("get token config failed", "chainID", args.FromChainID, "token", erc20SwapInfo.Token)
		return receiver, amount, tokens.ErrMissTokenConfig
	}
	toTokenCfg := b.GetTokenConfig(multichainToken)
	if toTokenCfg == nil {
		return receiver, amount, tokens.ErrMissTokenConfig
	}
	amount = tokens.CalcSwapValue(erc20SwapInfo.TokenID, args.FromChainID.String(), b.ChainConfig.ChainID, args.OriginValue, fromTokenCfg.Decimals, toTokenCfg.Decimals, args.OriginFrom, args.OriginTxTo)
	return receiver, amount, err
}

func (b *Bridge) swapoutDefaultArgs(txargs *tokens.BuildTxArgs, multichainToken string) (*tokens.AllExtras, error) {
	token := b.GetTokenConfig(multichainToken)
	if token == nil {
		return nil, tokens.ErrMissTokenConfig
	}

	seq, err := b.GetSeq(txargs)
	if err != nil {
		log.Warn("get sequence failed", "err", err)
		return nil, err
	}

	feeRes := b.GetFee()
	fee := strconv.Itoa(feeRes)

	return &tokens.AllExtras{
		Sequence: seq,
		Fee:      &fee,
	}, nil
}

// GetTxBlockInfo impl NonceSetter interface
func (b *Bridge) GetTxBlockInfo(txHash string) (blockHeight, blockTime uint64) {
	txStatus, err := b.GetTransactionStatus(txHash)
	if err != nil {
		return 0, 0
	}
	return txStatus.BlockHeight, txStatus.BlockTime
}

// GetPoolNonce impl NonceSetter interface
func (b *Bridge) GetPoolNonce(address, _height string) (uint64, error) {
	return uint64(0), nil
}

// GetSeq returns account tx sequence
func (b *Bridge) GetSeq(args *tokens.BuildTxArgs) (nonceptr *uint64, err error) {
	var nonce uint64

	// if params.IsParallelSwapEnabled() {
	// 	nonce, err = b.AllocateNonce(args)
	// 	return &nonce, err
	// }

	// if params.IsAutoSwapNonceEnabled(b.ChainConfig.ChainID) { // increase automatically
	// 	nonce = b.GetSwapNonce(args.From)
	// 	return &nonce, nil
	// }

	// nonce, err = b.GetPoolNonce(args.From, "pending")
	// if err != nil {
	// 	return nil, err
	// }
	nonce = b.AdjustNonce(args.From, nonce)
	return &nonce, nil
}

// NewUnsignedPaymentTransaction build stellar payment tx
func NewUnsignedPaymentTransaction(
	from *hProtocol.Account, network,
	dest, amt, fee string, memo *txnbuild.MemoHash, asset txnbuild.Asset) (*txnbuild.Transaction, error) {
	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        from,
			IncrementSequenceNum: true,
			BaseFee:              txnbuild.MinBaseFee,
			Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
			Memo:                 memo,
			Operations: []txnbuild.Operation{
				&txnbuild.Payment{
					Destination: dest,
					Amount:      amt,
					Asset:       asset,
				},
			},
		},
	)
	if err != nil {
		return nil, err
	}

	hash, err := tx.Hash(network)
	if err != nil {
		return nil, err
	}
	log.Info("Build unsigned payment tx success",
		"destination", dest, "amount", amt, "memo", memo,
		"fee", fee,
		"signing hash", hash)

	return tx, nil
}

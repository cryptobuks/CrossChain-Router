# How to deploy router swap

## 0. compile

```shell
make all
```
run the above command, it will generate `./build/bin/swaprouter` binary.

## 1. deploy `AnyswapRouter`

deploy a `AnyswapRouter` contract for each supported blockchain

## 2. deploy `AnyswapERC20`

deploy a `AnyswapERC20` contract for each token on each blockchain

## 3. deploy `RouterConfig`

deploy a `RouterConfig` contract to store router bridge configs

## 4. set router config on chain

call `RouterConfig` contract to set configs on blcokchain.

The following is the most used functions, please ref. the abi for more info.

### 4.1 set chain config

call the following contract function:

```solidity
function setChainConfig(uint256 chainID, string blockChain, address routerContract, uint64 confirmations, uint64 initialHeight);
```

### 4.2 set token config

call the following contract function:

```solidity
function setTokenConfig(string tokenID, uint256 chainID, address tokenAddr, uint8 decimals, uint256 version);
```

### 4.3 set swap and fee config

#### 4.3.1 set swap and fee config meantime

call the following contract function:

```solidity
function setSwapAndFeeConfig(
        string tokenID, uint256 srcChainID, uint256 dstChainID,
        uint256 maxSwap, uint256 minSwap, uint256 bigSwap,
        uint256 maxFee, uint256 minFee, uint256 feeRate)
```

#### 4.3.2 set swap config

call the following contract function to set swap config:

max/min/big value always uses decimals 18.

```solidity
function setSwapConfig(string tokenID, uint256 srcChainID, uint256 dstChainID, uint256 max, uint256 min, uint256 big);
```

swap config is stored in a map with keys tokenID,srcChainID,dstChainID
```solidity
mapping (bytes32 => mapping(uint256 => mapping(uint256 => SwapConfig))) private _swapConfig;
```

```text
the actual swap config is decided by the following steps
1. if _swapConfig[tokenID][srcChainID][dstChainID] exist, then use it.
2. else if _swapConfig[tokenID][srcChainID][0] exist, then use it.
3. else if _swapConfig[tokenID][0][dstChainID] exist, then use it.
4. else use _swapConfig[tokenID][0][0].
```

**Notice: you should always set _swapConfig[tokenID][0][0].**

#### 4.3.3 set fee config

call the following contract function:

rate is per million ration.

max/min value uses decimals same as token decimals on source chain.

```solidity
function setFeeConfig(string tokenID, uint256 srcChainID, uint256 dstChainID, uint256 max, uint256 min, uint256 big);
```

fee config is stored in a map with keys tokenID,srcChainID,dstChainID
```solidity
mapping (bytes32 => mapping(uint256 => mapping(uint256 => FeeConfig))) private _feeConfig;
```

```text
the actual fee config is decided by the following steps
1. if _feeConfig[tokenID][srcChainID][dstChainID] exist, then use it.
2. else if _feeConfig[tokenID][srcChainID][0] exist, then use it.
3. else if _feeConfig[tokenID][0][dstChainID] exist, then use it.
4. else use _feeConfig[tokenID][0][0].
```

### 4.4 set mpc address's public key

call the following contract function:

```solidity
function setMPCPubkey(address addr, string pubkey);
```

## 5. add local config file

please ref. [config-example.toml](https://github.com/anyswap/CrossChain-Router/blob/main/params/config-example.toml)

## 6. run swaprouter

```shell
# for server run (add '--runserver' option)
setsid ./build/bin/swaprouter --config config.toml --log logs/routerswap.log --runserver

# for oracle run
setsid ./build/bin/swaprouter --config config.toml --log logs/routerswap.log
```

## 7. sub commands

get all sub command list and help info, run

```shell
./build/bin/swaprouter -h
```

sub commands:

`admin` is admin tool

`config` is tool to process and query config data

## 8. RPC api

please ref. [server rpc api](https://github.com/anyswap/CrossChain-Router/blob/main/rpc/README.md)

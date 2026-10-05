package main

// Creates an Avalanche Native Token (ANT) on the X-Chain, and optionally mints
// more units of a variable-cap token you already created.
//
// Adapted from the official AvalancheGo example at
// wallet/subnet/primary/examples/create-asset.
//
// Usage (PRIVATE_KEY is read from snippets/.env, or set it in the shell):
//   go run .                                    # create a token on Fuji testnet
//   NETWORK=mainnet go run .                    # create a token on mainnet
//   MINT_ASSET_ID=... MINT_AMOUNT=500 go run .  # mint more (variable-cap only)

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/chain/x"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/joho/godotenv"
)

// ---------------------------------------------------------------------------
// Edit these to define your token. They cannot be changed after creation.
// Name, symbol, decimals and supply can also be set in .env with TOKEN_NAME,
// TOKEN_SYMBOL, TOKEN_DECIMALS and TOKEN_SUPPLY.
// ---------------------------------------------------------------------------
const (
	defaultTokenName   = "My Test Token"
	defaultTokenSymbol = "MTT"     // 1 to 4 upper case letters (A-Z)
	defaultDecimals    = 2         // decimal places: 2 means 1 token = 100 base units
	tokenType          = "fixed"   // "fixed" = supply created once, "variable" = you can mint more later
	defaultTotalSupply = 1_000_000 // whole tokens, only used for "fixed"
)

// Index of the secp256k1 feature extension on the X-Chain (fungible tokens).
const secp256k1FxIndex = 0

func main() {
	// Load PRIVATE_KEY (and optional settings) from snippets/.env.
	// ponytail: error ignored so the variables can also come from the shell
	godotenv.Load("../.env")

	// Token settings: from .env if set, otherwise the defaults above.
	tokenName := os.Getenv("TOKEN_NAME")
	if tokenName == "" {
		tokenName = defaultTokenName
	}
	tokenSymbol := os.Getenv("TOKEN_SYMBOL")
	if tokenSymbol == "" {
		tokenSymbol = defaultTokenSymbol
	}
	if !regexp.MustCompile(`^[A-Z]{1,4}$`).MatchString(tokenSymbol) {
		log.Fatalf("TOKEN_SYMBOL must be 1 to 4 upper case letters, e.g. TOKEN_SYMBOL=MTT")
	}
	decimals := defaultDecimals
	if s := os.Getenv("TOKEN_DECIMALS"); s != "" {
		// ponytail: the X-Chain allows up to 32, but above 19 not even 1 token fits in a uint64
		n, err := strconv.ParseUint(s, 10, 8)
		if err != nil || n > 19 {
			log.Fatalf("TOKEN_DECIMALS must be a whole number from 0 to 19, e.g. TOKEN_DECIMALS=2")
		}
		decimals = int(n)
	}
	totalSupply := uint64(defaultTotalSupply)
	if s := os.Getenv("TOKEN_SUPPLY"); s != "" {
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil || n == 0 {
			log.Fatalf("TOKEN_SUPPLY must be a positive whole number of tokens, e.g. TOKEN_SUPPLY=1000000")
		}
		totalSupply = n
	}
	if totalSupply > math.MaxUint64/pow10(decimals) {
		log.Fatalf("TOKEN_SUPPLY is too large for %d decimal places: lower TOKEN_SUPPLY or TOKEN_DECIMALS", decimals)
	}

	key, err := parseKey(os.Getenv("PRIVATE_KEY"))
	if err != nil {
		log.Fatalf("could not read PRIVATE_KEY: %s", err)
	}

	uri := primary.FujiAPIURI
	if strings.EqualFold(os.Getenv("NETWORK"), "mainnet") {
		uri = primary.MainnetAPIURI
	}

	// Sync the wallet: fetches the UTXOs your key controls on the network.
	ctx := context.Background()
	kc := secp256k1fx.NewKeychain(key)
	wallet, err := primary.MakeWallet(ctx, uri, kc, kc, primary.WalletConfig{})
	if err != nil {
		log.Fatalf("failed to initialize wallet: %s", err)
	}
	xWallet := wallet.X()
	xCtx := xWallet.Builder().Context()

	myAddr, err := address.Format("X", constants.GetHRP(xCtx.NetworkID), key.Address().Bytes())
	if err != nil {
		log.Fatalf("failed to format address: %s", err)
	}
	fmt.Printf("Network: %s\nYour X-Chain address: %s\n", uri, myAddr)

	// Everything below is owned by your single address (1-of-1).
	// For a multisig, add more addresses to Addrs and raise Threshold.
	owner := secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs:     []ids.ShortID{key.Address()},
	}

	if assetIDStr := os.Getenv("MINT_ASSET_ID"); assetIDStr != "" {
		mint(xWallet, assetIDStr, owner, decimals)
		return
	}

	var initialState []verify.State
	switch tokenType {
	case "fixed":
		initialState = []verify.State{
			&secp256k1fx.TransferOutput{
				Amt:          totalSupply * pow10(decimals),
				OutputOwners: owner,
			},
		}
	case "variable":
		// A MintOutput grants the right to mint; no tokens exist yet.
		initialState = []verify.State{
			&secp256k1fx.MintOutput{OutputOwners: owner},
		}
	default:
		log.Fatalf("tokenType must be \"fixed\" or \"variable\"")
	}

	fmt.Printf("Token: %s (%s), supply: %d, decimals: %d\n", tokenName, tokenSymbol, totalSupply, decimals)
	fmt.Printf("Creation fee: %s AVAX\n", formatAVAX(xCtx.CreateAssetTxFee))
	tx, err := xWallet.IssueCreateAssetTx(
		tokenName,
		tokenSymbol,
		byte(decimals),
		map[uint32][]verify.State{secp256k1FxIndex: initialState},
	)
	if err != nil {
		log.Fatalf("failed to create asset: %s", err)
	}

	fmt.Printf("\nSuccess! Your token's asset ID is:\n  %s\n", tx.ID())
	fmt.Println("(The asset ID is the ID of the transaction that created it.)")
}

// mint creates new units of a variable-cap token and sends them to owner.
// Only works if your key holds the token's mint right (tokenType "variable").
// decimals must match the token's decimal places (TOKEN_DECIMALS when it was created).
func mint(xWallet x.Wallet, assetIDStr string, owner secp256k1fx.OutputOwners, decimals int) {
	assetID, err := ids.FromString(assetIDStr)
	if err != nil {
		log.Fatalf("invalid MINT_ASSET_ID: %s", err)
	}
	wholeTokens, err := strconv.ParseUint(os.Getenv("MINT_AMOUNT"), 10, 64)
	if err != nil || wholeTokens == 0 || wholeTokens > math.MaxUint64/pow10(decimals) {
		log.Fatalf("set MINT_AMOUNT to a positive whole number of tokens (not too large for %d decimal places)", decimals)
	}

	tx, err := xWallet.IssueOperationTxMintFT(map[ids.ID]*secp256k1fx.TransferOutput{
		assetID: {
			Amt:          wholeTokens * pow10(decimals),
			OutputOwners: owner,
		},
	})
	if err != nil {
		log.Fatalf("failed to mint: %s", err)
	}
	fmt.Printf("Minted %d tokens. Transaction ID: %s\n", wholeTokens, tx.ID())
}

// parseKey accepts either a hex private key (as exported by Core wallet,
// with or without 0x) or the "PrivateKey-..." format used by AvalancheGo.
func parseKey(s string) (*secp256k1.PrivateKey, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("environment variable is empty")
	}
	if strings.HasPrefix(s, secp256k1.PrivateKeyPrefix) {
		k := new(secp256k1.PrivateKey)
		return k, k.UnmarshalText([]byte(s))
	}
	b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	if err != nil {
		return nil, fmt.Errorf("not valid hex or PrivateKey- format")
	}
	return secp256k1.ToPrivateKey(b)
}

func pow10(n int) uint64 {
	r := uint64(1)
	for i := 0; i < n; i++ {
		r *= 10
	}
	return r
}

// formatAVAX converts nAVAX (1 AVAX = 1,000,000,000 nAVAX) to a readable string.
func formatAVAX(nAVAX uint64) string {
	return strconv.FormatFloat(float64(nAVAX)/1e9, 'f', -1, 64)
}

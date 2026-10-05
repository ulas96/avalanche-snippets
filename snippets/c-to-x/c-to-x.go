package main

// Moves AVAX from your C-Chain balance to your X-Chain balance on Fuji testnet,
// so create-ant.go can pay its fee. Reads PRIVATE_KEY and C_TO_X_AMOUNT
// (in AVAX, e.g. 0.05) from snippets/.env.
//
// Adapted from the official AvalancheGo example at
// wallet/subnet/primary/examples/c-chain-export.
//
// Run from the snippets/c-to-x folder:
//   go run .

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/avm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"
	"github.com/joho/godotenv"
)

func main() {
	// 1. Load your private key and C_TO_X_AMOUNT from snippets/.env.
	// ponytail: error ignored so the variables can also come from the shell
	godotenv.Load("../.env")

	// How much AVAX to move, e.g. C_TO_X_AMOUNT=0.05. One token creation costs 0.01 AVAX.
	avax, amount, ok := parseAmount(os.Getenv("C_TO_X_AMOUNT"))
	if !ok {
		log.Fatal("set C_TO_X_AMOUNT in .env to a positive amount of AVAX, e.g. C_TO_X_AMOUNT=0.05")
	}

	keyBytes, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(os.Getenv("PRIVATE_KEY")), "0x"))
	if err != nil {
		log.Fatal("PRIVATE_KEY must be a hex string: ", err)
	}
	key, err := secp256k1.ToPrivateKey(keyBytes)
	if err != nil {
		log.Fatal(err)
	}

	// 2. Connect to Fuji. The same key controls your C-Chain and X-Chain addresses.
	kc := secp256k1fx.NewKeychain(key)
	wallet, err := primary.MakeWallet(context.Background(), primary.FujiAPIURI, kc, kc, primary.WalletConfig{})
	if err != nil {
		log.Fatal("failed to initialize wallet: ", err)
	}
	cChainID := wallet.C().Builder().Context().BlockchainID
	xChainID := wallet.X().Builder().Context().BlockchainID
	fmt.Println("C-Chain address:", key.EthAddress())

	// 3. Moving between chains takes two steps. Who receives it on the X-Chain: you.
	me := secp256k1fx.OutputOwners{Threshold: 1, Addrs: []ids.ShortID{key.Address()}}

	// Step A: export from the C-Chain (the AVAX leaves C and waits "in between").
	// SKIP_EXPORT=1 only imports AVAX that an earlier run already exported.
	if os.Getenv("SKIP_EXPORT") != "1" {
		// The public Fuji API disables avax.getAtomicTxStatus, so don't wait on it here;
		// Step B retries until the exported AVAX is importable.
		exportTx, err := wallet.C().IssueExportTx(xChainID, []*secp256k1fx.TransferOutput{{
			Amt:          amount,
			OutputOwners: me,
		}}, common.WithAssumeDecided())
		if err != nil {
			log.Fatal("export from C-Chain failed: ", err)
		}
		fmt.Println("Exported from C-Chain. Transaction ID:", exportTx.ID())
	}

	// Step B: import into the X-Chain (picks up the waiting AVAX).
	// Retries while the export finalizes; a failed attempt sends nothing and costs nothing.
	// ponytail: fixed 15 x 2s retry window, raise it if Fuji is slow.
	var importTx *txs.Tx
	for try := 1; ; try++ {
		importTx, err = wallet.X().IssueImportTx(cChainID, &me)
		if err == nil {
			break
		}
		if try == 15 {
			log.Fatal("import into X-Chain failed (the exported AVAX is not lost, rerun with SKIP_EXPORT=1): ", err)
		}
		time.Sleep(2 * time.Second)
	}
	fmt.Println("Imported into X-Chain. Transaction ID:", importTx.ID())
	fmt.Printf("Done! Moved %g AVAX to your X-Chain address.\n", avax)
}

// parseAmount reads an AVAX amount like "0.05" and also returns it in nAVAX
// (1 AVAX = 1,000,000,000 nAVAX). ok is false unless it is a positive, finite
// amount of at least 1 nAVAX.
func parseAmount(s string) (avax float64, nAVAX uint64, ok bool) {
	avax, err := strconv.ParseFloat(s, 64)
	if err != nil || !(avax > 0) || math.IsInf(avax, 0) { // also rejects "NaN" and "Inf"
		return 0, 0, false
	}
	nAVAX = uint64(math.Round(avax * float64(units.Avax)))
	return avax, nAVAX, nAVAX > 0
}

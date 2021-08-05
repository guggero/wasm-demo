// +build js

package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/btcsuite/btcutil/psbt"
	"os"
	"runtime/debug"
	"strings"
	"syscall/js"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/hdkeychain"
	flags "github.com/jessevdk/go-flags"
	"github.com/lightningnetwork/lnd/aezeed"
	"github.com/lightningnetwork/lnd/build"
	"github.com/lightningnetwork/lnd/signal"
	"github.com/lightningnetwork/lnd/zpay32"
)

var (
	chainParams = &chaincfg.MainNetParams
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Debugf("Recovered in f: %v", r)
			debug.PrintStack()
		}
	}()

	// Setup JS callbacks.
	js.Global().Set("demoIsReady", js.ValueOf(true))
	js.Global().Set("demoDecodeInvoice", js.FuncOf(decodeInvoice))
	js.Global().Set("demoGenerateAezeed", js.FuncOf(generateAezeed))
	js.Global().Set("demoDecodePsbt", js.FuncOf(decodePsbt))

	cfg := config{}

	// Parse command line flags.
	parser := flags.NewParser(&cfg, flags.Default)
	parser.SubcommandsOptional = true

	_, err := parser.Parse()
	if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
		exit(err)
	}
	if err != nil {
		exit(err)
	}

	// Hook interceptor for os signals.
	shutdownInterceptor, err := signal.Intercept()
	if err != nil {
		exit(err)
	}

	logWriter := build.NewRotatingLogWriter()
	SetupLoggers(logWriter, shutdownInterceptor)

	err = build.ParseAndSetDebugLevels(cfg.DebugLevel, logWriter)
	if err != nil {
		exit(err)
	}

	log.Debugf("WASM demo ready")

	select {
	case <-shutdownInterceptor.ShutdownChannel():
		log.Debugf("Shutting down WASM demo")

		log.Debugf("Shutdown of WASM demo complete")
	}
}

func decodeInvoice(_ js.Value, args []js.Value) interface{} {
	if len(args) != 1 {
		return js.ValueOf("invalid use of decodeInvoice, " +
			"need 1 parameter: invoice")
	}

	invoiceStr := args[0].String()

	log.Infof("Decoding invoice %s", invoiceStr)
	invoice, err := zpay32.Decode(invoiceStr, chainParams)
	if err != nil {
		return js.ValueOf(err.Error())
	}

	invoiceJSON, err := json.Marshal(invoice)
	if err != nil {
		return js.ValueOf(err.Error())
	}

	return js.ValueOf(string(invoiceJSON))
}

func generateAezeed(_ js.Value, _ []js.Value) interface{} {
	log.Infof("Generating aezeed")

	var entropy [16]byte
	_, err := rand.Read(entropy[:])
	if err != nil {
		return js.ValueOf(err.Error())
	}

	seed, err := aezeed.New(aezeed.CipherSeedVersion, &entropy, time.Now())
	if err != nil {
		return js.ValueOf(err.Error())
	}

	mnemonic, err := seed.ToMnemonic(nil)
	if err != nil {
		return js.ValueOf(err.Error())
	}

	extendedKey, err := hdkeychain.NewMaster(entropy[:], chainParams)
	if err != nil {
		return js.ValueOf(err.Error())
	}

	nodePath, err := ParsePath(IdentityPath(chainParams))
	if err != nil {
		return js.ValueOf(err.Error())
	}

	nodeKey, err := DeriveChildren(extendedKey, nodePath)
	if err != nil {
		return js.ValueOf(err.Error())
	}

	nodePubKey, err := nodeKey.ECPubKey()
	if err != nil {
		return js.ValueOf(err.Error())
	}

	addrPath, err := ParsePath(WalletPath(chainParams, 0))
	if err != nil {
		return js.ValueOf(err.Error())
	}

	addrKey, err := DeriveChildren(extendedKey, addrPath)
	if err != nil {
		return js.ValueOf(err.Error())
	}

	addrPubkey, err := addrKey.ECPubKey()
	if err != nil {
		return js.ValueOf(err.Error())
	}
	hash160 := btcutil.Hash160(addrPubkey.SerializeCompressed())
	addrP2WKH, err := btcutil.NewAddressWitnessPubKeyHash(
		hash160, chainParams,
	)
	if err != nil {
		return js.ValueOf(err.Error())
	}

	return js.ValueOf(`
	aezeed:		` + strings.Join(mnemonic[:], " ") + `
	xprv:		` + extendedKey.String() + `
	node ID: 	` + hex.EncodeToString(nodePubKey.SerializeCompressed()) + `
	first address: 	` + addrP2WKH.EncodeAddress())
}

func decodePsbt(_ js.Value, args []js.Value) interface{} {
	if len(args) != 1 {
		return js.ValueOf("invalid use of decodePsbt, " +
			"need 1 parameter: psbt")
	}

	psbtStr := args[0].String()

	log.Infof("Decoding PSBT %s", psbtStr)
	packet, err := psbt.NewFromRawBytes(strings.NewReader(psbtStr), true)
	if err != nil {
		return js.ValueOf(err.Error())
	}

	packetJSON, err := json.Marshal(packet)
	if err != nil {
		return js.ValueOf(err.Error())
	}

	return js.ValueOf(string(packetJSON))
}

func exit(err error) {
	log.Debugf("Error running wasm client: %v", err)
	os.Exit(1)
}

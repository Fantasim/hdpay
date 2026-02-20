package main

import (
	"fmt"

	"github.com/Fantasim/hdpay/internal/wallet/hd"
	"github.com/btcsuite/btcd/chaincfg"
)

func main() {
	mnemonic, _ := hd.ReadMnemonicFromFile("/home/louis/test/mnemonic_testnet.txt")
	seed, _ := hd.MnemonicToSeed(mnemonic)

	net := &chaincfg.TestNet3Params
	masterKey, _ := hd.DeriveMasterKey(seed, net)

	fmt.Println("=== BTC Testnet ===")
	for i := 0; i < 3; i++ {
		addr, _ := hd.DeriveBTCAddress(masterKey, uint32(i), net)
		fmt.Printf("  index %d: %s\n", i, addr)
	}

	fmt.Println("\n=== BSC (EVM) ===")
	bscAddrs, _ := hd.GenerateBSCAddresses(masterKey, 3, nil)
	for _, a := range bscAddrs {
		fmt.Printf("  index %d: %s\n", a.AddressIndex, a.Address)
	}

	fmt.Println("\n=== SOL ===")
	solAddrs, _ := hd.GenerateSOLAddresses(seed, 3, nil)
	for _, a := range solAddrs {
		fmt.Printf("  index %d: %s\n", a.AddressIndex, a.Address)
	}

	fmt.Println("\n=== Expected (from memory) ===")
	fmt.Println("BTC[0]: tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr")
	fmt.Println("BSC[0]: 0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb")
	fmt.Println("SOL[0]: 3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx")
}

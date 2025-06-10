package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	// Import schnorr directly for Taproot address generation
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
)

// Derivation paths constants
const (
	// BIP44Purpose is the purpose field for P2PKH addresses.
	BIP44Purpose uint32 = 44
	// BIP49Purpose is the purpose field for P2SH-P2WPKH addresses.
	BIP49Purpose uint32 = 49
	// BIP84Purpose is the purpose field for P2WPKH addresses.
	BIP84Purpose uint32 = 84
	// BIP86Purpose is the purpose field for P2TR addresses.
	BIP86Purpose uint32 = 86

	// CoinTypeBitcoin is the coin type for Bitcoin.
	CoinTypeBitcoin uint32 = 0

	// DefaultAccount is the default account index (0).
	DefaultAccount uint32 = 0

	// ExternalChain is the chain index for external (receiving) addresses (0).
	ExternalChain uint32 = 0
	
	// InternalChain is the chain index for internal (change) addresses (1).
	InternalChain uint32 = 1

	// AddressBatchSize defines how many addresses to generate per batch.
	AddressBatchSize = 20
)

// deriveChildKey derives a child key from an extended key based on the specified path components.
// It expects the purpose, coin type, account, chain, and index.
func deriveChildKey(masterKey *hdkeychain.ExtendedKey, purpose, coinType, account, chain, index uint32) (*hdkeychain.ExtendedKey, error) {
	// Derive the purpose node (e.g., m/44')
	purposeKey, err := masterKey.Derive(purpose + hdkeychain.HardenedKeyStart)
	 if err != nil {
		 return nil, fmt.Errorf("failed to derive purpose key: %w", err)
	 }

	 // Derive the coin type node (e.g., m/44'/0')
	 coinTypeKey, err := purposeKey.Derive(coinType + hdkeychain.HardenedKeyStart)
	 if err != nil {
		 return nil, fmt.Errorf("failed to derive coin type key: %w", err)
	 }

	 // Derive the account node (e.g., m/44'/0'/0')
	 accountKey, err := coinTypeKey.Derive(account + hdkeychain.HardenedKeyStart)
	 if err != nil {
		 return nil, fmt.Errorf("failed to derive account key: %w", err)
	 }

	 // Derive the chain node (e.g., m/44'/0'/0'/0)
	 chainKey, err := accountKey.Derive(chain)
	 if err != nil {
		 return nil, fmt.Errorf("failed to derive chain key: %w", err)
	 }

	 // Derive the address index node (e.g., m/44'/0'/0'/0/i)
	 indexKey, err := chainKey.Derive(index)
	 if err != nil {
		 return nil, fmt.Errorf("failed to derive index key %d: %w", index, err)
	 }

	 return indexKey, nil
}

// generateLegacyAddress generates a P2PKH address from a derived key.
func generateLegacyAddress(key *hdkeychain.ExtendedKey, netParams *chaincfg.Params) (btcutil.Address, error) {
	pubKey, err := key.ECPubKey()
	 if err != nil {
		 return nil, fmt.Errorf("failed to get public key: %w", err)
	 }
	 // Use NewAddressPubKeyHash for P2PKH
	 return btcutil.NewAddressPubKeyHash(btcutil.Hash160(pubKey.SerializeCompressed()), netParams)
}

// generateNestedSegWitAddress generates a P2SH-P2WPKH address from a derived key.
func generateNestedSegWitAddress(key *hdkeychain.ExtendedKey, netParams *chaincfg.Params) (btcutil.Address, error) {
	pubKey, err := key.ECPubKey()
	 if err != nil {
		 return nil, fmt.Errorf("failed to get public key: %w", err)
	 }
	 pubKeyBytes := pubKey.SerializeCompressed()
	 pubKeyHash := btcutil.Hash160(pubKeyBytes)

	 // Create P2WPKH script (witness program)
	 builder := txscript.NewScriptBuilder()
	 builder.AddOp(txscript.OP_0)
	 builder.AddData(pubKeyHash)
	 witnessScript, err := builder.Script()
	 if err != nil {
		 return nil, fmt.Errorf("failed to build witness script: %w", err)
	 }

	 // Create P2SH address from the P2WPKH script hash
	 // The witnessScript itself is the redeem script for P2SH
	 return btcutil.NewAddressScriptHash(witnessScript, netParams)
}

// generateNativeSegWitAddress generates a P2WPKH address from a derived key.
func generateNativeSegWitAddress(key *hdkeychain.ExtendedKey, netParams *chaincfg.Params) (btcutil.Address, error) {
	pubKey, err := key.ECPubKey()
	 if err != nil {
		 return nil, fmt.Errorf("failed to get public key: %w", err)
	 }
	 pubKeyBytes := pubKey.SerializeCompressed()
	 pubKeyHash := btcutil.Hash160(pubKeyBytes)
	 return btcutil.NewAddressWitnessPubKeyHash(pubKeyHash, netParams)
}

// generateTaprootAddress generates a P2TR address from a derived key.
func generateTaprootAddress(key *hdkeychain.ExtendedKey, netParams *chaincfg.Params) (btcutil.Address, error) {
	pubKey, err := key.ECPubKey()
	 if err != nil {
		 return nil, fmt.Errorf("failed to get public key: %w", err)
	 }
	 // Taproot uses Schnorr keys, need to convert the ECDSA pubkey
	 // ComputeTaprootKeyNoScript gives the tweaked Taproot internal key.
	 taprootPubKey := txscript.ComputeTaprootKeyNoScript(pubKey)
	 // NewAddressTaproot expects the 32-byte x-only public key
	 return btcutil.NewAddressTaproot(schnorr.SerializePubKey(taprootPubKey), netParams)
}

// generateAddressBatch generates and displays a batch of addresses for a specific chain (external or internal)
func generateAddressBatch(masterKey *hdkeychain.ExtendedKey, netParams *chaincfg.Params, currentBatchStart uint32, chain uint32, chainName string) {
	fmt.Printf("\n--- %s (chain=%d) ---\n", chainName, chain)
	
	fmt.Println("\nLegacy (BIP44 - P2PKH):")
	for i := currentBatchStart; i < currentBatchStart+AddressBatchSize; i++ {
		legacyKey, err := deriveChildKey(masterKey, BIP44Purpose, CoinTypeBitcoin, DefaultAccount, chain, i)
		if err != nil {
			fmt.Printf("  Erro ao derivar chave legacy para índice %d: %v\n", i, err)
			continue
		}
		legacyAddr, err := generateLegacyAddress(legacyKey, netParams)
		if err != nil {
			fmt.Printf("  Erro ao gerar endereço legacy para índice %d: %v\n", i, err)
		} else {
			fmt.Printf("  %d: %s\n", i, legacyAddr.EncodeAddress())
		}
	}

	fmt.Println("\nNested SegWit (BIP49 - P2SH-P2WPKH):")
	for i := currentBatchStart; i < currentBatchStart+AddressBatchSize; i++ {
		nestedKey, err := deriveChildKey(masterKey, BIP49Purpose, CoinTypeBitcoin, DefaultAccount, chain, i)
		if err != nil {
			fmt.Printf("  Erro ao derivar chave nested segwit para índice %d: %v\n", i, err)
			continue
		}
		nestedAddr, err := generateNestedSegWitAddress(nestedKey, netParams)
		if err != nil {
			fmt.Printf("  Erro ao gerar endereço nested segwit para índice %d: %v\n", i, err)
		} else {
			fmt.Printf("  %d: %s\n", i, nestedAddr.EncodeAddress())
		}
	}

	fmt.Println("\nNative SegWit (BIP84 - P2WPKH):")
	for i := currentBatchStart; i < currentBatchStart+AddressBatchSize; i++ {
		nativeKey, err := deriveChildKey(masterKey, BIP84Purpose, CoinTypeBitcoin, DefaultAccount, chain, i)
		if err != nil {
			fmt.Printf("  Erro ao derivar chave native segwit para índice %d: %v\n", i, err)
			continue
		}
		nativeAddr, err := generateNativeSegWitAddress(nativeKey, netParams)
		if err != nil {
			fmt.Printf("  Erro ao gerar endereço native segwit para índice %d: %v\n", i, err)
		} else {
			fmt.Printf("  %d: %s\n", i, nativeAddr.EncodeAddress())
		}
	}

	fmt.Println("\nTaproot (BIP86 - P2TR):")
	for i := currentBatchStart; i < currentBatchStart+AddressBatchSize; i++ {
		taprootKey, err := deriveChildKey(masterKey, BIP86Purpose, CoinTypeBitcoin, DefaultAccount, chain, i)
		if err != nil {
			fmt.Printf("  Erro ao derivar chave taproot para índice %d: %v\n", i, err)
			continue
		}
		taprootAddr, err := generateTaprootAddress(taprootKey, netParams)
		if err != nil {
			fmt.Printf("  Erro ao gerar endereço taproot para índice %d: %v\n", i, err)
		} else {
			fmt.Printf("  %d: %s\n", i, taprootAddr.EncodeAddress())
		}
	}
}

func main() {
	fmt.Println("Bitcoin HD Address Generator")
	fmt.Println("=============================")

	reader := bufio.NewReader(os.Stdin)

	// Get xprv from user
	fmt.Print("Digite a chave HD root (xprv): ")
	// Corrected: Added newline delimiter '\n'
	 xprv, _ := reader.ReadString('\n')
	 xprv = strings.TrimSpace(xprv)

	 netParams := &chaincfg.MainNetParams // Assuming Mainnet

	 // Parse the master key from the xprv string
	 masterKey, err := hdkeychain.NewKeyFromString(xprv)
	 if err != nil {
		 fmt.Printf("Erro ao analisar xprv: %v\n", err)
		 return
	 }

	 // Check if the key is private (needed for address derivation)
	 if !masterKey.IsPrivate() {
		 fmt.Println("Erro: A chave fornecida não é uma chave privada estendida (xprv).")
		 return
	 }

	 fmt.Printf("Chave mestra analisada com sucesso. Rede: %s\n", netParams.Name)

	 currentBatchStart := uint32(0)

	 for {
		 fmt.Printf("\n=== Gerando endereços de %d a %d ===\n", currentBatchStart, currentBatchStart+AddressBatchSize-1)

		 // Generate external addresses (receiving)
		 generateAddressBatch(masterKey, netParams, currentBatchStart, ExternalChain, "ENDEREÇOS EXTERNOS (RECEBIMENTO)")
		 
		 // Generate internal addresses (change)
		 generateAddressBatch(masterKey, netParams, currentBatchStart, InternalChain, "ENDEREÇOS INTERNOS (TROCO)")

		 // Ask user if they want the next batch
		 fmt.Printf("\nDeseja gerar os próximos %d endereços? (s/N): ", AddressBatchSize)
		 // Corrected: Added newline delimiter '\n'
		 response, _ := reader.ReadString('\n')
		 response = strings.ToLower(strings.TrimSpace(response))

		 if response != "s" {
			 break // Exit loop if response is not 's'
		 }

		 currentBatchStart += AddressBatchSize // Move to the next batch
	 }

	 fmt.Println("\nEncerrando...")
}

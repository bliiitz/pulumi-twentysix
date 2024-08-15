package basics

import (
	"crypto/ecdsa"
	"errors"
	"log"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	p "github.com/pulumi/pulumi-go-provider"

	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
)

// Each resource has a controlling struct.
// Resource behavior is determined by implementing methods on the controlling struct.
// The `Create` method is mandatory, but other methods are optional.
// - Check: Remap inputs before they are typed.
// - Diff: Change how instances of a resource are compared.
// - Update: Mutate a resource in place.
// - Read: Get the state of a resource from the backing provider.
// - Delete: Custom logic when the resource is deleted.
// - Annotate: Describe fields and set defaults for a resource.
// - WireDependencies: Control how outputs and secrets flows through values.
type TwentySixAccount struct{}

// Each resource has an input struct, defining what arguments it accepts.
type TwentySixAccountArgs struct {
	// Fields projected into Pulumi must be public and hava a `pulumi:"..."` tag.
	// The pulumi tag doesn't need to match the field name, but it's generally a
	// good idea.
	PrivateKey     string `pulumi:"privateKey,optional"`
	Mnemonic       string `pulumi:"mnemonic,optional"`
	DerivationPath string `pulumi:"derivationPath,optional"`
}

// Each resource has a state, describing the fields that exist on the created resource.
type TwentySixAccountState struct {
	// It is generally a good idea to embed args in outputs, but it isn't strictly necessary.
	TwentySixAccountArgs

	Address   string `pulumi:"address"`
	PublicKey string `pulumi:"publicKey"`
}

// All resources must implement Create at a minimum.
func (account TwentySixAccount) Create(ctx p.Context, name string, input TwentySixAccountArgs, preview bool) (string, TwentySixAccountState, error) {
	state := TwentySixAccountState{TwentySixAccountArgs: input}
	if preview {
		return name, state, nil
	}

	if len(state.PrivateKey) > 0 {
		privateKeyBytes, err := hexutil.Decode(input.PrivateKey)
		if err != nil {
			return "", TwentySixAccountState{}, errors.New("error casting public key to bytes")
		}

		privateKey, err := crypto.ToECDSA(privateKeyBytes)
		if err != nil {
			return "", TwentySixAccountState{}, errors.New("error casting public key to ECDSA")
		}

		publicKey := privateKey.Public()

		publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
		if !ok {
			return "", TwentySixAccountState{}, errors.New("error casting public key to ECDSA")
		}

		state.PublicKey = hexutil.Encode(crypto.FromECDSAPub(publicKeyECDSA))
		state.Address = crypto.PubkeyToAddress(*publicKeyECDSA).Hex()

		return name, state, nil
	}

	if len(state.Mnemonic) > 0 {
		wallet, err := hdwallet.NewFromMnemonic(state.Mnemonic)
		if err != nil {
			log.Fatal(err)
		}

		if len(state.DerivationPath) == 0 {
			state.DerivationPath = "m/44'/60'/0'/0/0"
		}

		path := hdwallet.MustParseDerivationPath(state.DerivationPath)
		account, err := wallet.Derive(path, true)
		if err != nil {
			return "", TwentySixAccountState{}, err
		}

		publicKey, err := wallet.PublicKeyBytes(account)
		if err != nil {
			return "", TwentySixAccountState{}, err
		}

		privateKey, err := wallet.PrivateKeyBytes(account)
		if err != nil {
			return "", TwentySixAccountState{}, err
		}

		address, err := wallet.AddressHex(account)
		if err != nil {
			return "", TwentySixAccountState{}, err
		}

		state.PrivateKey = hexutil.Encode(privateKey)
		state.PublicKey = hexutil.Encode(publicKey)
		state.Address = address

		return name, state, nil
	}

	return "", TwentySixAccountState{}, errors.New("no private key or mnemonic provided")
}

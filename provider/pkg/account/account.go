package instance

import (
	"crypto/ecdsa"
	"errors"
	"log"

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
	privateKey     []byte `pulumi:"privateKey,optional"`
	mnemonic       string `pulumi:"mnemonic,optional"`
	derivationPath string `pulumi:"derivationPath,optional"`
}

// Each resource has a state, describing the fields that exist on the created resource.
type TwentySixAccountState struct {
	// It is generally a good idea to embed args in outputs, but it isn't strictly necessary.
	TwentySixAccountArgs

	address   string `pulumi:"address"`
	publicKey []byte `pulumi:"publicKey"`
}

// All resources must implement Create at a minimum.
func (instance TwentySixAccount) Create(ctx p.Context, name string, input TwentySixAccountArgs, preview bool) (string, TwentySixAccountState, error) {
	state := TwentySixAccountState{TwentySixAccountArgs: input}
	if preview {
		return name, state, nil
	}

	if len(state.privateKey) > 0 {
		privateKey := crypto.ToECDSAUnsafe(state.privateKey)
		publicKey := privateKey.Public()

		publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
		if !ok {
			return "", TwentySixAccountState{}, errors.New("error casting public key to ECDSA")
		}

		state.publicKey = crypto.FromECDSAPub(publicKeyECDSA)
		state.address = crypto.PubkeyToAddress(*publicKeyECDSA).Hex()

		return name, state, nil
	}

	if len(state.mnemonic) > 0 {
		wallet, err := hdwallet.NewFromMnemonic(state.mnemonic)
		if err != nil {
			log.Fatal(err)
		}

		if len(state.derivationPath) == 0 {
			state.derivationPath = "m/44'/60'/0'/0/0"
		}

		path := hdwallet.MustParseDerivationPath(state.derivationPath)
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

		state.privateKey = privateKey
		state.publicKey = publicKey
		state.address = address

		return name, state, nil
	}

	return "", TwentySixAccountState{}, errors.New("no private key or mnemonic provided")
}

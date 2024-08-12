package instance

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
type TwentySixInstance struct{}

// Each resource has an input struct, defining what arguments it accepts.
type TwentySixInstanceArgs struct {
	// Fields projected into Pulumi must be public and hava a `pulumi:"..."` tag.
	// The pulumi tag doesn't need to match the field name, but it's generally a
	// good idea.
	
	metadata?: Record<string, unknown>
	variables?: Record<string, string>
	authorized_keys?: string[]
	resources?: Partial<MachineResources>
	requirements?: HostRequirements
	environment?: Partial<FunctionEnvironment>
	image?: string
	volumes?: MachineVolume[]
	storageEngine?: ItemType.ipfs | ItemType.storage
	payment?: Payment
	sync?: boolean
}

// Each resource has a state, describing the fields that exist on the created resource.
type TwentySixInstanceState struct {
	// It is generally a good idea to embed args in outputs, but it isn't strictly necessary.
	TwentySixInstanceArgs

	// Here we define a required output called result.
	Result string `pulumi:"result"`
}

// All resources must implement Create at a minimum.
func (instance TwentySixInstance) Create(ctx p.Context, name string, input TwentySixInstanceArgs, preview bool) (string, TwentySixInstanceState, error) {
	state := TwentySixInstanceState{TwentySixInstanceArgs: input}
	if preview {
		return name, state, nil
	}
	state.Result = makeRandom(input.Length)
	return name, state, nil
}

func makeRandom(length int) string {
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	charset := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") // SED_SKIP

	result := make([]rune, length)
	for i := range result {
		result[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(result)
}

package basics

import (
	"errors"
	"log"
	"reflect"
	"time"

	p "github.com/pulumi/pulumi-go-provider"
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
type TwentySixFunction struct{}

// Each resource has an input struct, defining what arguments it accepts.

type TwentySixFunctionFunctionEnvironment struct {
	Reproducible bool `pulumi:"reproducible"`
	Internet     bool `pulumi:"internet"`
	AlephApi     bool `pulumi:"alephApi"`
	SharedCache  bool `pulumi:"sharedCache"`
}

type TwentySixFunctionMachineResources struct {
	Vcpus   uint64 `pulumi:"vcpus"`
	Memory  uint64 `pulumi:"memory"`
	Seconds uint64 `pulumi:"seconds"`
}

type TwentySixFunctionNodeRequirements struct {
	Owner        string `pulumi:"owner"`
	AddressRegex string `pulumi:"addressRegex"`
}

type TwentySixFunctionCpuProperties struct {
	Architecture CpuArchitecture `pulumi:"architecture"`
	Vendor       CpuVendor       `pulumi:"vendor"`
}

type TwentySixFunctionHostRequirements struct {
	Cpu  CpuProperties    `pulumi:"cpu"`
	Node NodeRequirements `pulumi:"node"`
}

type TwentySixFunctionImmutableVolume struct {
	Comment   []string `pulumi:"comment"`
	Mount     []string `pulumi:"mount"`
	Ref       string   `pulumi:"ref"`
	UseLatest bool     `pulumi:"useLatest"`
}

type TwentySixFunctionEphemeralVolume struct {
	Comment   []string `pulumi:"comment"`
	Mount     []string `pulumi:"mount"`
	Ephemeral bool     `pulumi:"ephemeral"`
	SizeMib   uint64   `pulumi:"sizeMib"` //Limit to 1 GiB
}

type TwentySixFunctionPersistentVolume struct {
	Comment     []string          `pulumi:"comment"`
	Mount       []string          `pulumi:"mount"`
	Parent      ParentVolume      `pulumi:"parent"`
	Persistence VolumePersistence `pulumi:"persistence"`
	Name        string            `pulumi:"name"`
	SizeMib     uint64            `pulumi:"sizeMib"` //Limit to 1 GiB
}

type TwentySixFunctionPayment struct {
	Chain    MessageChain `pulumi:"chain"`
	Receiver string       `pulumi:"receiver,optional"`
	Type     PaymentType  `pulumi:"type"`
}

type TwentySixFunctionParentVolume struct {
	Ref       string `pulumi:"ref"`
	UseLatest bool   `pulumi:"useLatest"`
}

type TwentySixFunctionArgs struct {
	// Fields projected into Pulumi must be public and hava a `pulumi:"..."` tag.
	// The pulumi tag doesn't need to match the field name, but it's generally a
	// good idea.

	Account TwentySixAccountState `pulumi:"account"`
	Channel string                `pulumi:"channel"`

	AllowAmend     bool                                 `pulumi:"allowAmend"`
	Metadata       map[string]string                    `pulumi:"metadata,optional"`
	AuthorizedKeys []string                             `pulumi:"authorizedKeys"`
	Variables      map[string]string                    `pulumi:"variables,optional"`
	Environment    TwentySixFunctionFunctionEnvironment `pulumi:"environment"`
	Resources      TwentySixFunctionMachineResources    `pulumi:"resources"`
	Payment        TwentySixFunctionPayment             `pulumi:"payment"`
	Requirements   TwentySixFunctionHostRequirements    `pulumi:"requirements,optional"`
	Volumes        []interface{}                        `pulumi:"volumes"`
	Replaces       string                               `pulumi:"replaces,optional"`
}

// Each resource has a state, describing the fields that exist on the created resource.
type TwentySixFunctionState struct {
	// It is generally a good idea to embed args in outputs, but it isn't strictly necessary.
	TwentySixFunctionArgs

	SchedulerAllocation SchedulerAllocation `pulumi:"schedulerAllocation"`
	// Here we define a required output called result.
	MessageHash string `pulumi:"messageHash"`
}

// All resources must implement Create at a minimum.
func (volume TwentySixFunction) Create(ctx p.Context, name string, input TwentySixFunctionArgs, preview bool) (string, TwentySixFunctionState, error) {
	state := TwentySixFunctionState{TwentySixFunctionArgs: input}

	//create instance on aleph
	client := NewTwentySixClient(input.Account, state.Channel)
	message, response, err := client.CreateFunction(input)
	if err != nil {
		return "", TwentySixFunctionState{}, err
	}

	if response.Status == RejectedMessageStatus {
		return "", TwentySixFunctionState{}, errors.New("an error occured on function message")
	}

	if response.PublicationStatus.Status != SucceedMessageStatus {
		return "", TwentySixFunctionState{}, errors.New("an error occured on function message")
	}

	state.MessageHash = message.ItemHash

	//wait for instance ready buy checking on scheduler
	instanceAvailable := false

	timeout := int64(1800)
	startAt := time.Now().Unix()
	for !instanceAvailable {
		time.Sleep(10 * time.Second)

		instanceState, err := client.GetInstanceState(message.ItemHash)
		if err != nil {
			log.Println("error on retrieve instance state: ", err.Error())
			now := time.Now().Unix()
			if now > startAt+timeout {
				return "", TwentySixFunctionState{}, errors.New("timeout waiting for instance")
			}
			continue
		}

		state.SchedulerAllocation = instanceState
		instanceAvailable = true
	}

	return name, state, nil
}

func (volume TwentySixFunction) Diff(ctx p.Context, name string, olds TwentySixFunctionState, news TwentySixFunctionArgs) (p.DiffResponse, error) {

	client := NewTwentySixClient(news.Account, news.Channel)

	previous := TwentySixFunctionArgs{
		AllowAmend:     olds.AllowAmend,
		Metadata:       olds.Metadata,
		AuthorizedKeys: olds.AuthorizedKeys,
		Variables:      olds.Variables,
		Environment:    olds.Environment,
		Resources:      olds.Resources,
		Payment:        olds.Payment,
		Requirements:   olds.Requirements,
		Volumes:        olds.Volumes,
		Replaces:       olds.Replaces,
	}

	_, err := client.GetInstanceState(olds.SchedulerAllocation.VmHash)
	instanceStillExists := (err != nil)

	if reflect.DeepEqual(previous, news) && instanceStillExists {
		return p.DiffResponse{
			DeleteBeforeReplace: false,
			HasChanges:          false,
		}, nil
	} else {
		return p.DiffResponse{
			DeleteBeforeReplace: true,
			HasChanges:          true,
		}, nil
	}
}

func (volume TwentySixFunction) Delete(ctx p.Context, name string, olds TwentySixFunctionState) error {

	client := NewTwentySixClient(olds.Account, olds.Channel)
	message, err := client.GetMessageByHash(olds.MessageHash)
	if err != nil {
		if err.Error() == "message not found" {
			return nil
		} else {
			return err
		}
	}

	_, err = client.ForgetMessage(message.ItemHash)
	if err != nil {
		return err
	}

	return nil
}

//update-alternatives --set iptables /usr/sbin/iptables-legacy
//update-alternatives --set ip6tables /usr/sbin/ip6tables-legacy

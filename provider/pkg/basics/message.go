package basics

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

type MessageStatus string
type MessageType string
type MessageChain string
type MessageItemType string
type VolumePersistence string
type PaymentType string
type CpuArchitecture string
type CpuVendor string

const (
	AggregateMessageType MessageType = "AGGREGATE"
	ForgetMessageType    MessageType = "FORGET"
	ProgramMessageType   MessageType = "PROGRAM"
	PostMessageType      MessageType = "POST"
	StoreMessageType     MessageType = "STORE"
	InstanceMessageType  MessageType = "INSTANCE"

	InlineMessageItem  MessageItemType = "inline"
	StorageMessageItem MessageItemType = "storage"
	IpfsMessageItem    MessageItemType = "ipfs"

	PendingMessageStatus   MessageStatus = "pending"
	ProcessedMessageStatus MessageStatus = "processed"
	RejectedMessageStatus  MessageStatus = "rejected"
	ForgottenMessageStatus MessageStatus = "forgotten"

	EthereumChain MessageChain = "ETH"

	HostVolumePersistence  VolumePersistence = "host"
	StoreVolumePersistence VolumePersistence = "store"

	HoldPaymentType       PaymentType = "hold"
	SuperfluidPaymentType PaymentType = "superfluid"

	ArmCpuArchitecture CpuArchitecture = "arm64"
	X64CpuArchitecture CpuArchitecture = "x86_64"

	AmdCpuVendor   CpuArchitecture = "AuthenticAMD"
	IntelCpuVendor CpuArchitecture = "GenuineIntel"
)

type MessageConfirmation struct {
	Chain  MessageChain `json:"chain"`
	Hash   string       `json:"hash"`
	Height uint64       `json:"height"`
}

type Message struct {
	Type      MessageType  `json:"type"`
	Chain     MessageChain `json:"chain"`
	Sender    string       `json:"sender"`
	Time      float64      `json:"time"`
	Channel   string       `json:"channel"`
	Signature string       `json:"signature"`

	ItemHash    string          `json:"item_hash"`
	ItemType    MessageItemType `json:"item_type"`
	ItemContent string          `json:"item_content"`

	Confirmations []MessageConfirmation `json:"confirmations,omitempty"`
	Confirmed     bool                  `json:"confirmed,omitempty"`
}

type GetMessageResponse struct {
	Messages []Message `json:"messages"`

	PaginationPerPage uint64 `json:"pagination_per_page"`
	PaginationPage    uint64 `json:"pagination_page"`
	PaginationTotal   uint64 `json:"pagination_total"`
	PaginationItem    string `json:"pagination_item"`
}

type StoreMessageContent struct {
	Address  string          `json:"address"`
	Time     float64         `json:"time"`
	ItemType MessageItemType `json:"item_type"`
	ItemHash string          `json:"item_hash"`
	Ref      string          `json:"ref,omitempty"`
}

type ForgetMessageContent struct {
	Address string   `json:"address"`
	Time    float64  `json:"time"`
	Hashes  []string `json:"hashes"`
}

type InstanceMessageContent struct {
	Rootfs         RootFsVolume        `json:"rootfs"`
	AllowAmend     bool                `json:"allow_amend"`
	Metadata       map[string]string   `json:"metadata"`
	AuthorizedKeys []string            `json:"authorized_keys"`
	Variables      map[string]string   `json:"variables"`
	Environment    FunctionEnvironment `json:"environment"`
	Resources      MachineResources    `json:"resources"`
	Payment        Payment             `json:"payment"`
	Requirements   HostRequirements    `json:"requirements"`
	Volumes        []interface{}       `json:"volumes"`
	Replaces       string              `json:"replaces"`
}

type FunctionEnvironment struct {
	Reproducible bool `json:"reproducible"`
	Internet     bool `json:"internet"`
	AlephApi     bool `json:"aleph_api"`
	SharedCache  bool `json:"shared_cache"`
}

type MachineResources struct {
	Vcpus   uint64 `json:"vcpus"`
	Memory  uint64 `json:"memory"`
	Seconds uint64 `json:"seconds"`
}

type NodeRequirements struct {
	Owner        string `json:"owner"`
	AddressRegex string `json:"address_regex"`
}

type CpuProperties struct {
	Architecture CpuArchitecture `json:"architecture"`
	Vendor       CpuVendor       `json:"vendor"`
}

type HostRequirements struct {
	Cpu  CpuProperties    `json:"cpu"`
	Node NodeRequirements `json:"node"`
}

type ImmutableVolume struct {
	Comment   []string `json:"comment"`
	Mount     []string `json:"mount"`
	Ref       string   `json:"ref"`
	UseLatest bool     `json:"use_latest"`
}

type EphemeralVolume struct {
	Comment   []string `json:"comment"`
	Mount     []string `json:"mount"`
	Ephemeral bool     `json:"ephemeral"`
	SizeMib   uint64   `json:"size_mib"` //Limit to 1 GiB
}

type PersistentVolume struct {
	Comment     []string          `json:"comment"`
	Mount       []string          `json:"mount"`
	Parent      ParentVolume      `json:"parent"`
	Persistence VolumePersistence `json:"persistence"`
	Name        string            `json:"name"`
	SizeMib     uint64            `json:"size_mib"` //Limit to 1 GiB
}

type Payment struct {
	Chain    MessageChain `json:"chain"`
	Receiver string       `json:"receiver"`
	Type     PaymentType  `json:"Type"`
}

type RootFsVolume struct {
	Parent      ParentVolume      `json:"parent"`
	Persistence VolumePersistence `json:"persistence"`
	SizeMib     uint64            `json:"size_mib"`
}

type ParentVolume struct {
	Ref       string `json:"ref"`
	UseLatest bool   `json:"use_latest"`
}

type SendMessageResponse struct {
	Address  string          `json:"address"`
	Time     float64         `json:"time"`
	ItemType MessageItemType `json:"item_type"`
	ItemHash string          `json:"item_hash"`
	Ref      string          `json:"ref"`
}

type StoreFileMetadata struct {
	Message Message `json:"message"`
	Sync    bool    `json:"sync"`
}

type HashResponse struct {
	Hash string `json:"hash"`
}

type BroadcastRequest struct {
	Message Message `json:"message"`
	Sync    bool    `json:"sync"`
}

type StoreIPFSFileResponse struct {
	Hash   string        `json:"hash"`
	Status MessageStatus `json:"status"`
	Name   string        `json:"name"`
	Size   uint64        `json:"size"`
}

type BroadcastResponse struct {
	Message  Message       `json:"message"`
	Status   MessageStatus `json:"status"`
	Response []byte        `json:"response"`
}

type ForgetMessageResponse struct {
	PublicationStatus struct {
		Status MessageStatus `json:"status"`
		Failed []string      `json:"failed"`
	} `json:"publication_status"`
	Status MessageStatus `json:"message_status"`
}

func (msg Message) getVerificationPayload() []byte {
	//message signing in typescript
	//Buffer.from([this.chain, this.sender, this.type, this.item_hash].join('\n'))

	return []byte(fmt.Sprintf("%s\n%s\n%s\n%s", msg.Chain, msg.Sender, msg.Type, msg.ItemHash))
}

func (msg *Message) SignMessage(pkey string) error {
	messageHash := accounts.TextHash(msg.getVerificationPayload())
	privateKeyBytes, err := hexutil.Decode(pkey)
	if err != nil {
		return err
	}

	key, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return err
	}

	signature, err := crypto.Sign(messageHash, key)
	if err != nil {
		return err
	}

	signature[crypto.RecoveryIDOffset] += 27

	msg.Signature = hexutil.Encode(signature)
	return nil
}

func (msg *Message) JSON() []byte {
	payload, err := json.Marshal(msg)
	if err != nil {
		return []byte("")
	}

	return payload
}

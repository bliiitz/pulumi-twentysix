package basics

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

const AlephApiUrl string = "https://api3.aleph.im"

type TwentySixClient struct {
	account TwentySixAccountState
	channel string

	http http.Client
}

func (client *TwentySixClient) GetMessageByHash(hash string) (Message, error) {

	//https://api2.aleph.im/api/v0/messages.json?hashes=d51f34748974a1e652becd28c28249c2eb5a0cfaf8b718dde7121034d5733981
	messageEndpoint := AlephApiUrl + "/api/v0/messages.json?hashes=" + hash
	request, err := http.NewRequest("GET", messageEndpoint, bytes.NewBuffer([]byte("")))
	if err != nil {
		return Message{}, err
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Accept", "application/json")

	response, err := client.http.Do(request)
	if err != nil {
		return Message{}, err
	}

	resultBody, err := io.ReadAll(response.Body)
	if err != nil {
		return Message{}, err
	}

	var result GetMessageResponse
	if err := json.Unmarshal(resultBody, &result); err != nil { // Parse []byte to go struct pointer
		return Message{}, err
	}

	defer response.Body.Close()

	if result.PaginationTotal != 1 {
		return Message{}, errors.New("message not found")
	} else {
		return result.Messages[0], nil
	}
}

func (client *TwentySixClient) WaitMessageConfirmation(hash string, timeout int64, interval int64) error {
	var startAt int64 = time.Now().Unix()
	var message Message

	message, err := client.GetMessageByHash(hash)
	if err != nil {
		return err
	}

	for !message.Confirmed {
		time.Sleep(time.Duration(interval) * time.Second)

		message, err = client.GetMessageByHash(hash)
		if err != nil {
			return err
		}

		now := time.Now().Unix()
		if now > startAt+timeout {
			return errors.New("message confirmation timeout")
		}
	}

	return nil
}

func (client *TwentySixClient) SendMessage(msgType MessageType, content interface{}) ([]byte, error) {

	msgContent, err := json.Marshal(content)
	if err != nil {
		return []byte{}, err
	}

	contentHash := sha256.Sum256(msgContent)

	message := Message{
		Type:    msgType,
		Chain:   EthereumChain,
		Sender:  client.account.Address,
		Time:    float64(time.Now().Unix()),
		Channel: client.channel,

		ItemHash:    hex.EncodeToString(contentHash[:]),
		ItemType:    IpfsMessageItem,
		ItemContent: string(msgContent),
	}

	message.SignMessage(client.account.PrivateKey)

	req := BroadcastRequest{
		Message: message,
		Sync:    false,
	}

	buff, err := json.Marshal(req)
	if err != nil {
		return []byte{}, err
	}

	storeEndpoint := AlephApiUrl + "/api/v0/messages"
	request, err := http.NewRequest("POST", storeEndpoint, bytes.NewBuffer(buff))
	if err != nil {
		return []byte{}, err
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Accept", "application/json")

	response, err := client.http.Do(request)
	if err != nil {
		return []byte{}, err
	}

	resultBody, err := io.ReadAll(response.Body)
	if err != nil {
		return []byte{}, err
	}

	return resultBody, nil
}

func (client *TwentySixClient) StoreFile(filePath string) (Message, string, error) {
	now := float64(time.Now().UnixMilli()) / 1000
	file, err := os.Open(filePath)
	if err != nil {
		return Message{}, "", err
	}

	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return Message{}, "", err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	//Generate metadata
	metadatapart, err := writer.CreateFormField("metadata")
	if err != nil {
		return Message{}, "", err
	}

	itemContent := StoreMessageContent{
		Address:  client.account.Address,
		Time:     now,
		ItemHash: hex.EncodeToString(hash.Sum(nil)),
		ItemType: StorageMessageItem,
	}

	jsonItem, err := json.Marshal(itemContent)
	if err != nil {
		return Message{}, "", err
	}

	contentHash := sha256.Sum256(jsonItem)

	message := Message{
		Chain:       EthereumChain,
		Sender:      client.account.Address,
		Channel:     client.channel,
		Time:        now,
		Type:        StoreMessageType,
		ItemType:    InlineMessageItem,
		ItemHash:    hex.EncodeToString(contentHash[:]),
		ItemContent: string(jsonItem),
	}

	message.SignMessage(client.account.PrivateKey)

	req := BroadcastRequest{
		Message: message,
		Sync:    false,
	}

	jsonReq, err := json.Marshal(req)
	if err != nil {
		return Message{}, "", err
	}

	metadata := bytes.NewReader(jsonReq)
	io.Copy(metadatapart, metadata)

	//Upload file
	filepart, err := writer.CreateFormFile("file", filepath.Base(file.Name()))
	if err != nil {
		return Message{}, "", err
	}

	file, err = os.Open(filePath)
	if err != nil {
		return Message{}, "", err
	}

	defer file.Close()

	io.Copy(filepart, file)
	writer.Close()

	storeEndpoint := AlephApiUrl + "/api/v0/storage/add_file"
	request, err := http.NewRequest("POST", storeEndpoint, body)
	if err != nil {
		return Message{}, "", err
	}

	request.Header.Add("Content-Type", writer.FormDataContentType())
	request.Header.Add("Accept", "application/json")

	response, err := client.http.Do(request)
	if err != nil {
		return Message{}, "", err
	}

	resultBody, err := io.ReadAll(response.Body)
	if err != nil {
		return Message{}, "", err
	}

	var storeFileResponse StoreIPFSFileResponse
	if err := json.Unmarshal(resultBody, &storeFileResponse); err != nil {
		return Message{}, "", err
	}

	defer response.Body.Close()

	time.Sleep(5 * time.Second)

	createdMessage, err := client.GetVolumeByItemHash(storeFileResponse.Hash)
	if err != nil {
		return Message{}, "", err
	}

	return createdMessage, storeFileResponse.Hash, nil
}

func (client *TwentySixClient) CreateInstance(instance TwentySixInstanceArgs) (Message, MessageResponse, error) {
	now := float64(time.Now().UnixMilli()) / 1000

	instanceMessage := client.instanceArgsToMessage(instance)
	instanceMessage.Time = now
	instanceMessage.Address = client.account.Address

	jsonItem, err := json.Marshal(instanceMessage)
	if err != nil {
		return Message{}, MessageResponse{}, err
	}

	contentHash := sha256.Sum256(jsonItem)

	message := Message{
		Chain:       EthereumChain,
		Sender:      client.account.Address,
		Channel:     client.channel,
		Time:        now,
		Type:        InstanceMessageType,
		ItemType:    InlineMessageItem,
		ItemHash:    hex.EncodeToString(contentHash[:]),
		ItemContent: string(jsonItem),
	}

	message.SignMessage(client.account.PrivateKey)

	req := BroadcastRequest{
		Sync:    false,
		Message: message,
	}

	messageJSON, err := json.Marshal(req)
	if err != nil {
		return Message{}, MessageResponse{}, err
	}

	log.Println("_________________________ instance request _________________________")
	log.Println(string(messageJSON))

	storeEndpoint := AlephApiUrl + "/api/v0/messages"
	request, err := http.NewRequest("POST", storeEndpoint, bytes.NewBuffer(messageJSON))
	if err != nil {
		return Message{}, MessageResponse{}, err
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Accept", "application/json")

	response, err := client.http.Do(request)
	if err != nil {
		return Message{}, MessageResponse{}, err
	}

	resultBody, err := io.ReadAll(response.Body)
	if err != nil {
		return Message{}, MessageResponse{}, err
	}
	log.Println("_________________________ instance response _________________________")
	log.Println(string(resultBody))

	var createInstanceResponse MessageResponse
	if err := json.Unmarshal(resultBody, &createInstanceResponse); err != nil {
		return Message{}, MessageResponse{}, err
	}

	return message, createInstanceResponse, nil
}

func (client *TwentySixClient) CreateFunction(function TwentySixFunctionArgs) (Message, MessageResponse, error) {
	now := float64(time.Now().UnixMilli()) / 1000

	functionMessage := client.functionArgsToMessage(function)
	functionMessage.Time = now
	functionMessage.Address = client.account.Address

	jsonItem, err := json.Marshal(functionMessage)
	if err != nil {
		return Message{}, MessageResponse{}, err
	}

	contentHash := sha256.Sum256(jsonItem)

	message := Message{
		Chain:       EthereumChain,
		Sender:      client.account.Address,
		Channel:     client.channel,
		Time:        now,
		Type:        ProgramMessageType,
		ItemType:    InlineMessageItem,
		ItemHash:    hex.EncodeToString(contentHash[:]),
		ItemContent: string(jsonItem),
	}

	message.SignMessage(client.account.PrivateKey)

	req := BroadcastRequest{
		Sync:    false,
		Message: message,
	}

	messageJSON, err := json.Marshal(req)
	if err != nil {
		return Message{}, MessageResponse{}, err
	}

	log.Println("_________________________ function request _________________________")
	log.Println(string(messageJSON))

	storeEndpoint := AlephApiUrl + "/api/v0/messages"
	request, err := http.NewRequest("POST", storeEndpoint, bytes.NewBuffer(messageJSON))
	if err != nil {
		return Message{}, MessageResponse{}, err
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Accept", "application/json")

	response, err := client.http.Do(request)
	if err != nil {
		return Message{}, MessageResponse{}, err
	}

	resultBody, err := io.ReadAll(response.Body)
	if err != nil {
		return Message{}, MessageResponse{}, err
	}
	log.Println("_________________________ function response _________________________")
	log.Println(string(resultBody))

	var createfunctionResponse MessageResponse
	if err := json.Unmarshal(resultBody, &createfunctionResponse); err != nil {
		return Message{}, MessageResponse{}, err
	}

	return message, createfunctionResponse, nil
}

func (client *TwentySixClient) instanceArgsToMessage(instance TwentySixInstanceArgs) InstanceMessageContent {
	instanceMessage := InstanceMessageContent{
		Rootfs: RootFsVolume{
			Parent: ParentVolume{
				Ref:       instance.Rootfs.Parent.Ref,
				UseLatest: instance.Rootfs.Parent.UseLatest,
			},
			Persistence: instance.Rootfs.Persistence,
			SizeMib:     instance.Rootfs.SizeMib,
		},
		AllowAmend:     instance.AllowAmend,
		Metadata:       instance.Metadata,
		AuthorizedKeys: instance.AuthorizedKeys,
		Variables:      instance.Variables,
		Environment: FunctionEnvironment{
			Reproducible: instance.Environment.Reproducible,
			Internet:     instance.Environment.Internet,
			AlephApi:     instance.Environment.AlephApi,
			SharedCache:  instance.Environment.SharedCache,
		},
		Resources: MachineResources{
			Vcpus:   instance.Resources.Vcpus,
			Memory:  instance.Resources.Memory,
			Seconds: instance.Resources.Seconds,
		},
		Payment: Payment{
			Chain:    instance.Payment.Chain,
			Receiver: instance.Payment.Receiver,
			Type:     instance.Payment.Type,
		},
		// Requirements: HostRequirements{
		// 	Cpu:  instance.Requirements.Cpu,
		// 	Node: instance.Requirements.Node,
		// },
		Volumes:  instance.Volumes,
		Replaces: instance.Replaces,
	}

	return instanceMessage
}

func (client *TwentySixClient) functionArgsToMessage(function TwentySixFunctionArgs) ProgramMessageContent {
	functionMessage := ProgramMessageContent{
		AllowAmend:     function.AllowAmend,
		Metadata:       function.Metadata,
		AuthorizedKeys: function.AuthorizedKeys,
		Variables:      function.Variables,
		Environment: FunctionEnvironment{
			Reproducible: function.Environment.Reproducible,
			Internet:     function.Environment.Internet,
			AlephApi:     function.Environment.AlephApi,
			SharedCache:  function.Environment.SharedCache,
		},
		Resources: MachineResources{
			Vcpus:   function.Resources.Vcpus,
			Memory:  function.Resources.Memory,
			Seconds: function.Resources.Seconds,
		},
		Payment: Payment{
			Chain:    function.Payment.Chain,
			Receiver: function.Payment.Receiver,
			Type:     function.Payment.Type,
		},
		// Requirements: HostRequirements{
		// 	Cpu:  instance.Requirements.Cpu,
		// 	Node: instance.Requirements.Node,
		// },
		Volumes:  function.Volumes,
		Replaces: function.Replaces,
	}

	return functionMessage
}

func (client *TwentySixClient) GetInstanceState(hash string) (SchedulerAllocation, error) {
	body := &bytes.Buffer{}
	endpoint := "https://scheduler.api.aleph.sh/api/v0/allocation/" + hash

	var res SchedulerAllocation

	request, err := http.NewRequest("GET", endpoint, body)
	if err != nil {
		return res, err
	}

	request.Header.Add("Accept", "application/json")

	response, err := client.http.Do(request)
	if err != nil {
		return res, err
	}

	log.Println("status code: " + fmt.Sprint(response.StatusCode))

	resultBody, err := io.ReadAll(response.Body)
	if err != nil {
		return res, err
	}

	log.Println("body: " + string(resultBody))

	if err := json.Unmarshal(resultBody, &res); err != nil {
		return res, err
	}

	return res, nil
}

func (client *TwentySixClient) GetMessages(size uint64, page uint64, hashes []string, addresses []string, channels []string, msgTypes []MessageType) ([]Message, uint64, error) {
	var messages []Message
	body := &bytes.Buffer{}

	messageEndpoint := AlephApiUrl + "/api/v0/messages.json?"

	params := url.Values{}

	params.Add("page", fmt.Sprint(page))
	params.Add("size", fmt.Sprint(size))

	for i := 0; i < len(hashes); i++ {
		params.Add("hashes", hashes[i])
	}
	for i := 0; i < len(addresses); i++ {
		params.Add("addresses", addresses[i])
	}
	for i := 0; i < len(channels); i++ {
		params.Add("channels", channels[i])
	}
	for i := 0; i < len(msgTypes); i++ {
		params.Add("msgTypes", string(msgTypes[i]))
	}

	filteredEndpoint := messageEndpoint + params.Encode()

	request, err := http.NewRequest("GET", filteredEndpoint, body)
	if err != nil {
		return messages, 0, err
	}

	request.Header.Add("Accept", "application/json")

	response, err := client.http.Do(request)
	if err != nil {
		return messages, 0, err
	}

	resultBody, err := io.ReadAll(response.Body)
	if err != nil {
		return messages, 0, err
	}

	var getMessageResponse GetMessageResponse
	if err := json.Unmarshal(resultBody, &getMessageResponse); err != nil {
		return messages, 0, err
	}

	for i := 0; i < len(getMessageResponse.Messages); i++ {
		messages = append(messages, getMessageResponse.Messages[i])
	}

	var remainingItems uint64
	if getMessageResponse.PaginationPage*getMessageResponse.PaginationPerPage > getMessageResponse.PaginationTotal {
		remainingItems = 0
	} else {
		remainingItems = getMessageResponse.PaginationTotal - (getMessageResponse.PaginationPage * getMessageResponse.PaginationPerPage)
	}

	return messages, remainingItems, nil
}

func (client *TwentySixClient) GetVolumes(size uint64, page uint64) ([]Message, uint64, error) {
	return client.GetMessages(size, page, []string{}, []string{client.account.Address}, []string{client.channel}, []MessageType{StoreMessageType})
}

func (client *TwentySixClient) GetVolumeByItemHash(hash string) (Message, error) {
	var page uint64 = 1
	var parsingEnded = false

	for !parsingEnded {
		volumes, remainingItems, err := client.GetVolumes(50, page)
		if err != nil {
			return Message{}, err
		}

		for i := 0; i < len(volumes); i++ {
			var itemContent StoreMessageContent
			json.Unmarshal([]byte(volumes[i].ItemContent), &itemContent)

			if itemContent.ItemHash == hash {
				return volumes[i], nil
			}
		}

		if remainingItems > 0 {
			page += 1
		} else {
			parsingEnded = true
		}
	}

	return Message{}, errors.New("volume not found")
}

func (client *TwentySixClient) ForgetMessage(hash string) (MessageResponse, error) {
	now := float64(time.Now().UnixMilli()) / 1000

	itemContent := ForgetMessageContent{
		Address: client.account.Address,
		Time:    now,
		Hashes:  []string{hash},
	}

	msgContent, err := json.Marshal(itemContent)
	if err != nil {
		return MessageResponse{}, err
	}

	contentHash := sha256.Sum256(msgContent)

	message := Message{
		Type:    ForgetMessageType,
		Chain:   EthereumChain,
		Sender:  client.account.Address,
		Time:    now,
		Channel: client.channel,

		ItemHash:    hex.EncodeToString(contentHash[:]),
		ItemType:    InlineMessageItem,
		ItemContent: string(msgContent),
	}

	message.SignMessage(client.account.PrivateKey)

	req := BroadcastRequest{
		Message: message,
		Sync:    false,
	}

	buff, err := json.Marshal(req)
	if err != nil {
		return MessageResponse{}, err
	}

	storeEndpoint := AlephApiUrl + "/api/v0/messages"
	request, err := http.NewRequest("POST", storeEndpoint, bytes.NewBuffer(buff))
	if err != nil {
		return MessageResponse{}, err
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Accept", "application/json")

	response, err := client.http.Do(request)
	if err != nil {
		return MessageResponse{}, err
	}

	resultBody, err := io.ReadAll(response.Body)
	if err != nil {
		return MessageResponse{}, err
	}

	// response, err := client.SendMessage(ForgetMessageType, itemContent)
	// if err != nil {
	// 	return MessageResponse{}, err
	// }

	var parsedRes MessageResponse
	json.Unmarshal(resultBody, &parsedRes)

	return parsedRes, nil
}

func NewTwentySixClient(acc TwentySixAccountState, channel string) TwentySixClient {
	return TwentySixClient{
		account: acc,
		channel: channel,
		http:    http.Client{},
	}
}

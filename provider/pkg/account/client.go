package account

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const AlephApiUrl string = "https://api2.aleph.im"

type TwentySixClient struct {
	account TwentySixAccountState
	channel string

	http http.Client
}

func (client *TwentySixClient) GetMessageByHash(hash string) (Message, error) {

	//https://api2.aleph.im/api/v0/messages.json?hashes=d51f34748974a1e652becd28c28249c2eb5a0cfaf8b718dde7121034d5733981
	messageEndpoint := AlephApiUrl + "/api/v0/messages.json?hashes=" + hash
	request, err := http.NewRequest("GET", messageEndpoint, bytes.NewBuffer([]byte("{}")))
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
		return result.Messages[1], nil
	}
}

func (client *TwentySixClient) SendMessage(content interface{}) (BroadcastResponse, error) {

	msgContent, err := json.Marshal(content)
	if err != nil {
		return BroadcastResponse{}, err
	}

	contentHash := sha256.Sum256(msgContent)

	message := Message{
		Type:    StoreMessageType,
		Chain:   EthereumChain,
		Sender:  client.account.Address,
		Time:    float64(time.Now().Unix()),
		Channel: client.channel,

		ItemHash:    hex.EncodeToString(contentHash[:]),
		ItemType:    IpfsMessageItem,
		ItemContent: msgContent,
	}

	message.SignMessage(client.account.PrivateKey)

	storeEndpoint := AlephApiUrl + "/api/v0/messages"
	request, err := http.NewRequest("POST", storeEndpoint, bytes.NewBuffer(message.JSON()))
	if err != nil {
		return BroadcastResponse{}, err
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Accept", "application/json")

	response, err := client.http.Do(request)
	if err != nil {
		return BroadcastResponse{}, err
	}

	resultBody, err := io.ReadAll(response.Body)
	if err != nil {
		return BroadcastResponse{}, err
	}

	var broadcastResponse BroadcastResponse
	if err := json.Unmarshal(resultBody, &broadcastResponse); err != nil { // Parse []byte to go struct pointer
		return BroadcastResponse{}, err
	}

	defer response.Body.Close()

	if broadcastResponse.Status == RejectedMessageStatus {
		return BroadcastResponse{}, errors.New("twntysix message rejected")
	}

	return broadcastResponse, nil
}

func (client *TwentySixClient) StoreFile(filePath string) (BroadcastResponse, string, error) {

	file, _ := os.Open(filePath)
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, _ := writer.CreateFormFile("file", filepath.Base(file.Name()))

	io.Copy(part, file)
	writer.Close()

	storeEndpoint := AlephApiUrl + "/api/v0/ipfs/add_file"
	request, err := http.NewRequest("POST", storeEndpoint, body)
	if err != nil {
		return BroadcastResponse{}, "", err
	}

	request.Header.Add("Content-Type", "multipart/form-data")
	request.Header.Add("Accept", "application/json")

	response, err := client.http.Do(request)
	if err != nil {
		return BroadcastResponse{}, "", err
	}

	resultBody, err := io.ReadAll(response.Body)
	if err != nil {
		return BroadcastResponse{}, "", err
	}

	var hashResult HashResponse
	if err := json.Unmarshal(resultBody, &hashResult); err != nil {
		return BroadcastResponse{}, "", err
	}

	defer response.Body.Close()

	messageContent := StoreMessageContent{
		Address:  client.account.Address,
		Time:     float64(time.Now().Unix()),
		ItemHash: hashResult.Hash,
		ItemType: IpfsMessageItem,
	}

	result, err := client.SendMessage(messageContent)
	if err != nil {
		return BroadcastResponse{}, "", err
	}

	return result, hashResult.Hash, nil
}

func (client *TwentySixClient) CreateInstance(filePath string) (string, error) {

	return "", nil
}

func (client *TwentySixClient) ForgetMessage(filePath string) (string, error) {

	return "", nil
}

func NewTwentySixClient(acc TwentySixAccountState, channel string) TwentySixClient {
	return TwentySixClient{
		account: acc,
		channel: channel,
		http:    http.Client{},
	}
}

package basics

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gosimple/hashdir"

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
type TwentySixVolume struct{}

// Each resource has an input struct, defining what arguments it accepts.
type TwentySixVolumeArgs struct {
	// Fields projected into Pulumi must be public and hava a `pulumi:"..."` tag.
	// The pulumi tag doesn't need to match the field name, but it's generally a
	// good idea.

	Account    TwentySixAccountState `pulumi:"account"`
	Channel    string                `pulumi:"channel"`
	FolderPath string                `pulumi:"folderPath"`
	Size       int64                 `pulumi:"size,optional"`
}

// Each resource has a state, describing the fields that exist on the created resource.
type TwentySixVolumeState struct {
	// It is generally a good idea to embed args in outputs, but it isn't strictly necessary.
	TwentySixVolumeArgs

	// Here we define a required output called result.
	FolderHash  string `pulumi:"folderHash"`
	FileHash    string `pulumi:"fileHash"`
	MessageHash string `pulumi:"messageHash"`
}

// All resources must implement Create at a minimum.
func (volume TwentySixVolume) Create(ctx p.Context, name string, input TwentySixVolumeArgs, preview bool) (string, TwentySixVolumeState, error) {
	state := TwentySixVolumeState{TwentySixVolumeArgs: input}
	if preview {
		return name, state, nil
	}

	if state.FolderPath == "" && !folderExists(state.FolderPath) {
		return "", TwentySixVolumeState{}, errors.New("folder dosn't exists")
	}

	dirHash, err := hashdir.Make(state.FolderPath, "sha256")
	if err != nil {
		return "", TwentySixVolumeState{}, err
	}

	filesystemPath := "/tmp/pulumi-squashfs-" + fmt.Sprint(time.Now().Unix()) + ".squashfs"

	// create a new *Cmd instance
	// here we pass the command as the first argument and the arguments to pass to the command as the
	// remaining arguments in the function
	cmd := exec.Command("mksquashfs", state.FolderPath, filesystemPath)

	// The `Output` method executes the command and
	// collects the output, returning its value
	_, err = cmd.Output()
	if err != nil {
		return "", TwentySixVolumeState{}, err
	}

	size, err := FolderSize(filesystemPath)
	if err != nil {
		return "", TwentySixVolumeState{}, err
	}

	state.Size = size

	//store volume on aleph
	client := NewTwentySixClient(input.Account, state.Channel)
	message, fileHash, err := client.StoreFile(filesystemPath)
	os.Remove(filesystemPath)
	if err != nil {
		return "", TwentySixVolumeState{}, err
	}

	state.FolderHash = dirHash
	state.FileHash = fileHash
	state.MessageHash = string(message.ItemHash)

	return name, state, nil
}

func (volume TwentySixVolume) Diff(ctx p.Context, name string, olds TwentySixVolumeState, news TwentySixVolumeArgs) (p.DiffResponse, error) {

	dirHash, err := hashdir.Make(news.FolderPath, "sha256")
	if err != nil {
		return p.DiffResponse{}, err
	}

	client := NewTwentySixClient(news.Account, news.Channel)
	_, err = client.GetMessageByHash(olds.MessageHash)

	if olds.FolderHash == dirHash && err == nil {
		return p.DiffResponse{
			DeleteBeforeReplace: false,
			HasChanges:          false,
		}, nil
	} else {
		return p.DiffResponse{
			DeleteBeforeReplace: err != nil,
			HasChanges:          true,
		}, nil
	}
}

func (volume TwentySixVolume) Delete(ctx p.Context, name string, olds TwentySixVolumeState) error {

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

func folderExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	} else {
		return true
	}
}

func FolderSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}

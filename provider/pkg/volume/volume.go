package volume

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	account "github.com/bliiitz/pulumi-twentysix/provider/pkg/account"
	diskfs "github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/filesystem/squashfs"

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

	account    account.TwentySixAccountState `pulumi:"account"`
	channel    string                        `pulumi:"channel"`
	folderPath string                        `pulumi:"folderPath"`
	size       int64                         `pulumi:"size,optional"`
}

// Each resource has a state, describing the fields that exist on the created resource.
type TwentySixVolumeState struct {
	// It is generally a good idea to embed args in outputs, but it isn't strictly necessary.
	TwentySixVolumeArgs

	// Here we define a required output called result.
	IpfsHash    string `pulumi:"ipfsHash"`
	MessageHash string `pulumi:"messageHash"`
}

// All resources must implement Create at a minimum.
func (volume TwentySixVolume) Create(ctx p.Context, name string, input TwentySixVolumeArgs, preview bool) (string, TwentySixVolumeState, error) {
	state := TwentySixVolumeState{TwentySixVolumeArgs: input}
	if preview {
		return name, state, nil
	}

	if state.folderPath == "" && !folderExists(state.folderPath) {
		log.Fatal("must have a valid path for diskImg")
	}

	filesystemPath := "/tmp/pulumi-squashfs-" + fmt.Sprint(time.Now().Unix()) + ".squashfs"

	err := CreateVolumeFromFolder(state.folderPath, filesystemPath)
	if err != nil {
		return "", TwentySixVolumeState{}, err
	}

	size, err := FolderSize(filesystemPath)
	if err != nil {
		return "", TwentySixVolumeState{}, err
	}

	state.size = size

	//store volume on aleph
	client := account.NewTwentySixClient(input.account, state.channel)
	response, ipfsHash, err := client.StoreFile(filesystemPath)
	if err != nil {
		return "", TwentySixVolumeState{}, err
	}

	state.IpfsHash = ipfsHash
	state.MessageHash = response.Message.ItemHash

	return name, state, nil
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

func CreateVolumeFromFolder(srcFolder string, outputFileName string) error {
	// We need to know the size of the folder before we can create a disk image
	// TODO: Are we able to create a disk image with a dynamic size?
	folderSize, err := FolderSize(srcFolder)
	if err != nil {
		return err
	}

	// TODO: Explain why we need to set the logical block size and which values should be used
	var LogicalBlocksize diskfs.SectorSize = 2048

	// Create the disk image
	// TODO: Explain why we need to use Raw here
	mydisk, err := diskfs.Create(outputFileName, folderSize, diskfs.Raw, LogicalBlocksize)
	if err != nil {
		return err
	}

	// Create the ISO filesystem on the disk image
	fspec := disk.FilesystemSpec{
		Partition:   0,
		FSType:      filesystem.TypeSquashfs,
		VolumeLabel: "label",
	}

	fs, err := mydisk.CreateFilesystem(fspec)
	if err != nil {
		return err
	}

	// Walk the source folder to copy all files and folders to the ISO filesystem
	err = filepath.Walk(srcFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcFolder, path)
		if err != nil {
			return err
		}

		// If the current path is a folder, create the folder in the ISO filesystem
		if info.IsDir() {
			// Create the directory in the ISO file
			err = fs.Mkdir(relPath)
			if err != nil {
				return err
			}

			return nil
		}

		// If the current path is a file, copy the file to the ISO filesystem
		if !info.IsDir() {
			// Open the file in the ISO file for writing
			rw, err := fs.OpenFile(relPath, os.O_CREATE|os.O_RDWR)
			if err != nil {
				return err
			}

			// Open the source file for reading
			in, errorOpeningFile := os.Open(path)
			if errorOpeningFile != nil {
				return errorOpeningFile
			}
			defer in.Close()

			// Copy the contents of the source file to the ISO file
			_, err = io.Copy(rw, in)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	iso, ok := fs.(*squashfs.FileSystem)
	if !ok {
		return fmt.Errorf("not an squashfs filesystem")
	}

	err = iso.Finalize(squashfs.FinalizeOptions{})
	if err != nil {
		return err
	}

	return nil
}

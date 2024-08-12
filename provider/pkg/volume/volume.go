package volume

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"syscall"
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

	account    account.TwentySixAccount `pulumi:"account"`
	folderPath string                   `pulumi:"folderPath"`
	size       int64                    `pulumi:"size,optional"`
}

// Each resource has a state, describing the fields that exist on the created resource.
type TwentySixVolumeState struct {
	// It is generally a good idea to embed args in outputs, but it isn't strictly necessary.
	TwentySixVolumeArgs

	// Here we define a required output called result.
	Result string `pulumi:"result"`
}

// All resources must implement Create at a minimum.
func (instance TwentySixVolume) Create(ctx p.Context, name string, input TwentySixVolumeArgs, preview bool) (string, TwentySixVolumeState, error) {
	state := TwentySixVolumeState{TwentySixVolumeArgs: input}
	if preview {
		return name, state, nil
	}

	if state.folderPath == "" && !folderExists(state.folderPath) {
		log.Fatal("must have a valid path for diskImg")
	}

	filesystemPath := "/tmp/pulumi-squashfs-" + fmt.Sprint(time.Now().Unix())

	mydisk, err := diskfs.Create(filesystemPath, state.size, diskfs.Raw, diskfs.SectorSizeDefault)
	if err != nil {
		return "", TwentySixVolumeState{}, err
	}

	fspec := disk.FilesystemSpec{Partition: 0, FSType: filesystem.TypeSquashfs, VolumeLabel: "label"}

	fs, err := mydisk.CreateFilesystem(fspec)
	if err != nil {
		return "", TwentySixVolumeState{}, err
	}

	rw, err := fs.OpenFile("demo.txt", os.O_CREATE|os.O_RDWR)
	content := []byte("demo")
	_, err = rw.Write(content)

	sqs, ok := fs.(*squashfs.FileSystem)
	if !ok {
		if err != nil {
			return "", TwentySixVolumeState{}, fmt.Errorf("not a squashfs filesystem")
		}
	}

	err = sqs.Finalize(squashfs.FinalizeOptions{})
	if err != nil {
		return "", TwentySixVolumeState{}, err
	}

	return name, state, nil
}

func folderExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	} else {
		return true
	}
}

func CopyDirectory(scrDir string, fs filesystem.FileSystem) error {
	entries, err := os.ReadDir(scrDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(scrDir, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		fileInfo, err := os.Stat(sourcePath)
		if err != nil {
			return err
		}

		stat, ok := fileInfo.Sys().(*syscall.Stat_t)
		if !ok {
			return fmt.Errorf("failed to get raw syscall.Stat_t data for '%s'", sourcePath)
		}

		switch fileInfo.Mode() & os.ModeType {
		case os.ModeDir:
			if err := CreateIfNotExists(destPath, 0755); err != nil {
				return err
			}
			if err := CopyDirectory(sourcePath, destPath); err != nil {
				return err
			}
		case os.ModeSymlink:
			if err := CopySymLink(sourcePath, destPath); err != nil {
				return err
			}
		default:
			if err := Copy(sourcePath, destPath); err != nil {
				return err
			}
		}

		if err := os.Lchown(destPath, int(stat.Uid), int(stat.Gid)); err != nil {
			return err
		}

		fInfo, err := entry.Info()
		if err != nil {
			return err
		}

		isSymlink := fInfo.Mode()&os.ModeSymlink != 0
		if !isSymlink {
			if err := os.Chmod(destPath, fInfo.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}

func Copy(srcFile, dstFile string) error {
	out, err := os.Create(dstFile)
	if err != nil {
		return err
	}

	defer out.Close()

	in, err := os.Open(srcFile)
	if err != nil {
		return err
	}

	defer in.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return nil
}

func Exists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}

	return true
}

func CreateIfNotExists(dir string, perm os.FileMode) error {
	if Exists(dir) {
		return nil
	}

	if err := os.MkdirAll(dir, perm); err != nil {
		return fmt.Errorf("failed to create directory: '%s', error: '%s'", dir, err.Error())
	}

	return nil
}

func CopySymLink(source, dest string) error {
	link, err := os.Readlink(source)
	if err != nil {
		return err
	}
	return os.Symlink(link, dest)
}

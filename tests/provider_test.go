// Copyright 2016-2023, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tests

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	twentysix "github.com/bliiitz/pulumi-twentysix/provider"
)

const (
	debian12Image string = "6e30de68c6cedfa6b45240c2b51e52495ac6fb1bd4b36457b3d5ca307594d595"
	ubuntu22Image string = "77fef271aa6ff9825efa3186ca2e715d19e7108279b817201c69c34cedc74c27"
)

func TestPublishVolume(t *testing.T) {
	prov := provider()

	path, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}

	account, err := prov.Create(p.CreateRequest{
		Urn: urn("twentysix:basics:TwentySixAccount"),
		Properties: resource.PropertyMap{
			"privateKey": resource.NewStringProperty("0x02d64d22b41c5556758303763d39ee5b271832b198e6df28e8bda3295ee7a6c3"),
		},
		Preview: false,
	})

	require.NoError(t, err)

	volume, err := prov.Create(p.CreateRequest{
		Urn: urn("twentysix:basics:TwentySixVolume"),
		Properties: resource.PropertyMap{
			"account":    resource.NewObjectProperty(account.Properties.Copy()),
			"channel":    resource.NewStringProperty("ALEPH-CLOUDSOLUTIONS"),
			"folderPath": resource.NewStringProperty(path + "/../sdk"),
		},
		Preview: false,
	})

	require.NoError(t, err)
	fileHash := volume.Properties["fileHash"].StringValue()
	messageHash := volume.Properties["messageHash"].StringValue()
	assert.Len(t, fileHash, 64)
	assert.Len(t, messageHash, 64)

	time.Sleep(10 * time.Second)

	err = prov.Delete(p.DeleteRequest{
		Urn:        urn("twentysix:basics:TwentySixVolume"),
		Properties: volume.Properties.Copy(),
	})

	require.NoError(t, err)
}

func TestPublishInstance(t *testing.T) {
	prov := provider()

	account, err := prov.Create(p.CreateRequest{
		Urn: urn("twentysix:basics:TwentySixAccount"),
		Properties: resource.PropertyMap{
			"privateKey": resource.NewStringProperty("0x02d64d22b41c5556758303763d39ee5b271832b198e6df28e8bda3295ee7a6c3"),
		},
		Preview: false,
	})

	require.NoError(t, err)

	instance, err := prov.Create(p.CreateRequest{
		Urn: urn("twentysix:basics:TwentySixInstance"),
		Properties: resource.PropertyMap{
			"account": resource.NewObjectProperty(account.Properties.Copy()),
			"channel": resource.NewStringProperty("ALEPH-CLOUDSOLUTIONS"),
			"rootfs": resource.NewObjectProperty(resource.PropertyMap{
				"parent": resource.NewObjectProperty(resource.PropertyMap{
					"ref":       resource.NewStringProperty(debian12Image),
					"useLatest": resource.NewBoolProperty(true),
				}),
				"sizeMib":     resource.NewNumberProperty(20480),
				"persistence": resource.NewStringProperty("host"),
			}),
			"payment": resource.NewObjectProperty(resource.PropertyMap{
				"type":  resource.NewStringProperty("hold"),
				"chain": resource.NewStringProperty("ETH"),
			}),
			"volumes": resource.NewArrayProperty([]resource.PropertyValue{}),
			"metadata": resource.NewObjectProperty(resource.PropertyMap{
				"name": resource.NewStringProperty("pulumi-provider-test"),
			}),
			"resources": resource.NewObjectProperty(resource.PropertyMap{
				"vcpus":   resource.NewNumberProperty(1),
				"memory":  resource.NewNumberProperty(2048),
				"seconds": resource.NewNumberProperty(30),
			}),
			"allowAmend": resource.NewBoolProperty(false),
			"environment": resource.NewObjectProperty(resource.PropertyMap{
				"internet":     resource.NewBoolProperty(true),
				"alephApi":     resource.NewBoolProperty(true),
				"reproducible": resource.NewBoolProperty(false),
				"sharedCache":  resource.NewBoolProperty(false),
			}),
			"authorizedKeys": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDY1+NlRYpV6pjlqdIFiofLWJ7eXSwF4XcSHfG+dJnXxmbx/9gJuczYhewGE7bh9sZWnmXgnzqaGKo6AZ4sjdS0JFqIDGaVbTEzIIlPK3rWhJDagGBB2r6DJZoEHFdTYim7B2DV2j0EuaCARTfhFQWqHlFhNW4IKeZJunKuVY8HAvuyMib8Tth/QoF184wOTY4+HupfY/8qkbA6mng/hmVzvQLKey/INKV2/F9K44XzPuIuy7YztYEzRJ8ayWXK+q4V7m1l+Eh0ryJcdKi8+bPC/cMIzSis1eFUBKeiq3osCqeV/5K4uFDK2cJpJeBPfMdMRmmEJzv4yWYwArl7KNASKhGfDceafNbkLaBMEDHrcERoYl2MpkYAzDEY6ho6YuQPnP9O9j0NxE5HGsGBOWGkCcptqu5WLzMoxfy37O5VmfcDl4kCvkk44hrWHD1/IT2CgcT1VObt9uEyg96LalZMeWP9WfQxwrKso0aV/mpAgIOrptbwMNxEyuN7ScPFHjS91S33oCp7no446fR8x3acL6gbUhkaVllQgiBfVy/YvoY316UXBV/nwMK1JiNosoc5GBabAzBg6DvLQKRcv6oFiZAC6Hu+ngHaIIqe9gZTY2r3BJx5bFwM13+OPDfhKlsW2XnMo5NY7DDhSzv4AU2YTljuE4fNr29MzxbLQ3H8Aw=="),
			}),
		},
		Preview: false,
	})

	require.NoError(t, err)
	messageHash := instance.Properties["messageHash"].StringValue()
	assert.Len(t, messageHash, 64)

	// err = prov.Delete(p.DeleteRequest{
	// 	Urn:        urn("twentysix:basics:TwentySixInstance"),
	// 	Properties: instance.Properties.Copy(),
	// })

	require.NoError(t, err)
}

// urn is a helper function to build an urn for running integration tests.
func urn(typ string) resource.URN {
	return resource.NewURN("stack", "proj", "",
		tokens.Type(typ), "name")
}

// Create a test server.
func provider() integration.Server {
	return integration.NewServer(twentysix.Name, semver.MustParse("1.0.0"), twentysix.Provider())
}

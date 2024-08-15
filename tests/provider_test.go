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

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	twentysix "github.com/bliiitz/pulumi-twentysix/provider"
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
			"privateKey": resource.NewStringProperty(""),
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

	err = prov.Delete(p.DeleteRequest{
		Urn:        urn("twentysix:basics:TwentySixVolume"),
		Properties: volume.Properties.Copy(),
	})

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

// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package apply

import (
	"encoding/json"
	"testing"

	"github.com/GoogleCloudPlatform/healthcare/deploy/terraform"
	"github.com/GoogleCloudPlatform/healthcare/deploy/testconf"
	"github.com/google/go-cmp/cmp"
)

func TestDeployTerraform(t *testing.T) {
	conf, project := testconf.ConfigAndProject(t, nil)
	var gotConfig *terraform.Config
	var gotImports []terraform.Import
	terraformApply = func(config *terraform.Config, _ string, opts *terraform.Options) error {
		gotConfig = config
		if opts != nil {
			gotImports = opts.Imports
		}
		return nil
	}

	if err := deployTerraform(conf, project); err != nil {
		t.Fatalf("deployTerraform: %v", err)
	}

	wantConfig := `{
	"terraform": {
		"required_version": ">= 0.12.0"
	},
	"resource": [{
		"google_storage_bucket": {
			"my-project-state": {
				"name": "my-project-state",
				"project": "my-project",
				"location": "US",
				"versioning": {
					"enabled": true
				}
			}
		}
	}]
}`

	var got, want interface{}
	b, err := json.Marshal(gotConfig)
	if err != nil {
		t.Fatalf("json.Marshal gotConfig: %v", err)
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("json.Unmarshal got = %v", err)
	}
	if err := json.Unmarshal([]byte(wantConfig), &want); err != nil {
		t.Fatalf("json.Unmarshal want = %v", err)
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("terraform config differs (-got, +want):\n%v", diff)
	}

	wantImports := []terraform.Import{{
		Address: "google_storage_bucket.my-project-state",
		ID:      "my-project/my-project-state",
	}}

	if diff := cmp.Diff(gotImports, wantImports); diff != "" {
		t.Errorf("imports differ (-got, +want):\n%v", diff)
	}
}

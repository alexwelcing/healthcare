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
	"os/exec"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/healthcare/deploy/config"
	"github.com/GoogleCloudPlatform/healthcare/deploy/testconf"
	"github.com/google/go-cmp/cmp"
)

func TestGetGCloudCredentials(t *testing.T) {
	region := "foo-center"
	clusterName := "bar-cluster"
	projectID := "foo-project"
	var gotArgs [][]string
	cmdRun = func(cmd *exec.Cmd) error {
		gotArgs = append(gotArgs, cmd.Args)
		return nil
	}
	wantArgs := [][]string{{
		"gcloud", "container", "clusters", "get-credentials", clusterName, "--region", region, "--project", projectID}}
	if err := getGCloudCredentials(clusterName, "--region", region, projectID); err != nil {
		t.Fatalf("getGCloudCredentials error: %v", err)
	}
	if diff := cmp.Diff(gotArgs, wantArgs); len(diff) != 0 {
		t.Fatalf("getGCloudCredentials commands differ: (-got, +want)\n:%v", diff)
	}
}

func TestApplyClusterResource(t *testing.T) {
	containerYamlPath := "foo/bar/abc.yaml"
	var gotArgs [][]string
	cmdRun = func(cmd *exec.Cmd) error {
		gotArgs = append(gotArgs, cmd.Args)
		return nil
	}
	wantArgs := [][]string{{
		"kubectl", "apply", "-f", containerYamlPath}}
	if err := applyClusterWorkload(containerYamlPath); err != nil {
		t.Fatalf("applyClusterWorkload error: %v", err)
	}
	if diff := cmp.Diff(gotArgs, wantArgs); len(diff) != 0 {
		t.Fatalf("applyClusterWorkload commands differ: (-got, +want)\n:%v", diff)
	}
}

func TestLocationTypeAndValue(t *testing.T) {
	testcases := []struct {
		in            config.GKECluster
		locationType  string
		locationValue string
	}{
		{
			in: config.GKECluster{GKEClusterProperties: config.GKEClusterProperties{
				ClusterLocationType: "Regional",
				Region:              "some_region",
				Cluster:             config.GKEClusterSettings{"cluster_with_region"},
			}},
			locationType:  "--region",
			locationValue: "some_region",
		},
		{
			in: config.GKECluster{GKEClusterProperties: config.GKEClusterProperties{
				ClusterLocationType: "Zonal",
				Zone:                "some_zone",
				Cluster:             config.GKEClusterSettings{"cluster_with_zone"},
			}},
			locationType:  "--zone",
			locationValue: "some_zone",
		},
	}

	for _, tc := range testcases {
		locationType, locationValue, err := getLocationTypeAndValue(&tc.in)
		if err != nil {
			t.Errorf("getLocationTypeAndValue error at cluster %q", tc.in.Name())
		}
		if locationType != tc.locationType {
			t.Errorf("getLocationTypeAndValue locationType error at cluster %q: %q", tc.in.Name(), locationType)
		}
		if locationValue != tc.locationValue {
			t.Errorf("getLocationTypeAndValue locationValue error at cluster %q: %q", tc.in.Name(), locationValue)
		}
	}
}

func TestInstallClusterWorkload(t *testing.T) {
	configExtend := &testconf.ConfigData{`
resources:
  gke_clusters:
  - properties:
      name: cluster1
      clusterLocationType: Regional
      region: somewhere1
      cluster:
        name: cluster1
  gke_workloads:
  - cluster_name: cluster1
    properties:
      apiVersion: extensions/v1beta1`,
	}

	wantArgs := [][]string{
		{"gcloud", "container", "clusters", "get-credentials", "cluster1", "--region", "somewhere1", "--project", "my-project"},
		{"kubectl", "apply", "-f"},
	}

	_, project := testconf.ConfigAndProject(t, configExtend)
	var gotArgs [][]string
	cmdRun = func(cmd *exec.Cmd) error {
		gotArgs = append(gotArgs, cmd.Args)
		return nil
	}
	err := deployGKEWorkloads(project)
	if err != nil {
		t.Fatalf("deployGKEWorkloads error: %v", err)
	}
	if len(gotArgs) != 2 {
		t.Fatalf("deployGKEWorkloads does not run correct number of commands: %d", len(gotArgs))
	}
	if diff := cmp.Diff(gotArgs[0], wantArgs[0]); len(diff) != 0 {
		t.Fatalf("get-credentials cmd error: %v", gotArgs[0])
	}
	if diff := cmp.Diff(gotArgs[1][:3], wantArgs[1]); len(diff) != 0 {
		t.Fatalf("kubectl cmd error: %v", gotArgs[1])
	}
}

func TestLocationTypeAndValueError(t *testing.T) {
	testcases := []struct {
		in  config.GKECluster
		err string
	}{
		{
			in: config.GKECluster{GKEClusterProperties: config.GKEClusterProperties{
				ClusterLocationType: "Zonal",
				Region:              "some_region",
				Zone:                "",
				Cluster:             config.GKEClusterSettings{"cluster_zonal_error"},
			}},
			err: "failed to get cluster's zone: cluster_zonal_error",
		},
		{
			in: config.GKECluster{GKEClusterProperties: config.GKEClusterProperties{
				ClusterLocationType: "Regional",
				Zone:                "some_zone",
				Cluster:             config.GKEClusterSettings{"cluster_regional_error"},
			}},
			err: "failed to get cluster's region: cluster_regional_error",
		},
		{
			in: config.GKECluster{GKEClusterProperties: config.GKEClusterProperties{
				ClusterLocationType: "Location",
				Region:              "some_region",
				Zone:                "some_zone",
				Cluster:             config.GKEClusterSettings{"cluster_wrong_type"},
			}},
			err: "failed to get cluster's location: cluster_wrong_type",
		},
	}

	for _, tc := range testcases {
		_, _, err := getLocationTypeAndValue(&tc.in)
		if err == nil || !strings.Contains(err.Error(), tc.err) {
			t.Errorf("getLocationTypeAndValue for cluster %q: got %q, want error with substring %q", tc.in.Name(), err, tc.err)
		}
	}
}

func TestInstallClusterWorkloadErrors(t *testing.T) {
	testcases := []struct {
		in  testconf.ConfigData
		err string
	}{
		{
			in: testconf.ConfigData{`
resources:
  gke_clusters:
  - properties:
      name: cluster1
      clusterLocationType: Regional
      region: somewhere1
      cluster:
        name: cluster1
  gke_workloads:
  - cluster_name: clusterX
    properties:
      apiVersion: extensions/v1beta1`,
			},
			err: "failed to find cluster: \"clusterX\"",
		},
		{
			in: testconf.ConfigData{`
resources:
  gke_clusters:
  - properties:
      name: cluster1
      clusterLocationType: Location
      region: somewhere1
      cluster:
        name: cluster1
  gke_workloads:
  - cluster_name: cluster1
    properties:
      apiVersion: extensions/v1beta1`,
			},
			err: "failed to get cluster's location: cluster1",
		},
	}

	for _, tc := range testcases {
		_, project := testconf.ConfigAndProject(t, &tc.in)
		var gotArgs [][]string
		cmdRun = func(cmd *exec.Cmd) error {
			gotArgs = append(gotArgs, cmd.Args)
			return nil
		}

		err := deployGKEWorkloads(project)
		if err == nil || !strings.Contains(err.Error(), tc.err) {
			t.Errorf("deployGKEWorkloads unexpected error: got %q, want error with substring %q", err, tc.err)
		}
	}
}

func TestImportBinaryAuthorizationPolicy(t *testing.T) {
	configExtend := &testconf.ConfigData{`
binauthz:
  properties: {}`}

	wantArgs := [][]string{
		{"gcloud", "beta", "container", "binauthz", "policy", "import", "foo.json", "--project", "my-project"},
	}

	_, project := testconf.ConfigAndProject(t, configExtend)
	var gotArgs [][]string
	cmdRun = func(cmd *exec.Cmd) error {
		gotArgs = append(gotArgs, cmd.Args)
		return nil
	}
	if project.BinauthzPolicy == nil {
		return
	}

	if err := importBinauthz(project.ID, project.BinauthzPolicy); err != nil {
		t.Fatalf("importBinauthz error: %v", err)
	}
	if len(gotArgs) != 1 {
		t.Fatalf("importBinauthz does not run correct number of commands: %d", len(gotArgs))
	}
	if diff := cmp.Diff(gotArgs[0][:6], wantArgs[0][:6]); diff != "" {
		t.Fatalf("binauthz cmd error between index 0 and 5: %v", gotArgs[0])
	}
	if diff := cmp.Diff(gotArgs[0][7:9], wantArgs[0][7:9]); diff != "" {
		t.Fatalf("binauthz cmd error between index 7 and 8: %v", gotArgs[0])
	}
}

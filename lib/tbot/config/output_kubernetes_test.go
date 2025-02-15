/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import "testing"

func TestKubernetesOutput_YAML(t *testing.T) {
	dest := &DestinationMemory{}
	tests := []testYAMLCase[KubernetesOutput]{
		{
			name: "full",
			in: KubernetesOutput{
				Destination:       dest,
				Roles:             []string{"access"},
				KubernetesCluster: "k8s.example.com",
			},
		},
		{
			name: "minimal",
			in: KubernetesOutput{
				Destination:       dest,
				KubernetesCluster: "k8s.example.com",
			},
		},
	}
	testYAML(t, tests)
}

func TestKubernetesOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testCheckAndSetDefaultsCase[*KubernetesOutput]{
		{
			name: "valid",
			in: func() *KubernetesOutput {
				return &KubernetesOutput{
					Destination:       memoryDestForTest(),
					Roles:             []string{"access"},
					KubernetesCluster: "my-cluster",
				}
			},
		},
		{
			name: "missing destination",
			in: func() *KubernetesOutput {
				return &KubernetesOutput{
					Destination:       nil,
					KubernetesCluster: "my-cluster",
				}
			},
			wantErr: "no destination configured for output",
		},
		{
			name: "missing kubernetes_config",
			in: func() *KubernetesOutput {
				return &KubernetesOutput{
					Destination: memoryDestForTest(),
				}
			},
			wantErr: "kubernetes_cluster must not be empty",
		},
	}
	testCheckAndSetDefaults(t, tests)
}

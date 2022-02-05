//
// Copyright (c) 2019-2022 Red Hat, Inc.
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
//

package automount

import (
	"path"

	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	corev1 "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

func getDevWorkspaceSecrets(namespace string, api sync.ClusterAPI) (*automountResources, error) {
	secrets := &corev1.SecretList{}
	if err := api.Client.List(api.Ctx, secrets, k8sclient.InNamespace(namespace), k8sclient.MatchingLabels{
		constants.DevWorkspaceMountLabel: "true",
	}); err != nil {
		return nil, err
	}
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	var additionalEnvVars []corev1.EnvFromSource
	for _, secret := range secrets.Items {
		mountAs := secret.Annotations[constants.DevWorkspaceMountAsAnnotation]
		if mountAs == "env" {
			additionalEnvVars = append(additionalEnvVars, getAutoMountSecretEnvFromSource(secret.Name))
			continue
		}
		mountPath := secret.Annotations[constants.DevWorkspaceMountPathAnnotation]
		if mountPath == "" {
			mountPath = path.Join("/etc/", "secret/", secret.Name)
		}
		if mountAs == "subpath" {
			volumes = append(volumes, getAutoMountVolumeWithSecret(secret.Name))
			volumeMounts = append(volumeMounts, getAutoMountSecretSubpathVolumeMounts(mountPath, secret)...)
		} else {
			// mountAs == "file", "", or anything else (default). Don't treat invalid values as errors to avoid
			// failing all workspace starts in this namespace
			volumes = append(volumes, getAutoMountVolumeWithSecret(secret.Name))
			volumeMounts = append(volumeMounts, getAutoMountSecretVolumeMount(mountPath, secret.Name))
		}
	}
	return &automountResources{
		Volumes:       volumes,
		VolumeMounts:  volumeMounts,
		EnvFromSource: additionalEnvVars,
	}, nil
}

func getAutoMountVolumeWithSecret(name string) corev1.Volume {
	modeReadOnly := int32(0640)
	workspaceVolumeMount := corev1.Volume{
		Name: common.AutoMountSecretVolumeName(name),
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  name,
				DefaultMode: &modeReadOnly,
			},
		},
	}
	return workspaceVolumeMount
}

func getAutoMountSecretVolumeMount(mountPath, name string) corev1.VolumeMount {
	workspaceVolumeMount := corev1.VolumeMount{
		Name:      common.AutoMountSecretVolumeName(name),
		ReadOnly:  true,
		MountPath: mountPath,
	}
	return workspaceVolumeMount
}

func getAutoMountSecretSubpathVolumeMounts(mountPath string, secret corev1.Secret) []corev1.VolumeMount {
	var workspaceVolumeMounts []corev1.VolumeMount
	for secretKey := range secret.Data {
		workspaceVolumeMounts = append(workspaceVolumeMounts, corev1.VolumeMount{
			Name:      common.AutoMountSecretVolumeName(secret.Name),
			ReadOnly:  true,
			MountPath: path.Join(mountPath, secretKey),
			SubPath:   secretKey,
		})
	}
	return workspaceVolumeMounts
}

func getAutoMountSecretEnvFromSource(name string) corev1.EnvFromSource {
	return corev1.EnvFromSource{
		SecretRef: &corev1.SecretEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: name,
			},
		},
	}
}
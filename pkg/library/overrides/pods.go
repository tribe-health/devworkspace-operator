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

package overrides

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

// NeedsPodOverrides returns whether the current DevWorkspace defines pod overrides via an attribute
// attribute.
func NeedsPodOverrides(workspace *common.DevWorkspaceWithConfig) bool {
	if workspace.Spec.Template.Attributes.Exists(constants.PodOverridesAttribute) {
		return true
	}
	for _, component := range workspace.Spec.Template.Components {
		if component.Attributes.Exists(constants.PodOverridesAttribute) {
			return true
		}
	}
	return false
}

func ApplyPodOverrides(workspace *common.DevWorkspaceWithConfig, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	overrides, err := getPodOverrides(workspace)
	if err != nil {
		return nil, err
	}

	patched := deployment.DeepCopy()
	// Workaround: the definition for corev1.PodSpec does not make containers optional, so even a nil list
	// will be interpreted as "delete all containers" as the serialized patch will include "containers": null.
	// To avoid this, save the original containers and reset them at the end.
	originalContainers := patched.Spec.Template.Spec.Containers
	patchedTemplateBytes, err := json.Marshal(patched.Spec.Template)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal deployment to yaml: %w", err)
	}
	for _, override := range overrides {
		patchBytes, err := json.Marshal(override)
		if err != nil {
			return nil, fmt.Errorf("error applying pod overrides: %w", err)
		}

		patchedTemplateBytes, err = strategicpatch.StrategicMergePatch(patchedTemplateBytes, patchBytes, &corev1.PodTemplateSpec{})
		if err != nil {
			return nil, fmt.Errorf("error applying pod overrides: %w", err)
		}
	}

	patchedPodSpecTemplate := corev1.PodTemplateSpec{}
	if err := json.Unmarshal(patchedTemplateBytes, &patchedPodSpecTemplate); err != nil {
		return nil, fmt.Errorf("error applying pod overrides: %w", err)
	}
	patched.Spec.Template = patchedPodSpecTemplate
	patched.Spec.Template.Spec.Containers = originalContainers
	return patched, nil
}

// getPodOverrides returns PodTemplateSpecOverrides for every instance of the pod overrides attribute
// present in the DevWorkspace. The order of elements is
// 1. Pod overrides defined on Container components, in the order they appear in the DevWorkspace
// 2. Pod overrides defined in the global attributes field (.spec.template.attributes)
func getPodOverrides(workspace *common.DevWorkspaceWithConfig) ([]corev1.PodTemplateSpec, error) {
	var allOverrides []corev1.PodTemplateSpec

	for _, component := range workspace.Spec.Template.Components {
		if component.Attributes.Exists(constants.PodOverridesAttribute) {
			override := corev1.PodTemplateSpec{}
			err := component.Attributes.GetInto(constants.PodOverridesAttribute, &override)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %s attribute on component %s: %w", constants.PodOverridesAttribute, component.Name, err)
			}
			// Do not allow overriding containers
			override.Spec.Containers = nil
			override.Spec.InitContainers = nil
			override.Spec.Volumes = nil
			allOverrides = append(allOverrides, override)
		}
	}
	if workspace.Spec.Template.Attributes.Exists(constants.PodOverridesAttribute) {
		override := corev1.PodTemplateSpec{}
		err := workspace.Spec.Template.Attributes.GetInto(constants.PodOverridesAttribute, &override)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s attribute for workspace: %w", constants.PodOverridesAttribute, err)
		}
		// Do not allow overriding containers or volumes
		override.Spec.Containers = nil
		override.Spec.InitContainers = nil
		override.Spec.Volumes = nil
		allOverrides = append(allOverrides, override)
	}
	return allOverrides, nil
}
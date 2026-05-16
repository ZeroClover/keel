package kubernetes

import (
	"fmt"
	"time"

	"github.com/keel-hq/keel/internal/k8s"
	"github.com/keel-hq/keel/internal/policy"
	"github.com/keel-hq/keel/types"
	"github.com/keel-hq/keel/util/image"

	log "github.com/sirupsen/logrus"
)

func checkForUpdate(plc policy.Policy, filter policy.Filter, repo *types.Repository, triggerName string, resource *k8s.GenericResource) (updatePlan *UpdatePlan, shouldUpdateDeployment bool, err error) {
	updatePlan = &UpdatePlan{}

	eventRepoRef, err := image.Parse(repo.String())
	if err != nil {
		return
	}

	log.WithFields(log.Fields{
		"name":      resource.Name,
		"namespace": resource.Namespace,
		"kind":      resource.Kind(),
		"policy":    policyName(plc),
	}).Debug("provider.kubernetes.checkVersionedDeployment: keel policy found, checking resource...")
	shouldUpdateDeployment = false

	containerFilterFunc := GetMonitorContainersFromMeta(resource.GetAnnotations(), resource.GetLabels())

	if schedule, ok := resource.GetAnnotations()[types.KeelInitContainerAnnotation]; ok && schedule == "true" {
		for idx, c := range resource.InitContainers() {
			if !containerFilterFunc(c) {
				continue
			}
			containerImageRef, err := image.Parse(c.Image)
			if err != nil {
				log.WithFields(log.Fields{
					"error":      err,
					"image_name": c.Image,
				}).Error("provider.kubernetes: failed to parse image name")
				continue
			}

			log.WithFields(log.Fields{
				"name":              resource.Name,
				"namespace":         resource.Namespace,
				"kind":              resource.Kind(),
				"parsed_image_name": containerImageRef.Remote(),
				"target_image_name": repo.Name,
				"target_tag":        repo.Tag,
				"policy":            policyName(plc),
				"image":             c.Image,
			}).Debug("provider.kubernetes: checking image")

			if containerImageRef.Repository() != eventRepoRef.Repository() {
				log.WithFields(log.Fields{
					"parsed_image_name": containerImageRef.Remote(),
					"target_image_name": repo.Name,
				}).Debug("provider.kubernetes: images do not match, ignoring")
				continue
			}

			if containerImageRef.Tag() == eventRepoRef.Tag() {
				continue
			}

			shouldUpdateContainer, err := shouldUpdateFromEvent(plc, filter, containerImageRef.Tag(), eventRepoRef.Tag(), triggerName)
			if err != nil {
				log.WithFields(log.Fields{
					"error":             err,
					"parsed_image_name": containerImageRef.Remote(),
					"target_image_name": repo.Name,
					"policy":            policyName(plc),
				}).Error("provider.kubernetes: failed to check whether init container should be updated")
				continue
			}

			if !shouldUpdateContainer {
				continue
			}

			// updating spec template annotations
			setUpdateTime(resource)

			// updating image
			if containerImageRef.Registry() == image.DefaultRegistryHostname {
				resource.UpdateInitContainer(idx, fmt.Sprintf("%s:%s", containerImageRef.ShortName(), repo.Tag))
			} else {
				resource.UpdateInitContainer(idx, fmt.Sprintf("%s:%s", containerImageRef.Repository(), repo.Tag))
			}

			shouldUpdateDeployment = true

			updatePlan.CurrentVersion = containerImageRef.Tag()
			updatePlan.NewVersion = repo.Tag
			updatePlan.Resource = resource
		}
	}
	for idx, c := range resource.Containers() {
		if !containerFilterFunc(c) {
			continue
		}
		containerImageRef, err := image.Parse(c.Image)
		if err != nil {
			log.WithFields(log.Fields{
				"error":      err,
				"image_name": c.Image,
			}).Error("provider.kubernetes: failed to parse image name")
			continue
		}

		log.WithFields(log.Fields{
			"name":              resource.Name,
			"namespace":         resource.Namespace,
			"kind":              resource.Kind(),
			"parsed_image_name": containerImageRef.Remote(),
			"target_image_name": repo.Name,
			"target_tag":        repo.Tag,
			"policy":            policyName(plc),
			"image":             c.Image,
		}).Debug("provider.kubernetes: checking image")

		if containerImageRef.Repository() != eventRepoRef.Repository() {
			log.WithFields(log.Fields{
				"parsed_image_name": containerImageRef.Remote(),
				"target_image_name": repo.Name,
			}).Debug("provider.kubernetes: images do not match, ignoring")
			continue
		}

		if containerImageRef.Tag() == eventRepoRef.Tag() {
			continue
		}

		shouldUpdateContainer, err := shouldUpdateFromEvent(plc, filter, containerImageRef.Tag(), eventRepoRef.Tag(), triggerName)
		if err != nil {
			log.WithFields(log.Fields{
				"error":             err,
				"parsed_image_name": containerImageRef.Remote(),
				"target_image_name": repo.Name,
				"policy":            policyName(plc),
			}).Error("provider.kubernetes: failed to check whether container should be updated")
			continue
		}

		if !shouldUpdateContainer {
			continue
		}

		// updating spec template annotations
		setUpdateTime(resource)

		// updating image
		if containerImageRef.Registry() == image.DefaultRegistryHostname {
			resource.UpdateContainer(idx, fmt.Sprintf("%s:%s", containerImageRef.ShortName(), repo.Tag))
		} else {
			resource.UpdateContainer(idx, fmt.Sprintf("%s:%s", containerImageRef.Repository(), repo.Tag))
		}

		shouldUpdateDeployment = true

		updatePlan.CurrentVersion = containerImageRef.Tag()
		updatePlan.NewVersion = repo.Tag
		updatePlan.Resource = resource
	}

	return updatePlan, shouldUpdateDeployment, nil
}

func policyName(plc policy.Policy) string {
	if plc == nil {
		return ""
	}
	return plc.Name()
}

func shouldUpdateFromEvent(plc policy.Policy, filter policy.Filter, currentTag, eventTag, triggerName string) (bool, error) {
	if triggerName == types.TriggerTypePoll.String() {
		return true, nil
	}
	return policy.AllowsTag(plc, filter, currentTag, eventTag)
}

func setUpdateTime(resource *k8s.GenericResource) {
	specAnnotations := resource.GetSpecAnnotations()
	specAnnotations[types.KeelUpdateTimeAnnotation] = time.Now().String()
	resource.SetSpecAnnotations(specAnnotations)
}

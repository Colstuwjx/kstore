package store

import (
	"encoding/json"
	"errors"
	"fmt"

	"k8s.io/api/core/v1"

	"github.com/Colstuwjx/kstore/pkg/cache"
)

const (
	DeleteCategory = "delete"
	Deleted        = "deleted"
	NonDeleted     = "non_deleted"
)

// make a factory to produce indexFunc for those regular field category, such as meta, spec, status etc.
// fieldName is a helper while we can NOT determine the key via fieldCategory.
func PodObjectIndexFuncFactory(fieldCategory, fieldName string) cache.IndexFunc {
	// FIXME: return single indexed key right now.
	return func(obj interface{}) ([]string, error) {
		object, ok := obj.(*cache.ObjectDef)
		if !ok {
			return []string{}, errors.New("invalid cache object found")
		}

		if object == nil {
			return []string{}, errors.New("nil cache object found")
		}

		// deleted is another index.
		if fieldCategory == DeleteCategory {
			if object.Deleted {
				return []string{Deleted}, nil
			} else {
				return []string{NonDeleted}, nil
			}
		}

		// FIXME: performance lost here, we need to find out the root casue of `type assertion failed`.
		byteData, _ := json.Marshal(object.Body)
		pod := v1.Pod{}
		err := json.Unmarshal(byteData, &pod)
		if err != nil {
			return []string{}, fmt.Errorf("pod object convert failed, %s", err)
		}

		switch fieldCategory {
		case "metadata.labels":
			if label, exists := pod.Labels[fieldName]; exists {
				return []string{label}, nil
			} else {
				return []string{}, fmt.Errorf("%s has no label %s", pod.Name, fieldName)
			}
		case "metadata.namespace":
			return []string{pod.Namespace}, nil
		case "spec.containers.env":
			for _, c := range pod.Spec.Containers {
				for _, ev := range c.Env {
					if ev.Name == fieldName {
						if ev.Value != "" {
							return []string{ev.Value}, nil
						} else {
							return []string{}, fmt.Errorf("unsupported envFrom %s in %s", ev.Name, pod.Name)
						}
					}
				}
			}

			return []string{}, fmt.Errorf("%s has no env %s", pod.Name, fieldName)
		case "spec.nodeName":
			return []string{pod.Spec.NodeName}, nil
		case "status.podIP":
			return []string{pod.Status.PodIP}, nil
		default:
			return []string{}, fmt.Errorf("field category %s not support yet", fieldCategory)
		}
	}
}

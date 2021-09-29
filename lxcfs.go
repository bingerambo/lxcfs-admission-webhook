package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/golang/glog"
	"k8s.io/api/admission/v1beta1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultAnnotation = "initializer.kubernetes.io/lxcfs"
	defaultNamespace  = "default"
	// custom config
	lxcfsrootName = "lxcfs-root"
	lxcfsrootPath = "/var/lib/lxc"
)

var (
	annotation        string
	configmap         string
	initializerName   string
	namespace         string
	requireAnnotation bool
	// custom config
	HostToContainer  corev1.MountPropagationMode = "HostToContainer"
)

// lxcfs custom config
// -v //var/lib/lxc:/var/lib/lxc:rw

// -v /var/lib/lxcfs/proc/cpuinfo:/proc/cpuinfo:rw
// -v /var/lib/lxcfs/proc/diskstats:/proc/diskstats:rw
// -v /var/lib/lxcfs/proc/meminfo:/proc/meminfo:rw
// -v /var/lib/lxcfs/proc/stat:/proc/stat:rw
// -v /var/lib/lxcfs/proc/swaps:/proc/swaps:rw
// -v /var/lib/lxcfs/proc/uptime:/proc/uptime:rw
// -v /var/lib/lxcfs/proc/loadavg:/proc/loadavg:rw
var volumeMountsTemplate = []corev1.VolumeMount{
	// custom config, add MountPropagation filed
	{
		Name: lxcfsrootName,
		MountPath:lxcfsrootPath,
		MountPropagation: &HostToContainer,
	},

	{
		Name:      "lxcfs-proc-cpuinfo",
		MountPath: "/proc/cpuinfo",
		MountPropagation: &HostToContainer,
	},
	{
		Name:      "lxcfs-proc-meminfo",
		MountPath: "/proc/meminfo",
		MountPropagation: &HostToContainer,
	},
	{
		Name:      "lxcfs-proc-diskstats",
		MountPath: "/proc/diskstats",
		MountPropagation: &HostToContainer,
	},
	{
		Name:      "lxcfs-proc-stat",
		MountPath: "/proc/stat",
		MountPropagation: &HostToContainer,
	},
	{
		Name:      "lxcfs-proc-swaps",
		MountPath: "/proc/swaps",
		MountPropagation: &HostToContainer,
	},
	{
		Name:      "lxcfs-proc-uptime",
		MountPath: "/proc/uptime",
		MountPropagation: &HostToContainer,
	},
	{
		Name:      "lxcfs-proc-loadavg",
		MountPath: "/proc/loadavg",
		MountPropagation: &HostToContainer,
	},
	{
		Name:      "lxcfs-sys-devices-system-cpu-online",
		MountPath: "/sys/devices/system/cpu/online",
		MountPropagation: &HostToContainer,
	},
}
var volumesTemplate = []corev1.Volume{
	{
		Name: lxcfsrootName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: lxcfsrootPath,
			},
		},
	},

	{
		Name: "lxcfs-proc-cpuinfo",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				//Path: "/var/lib/lxcfs/proc/cpuinfo",
				Path: lxcfsrootPath + "/lxcfs/proc/cpuinfo",
			},
		},
	},
	{
		Name: "lxcfs-proc-diskstats",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				//Path: "/var/lib/lxcfs/proc/diskstats",
				Path: lxcfsrootPath + "/lxcfs/proc/diskstats",
			},
		},
	},
	{
		Name: "lxcfs-proc-meminfo",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				//Path: "/var/lib/lxcfs/proc/meminfo",
				Path: lxcfsrootPath + "/lxcfs/proc/meminfo",
			},
		},
	},
	{
		Name: "lxcfs-proc-stat",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				//Path: "/var/lib/lxcfs/proc/stat",
				Path: lxcfsrootPath + "/lxcfs/proc/stat",
			},
		},
	},
	{
		Name: "lxcfs-proc-swaps",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				//Path: "/var/lib/lxcfs/proc/swaps",
				Path: lxcfsrootPath + "/lxcfs/proc/swaps",
			},
		},
	},
	{
		Name: "lxcfs-proc-uptime",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				//Path: "/var/lib/lxcfs/proc/uptime",
				Path: lxcfsrootPath + "/lxcfs/proc/uptime",
			},
		},
	},
	{
		Name: "lxcfs-proc-loadavg",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				//Path: "/var/lib/lxcfs/proc/loadavg",
				Path: lxcfsrootPath + "/lxcfs/proc/loadavg",
			},
		},
	},
	{
		Name: "lxcfs-sys-devices-system-cpu-online",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				//Path: "/var/lib/lxcfs/sys/devices/system/cpu/online",
				Path: lxcfsrootPath + "/lxcfs/sys/devices/system/cpu/online",
			},
		},
	},
}

// main mutation process
func (whsvr *WebhookServer) mutatePod(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	req := ar.Request
	var (
		objectMeta                      *metav1.ObjectMeta
		resourceNamespace, resourceName string
	)

	glog.Infof("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, resourceName, req.UID, req.Operation, req.UserInfo)

	var pod corev1.Pod

	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		glog.Errorf("Could not unmarshal raw object to pod: %v", err)
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}
	resourceName, resourceNamespace, objectMeta = pod.Name, pod.Namespace, &pod.ObjectMeta

	if !mutationRequired(ignoredNamespaces, objectMeta) {
		glog.Infof("Skipping validation for %s/%s due to policy check", resourceNamespace, resourceName)
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	patchBytes, err := createPodPatch(&pod)
	if err != nil {
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	patchType := v1beta1.PatchTypeJSONPatch

	glog.Infof("AdmissionResponse: patch=%v\n", string(patchBytes))
	return &v1beta1.AdmissionResponse{
		UID:       req.UID,
		Allowed:   true,
		Patch:     patchBytes,
		PatchType: &patchType,
	}
}

func createPodPatch(pod *corev1.Pod) ([]byte, error) {

	var patches []patchOperation

	var op = patchOperation{
		Path: "/metadata/annotations",
		Value: map[string]string{
			admissionWebhookAnnotationStatusKey: "mutated",
		},
	}

	if pod.Annotations == nil {
		op.Op = "add"
	} else {
		op.Op = "add"
		if pod.Annotations[admissionWebhookAnnotationStatusKey] != "" {
			op.Op = " replace"
		}
		op.Path = "/metadata/annotations/" + escapeJSONPointerValue(admissionWebhookAnnotationStatusKey)
		op.Value = "mutated"
	}

	patches = append(patches, op)

	containers := pod.Spec.Containers

	// Modify the Pod spec to include the LXCFS volumes, then op the original pod.
	for i := range containers {
		if containers[i].VolumeMounts == nil {
			path := fmt.Sprintf("/spec/containers/%d/volumeMounts", i)
			op = patchOperation{
				Op:    "add",
				Path:  path,
				Value: volumeMountsTemplate,
			}
			patches = append(patches, op)
		} else {
			path := fmt.Sprintf("/spec/containers/%d/volumeMounts/-", i)
			for _, volumeMount := range volumeMountsTemplate {
				op = patchOperation{
					Op:    "add",
					Path:  path,
					Value: volumeMount,
				}
				patches = append(patches, op)
			}
		}
	}

	if pod.Spec.Volumes == nil {
		op = patchOperation{
			Op:    "add",
			Path:  "/spec/volumes",
			Value: volumesTemplate,
		}
		patches = append(patches, op)
	} else {
		for _, volume := range volumesTemplate {
			op = patchOperation{
				Op:    "add",
				Path:  "/spec/volumes/-",
				Value: volume,
			}
			patches = append(patches, op)
		}
	}

	patchBytes, err := json.Marshal(patches)
	if err != nil {
		glog.Warningf("error in json.Marshal %s: %v", pod.Name, err)
		return nil, err
	}
	return patchBytes, nil
}

// validate deployments and services
func (whsvr *WebhookServer) validatePod(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Allowed: true,
	}
}

func escapeJSONPointerValue(in string) string {
	step := strings.Replace(in, "~", "~0", -1)
	return strings.Replace(step, "/", "~1", -1)
}

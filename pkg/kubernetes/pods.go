package kubernetes

import (
	"context"
	"fmt"
	"strings"

	pb "github.com/comrade-coop/trusted-pods/pkg/proto"
	kedahttpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8cl "sigs.k8s.io/controller-runtime/pkg/client"
)

type FetchSecret func(cid []byte) (map[string][]byte, error)

func updateOrCreate(ctx context.Context, ressourceName, kind, namespace string, ressource interface{}, client k8cl.Client, update bool) error {
	if update {
		key := &k8cl.ObjectKey{
			Namespace: namespace,
			Name:      ressourceName,
		}
		oldRessource := GetRessource(kind)

		updatedRessource := ressource.(k8cl.Object)
		updatedRessource.SetNamespace(namespace)
		updatedRessource.SetName(ressourceName)

		err := client.Get(ctx, *key, oldRessource.(k8cl.Object))
		updatedRessource.SetResourceVersion(oldRessource.(k8cl.Object).GetResourceVersion()) // resource version should be retreived from the old ressource in order for httpSo to work
		if err != nil {
			fmt.Printf("Added New Ressource: %v \n", ressourceName)
			if err := client.Create(ctx, updatedRessource); err != nil {
				return err
			}
			return nil
		}

		err = client.Update(ctx, updatedRessource)
		if err != nil {
			return err
		}
		fmt.Printf("Updated %v \n", ressourceName)
		return nil
	}
	if err := client.Create(ctx, ressource.(k8cl.Object)); err != nil {
		return err
	}
	return nil
}

func ApplyPodRequest(
	ctx context.Context,
	client k8cl.Client,
	namespace string,
	update bool,
	podManifest *pb.Pod,
	images map[string]string,
	secrets map[string][]byte,
	response *pb.ProvisionPodResponse,
) error {
	if podManifest == nil {
		return fmt.Errorf("Expected value for pod")
	}

	labels := map[string]string{"tpod": "1"}
	depLabels := map[string]string{}
	activeRessource := []string{}
	startupReplicas := int32(0)
	podId := strings.Split(namespace, "-")[1]
	// NOTE: podId is 52 characters long. Most names are 63 characters max. Thus: don't make any constant string longer than 10 characters. "tpod-dep-" is 9.
	var deploymentName = fmt.Sprintf("tpod-dep-%v", podId)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   deploymentName,
			Labels: depLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &startupReplicas,
			Selector: metav1.SetAsLabelSelector(labels),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
			},
		},
	}

	podTemplate := &deployment.Spec.Template

	httpSoName := fmt.Sprintf("tpod-hso")
	httpSO := NewHttpSo(namespace, httpSoName)

	localhostAliases := corev1.HostAlias{IP: "127.0.0.1"}

	for cIdx, container := range podManifest.Containers {
		containerSpec := corev1.Container{
			Name:       container.Name,
			Image:      images[container.Name],
			Command:    container.Entrypoint,
			Args:       container.Command,
			WorkingDir: container.WorkingDir,
		}
		for field, value := range container.Env {
			containerSpec.Env = append(containerSpec.Env, corev1.EnvVar{Name: field, Value: value})
		}
		for _, port := range container.Ports {
			portName := fmt.Sprintf("p%d-%d", cIdx, port.ContainerPort)
			containerSpec.Ports = append(containerSpec.Ports, corev1.ContainerPort{
				ContainerPort: int32(port.ContainerPort),
				Name:          portName,
			})
			service, servicePort, err := NewService(port, portName, httpSO, labels)
			if err != nil {
				return err
			}

			err = updateOrCreate(ctx, service.GetName(), "Service", namespace, service, client, update)
			if err != nil {
				return err
			}

			activeRessource = append(activeRessource, service.GetName())
			multiaddrPart := ""

			switch port.ExposedPort.(type) {
			case *pb.Container_Port_HostHttpHost:
				httpSO.Spec.ScaleTargetRef.Service = service.ObjectMeta.Name
				httpSO.Spec.ScaleTargetRef.Port = servicePort
				multiaddrPart = fmt.Sprintf("http/%s", httpSO.Spec.Hosts[0])
			case *pb.Container_Port_HostTcpPort:
				multiaddrPart = fmt.Sprintf("tcp/%d", service.Spec.Ports[0].NodePort)
			}
			response.Addresses = append(response.Addresses, &pb.ProvisionPodResponse_ExposedHostPort{
				Multiaddr:     multiaddrPart,
				ContainerName: container.Name,
				ContainerPort: port.ContainerPort,
			})
		}
		for _, volume := range container.Volumes {
			volumeMount := corev1.VolumeMount{
				Name:      volume.Name,
				MountPath: volume.MountPath,
			}
			for _, targetVolume := range podManifest.Volumes {
				if targetVolume.Name == volume.Name {
					if targetVolume.Type == pb.Volume_VOLUME_SECRET {
						volumeMount.SubPath = "data" // NOTE: Change when secrets start supporting filesystems
					}
				}
			}
			containerSpec.VolumeMounts = append(containerSpec.VolumeMounts, volumeMount)
		}
		containerSpec.Resources.Requests = convertResourceList(container.ResourceRequests)
		// TODO: Enforce specifying resources?
		podTemplate.Spec.Containers = append(podTemplate.Spec.Containers, containerSpec)
		localhostAliases.Hostnames = append(localhostAliases.Hostnames, container.Name)
		if depLabels["containers"] == "" {
			depLabels["containers"] = containerSpec.Name
		} else {
			depLabels["containers"] = depLabels["containers"] + "_" + containerSpec.Name
		}
	}
	podTemplate.Spec.HostAliases = append(podTemplate.Spec.HostAliases, localhostAliases)
	for _, volume := range podManifest.Volumes {
		volumeSpec := corev1.Volume{
			Name: volume.Name,
		}
		var volumeName = fmt.Sprintf("tpod-pvc-%v", volume.Name)
		switch volume.Type {
		case pb.Volume_VOLUME_EMPTY:
			volumeSpec.VolumeSource.EmptyDir = &corev1.EmptyDirVolumeSource{}
		case pb.Volume_VOLUME_FILESYSTEM:
			persistentVolumeClaim := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: volumeName,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					Resources: corev1.ResourceRequirements{
						Requests: convertResourceList(volume.GetFilesystem().ResourceRequests),
					},
				},
			}

			switch volume.AccessMode {
			case pb.Volume_VOLUME_RW_ONE:
				persistentVolumeClaim.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
			case pb.Volume_VOLUME_RW_MANY:
				persistentVolumeClaim.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
			}

			// pvcs couldn't and shouldn't be updated
			if !update {
				err := updateOrCreate(ctx, volumeName, "Volume", namespace, persistentVolumeClaim, client, update)
				if err != nil {
					return err
				}

			}
			activeRessource = append(activeRessource, volumeName)

			volumeSpec.VolumeSource.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: persistentVolumeClaim.ObjectMeta.Name,
			}
		case pb.Volume_VOLUME_SECRET:
			var secretName = fmt.Sprintf("tpod-secret-%v", volume.Name)

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: secretName,
				},
				Data: map[string][]byte{
					"data": secrets[volume.Name],
				},
			}

			err := updateOrCreate(ctx, secretName, "Secret", namespace, secret, client, update)
			if err != nil {
				return err
			}
			activeRessource = append(activeRessource, secretName)

			volumeSpec.VolumeSource.Secret = &corev1.SecretVolumeSource{
				SecretName: secret.ObjectMeta.Name,
			}
		}
		podTemplate.Spec.Volumes = append(podTemplate.Spec.Volumes, volumeSpec)
	}
	err := updateOrCreate(ctx, deploymentName, "Deployment", namespace, deployment, client, update)
	if err != nil {
		return err
	}
	activeRessource = append(activeRessource, deploymentName)

	if httpSO.Spec.ScaleTargetRef.Service != "" {
		httpSO.Spec.ScaleTargetRef.Deployment = deployment.ObjectMeta.Name
		minReplicas := int32(podManifest.Replicas.GetMin())
		maxReplicas := int32(podManifest.Replicas.GetMax())
		targetPendingRequests := int32(podManifest.Replicas.GetTargetPendingRequests())
		httpSO.Spec.Replicas = &kedahttpv1alpha1.ReplicaStruct{
			Min: &minReplicas,
		}
		if maxReplicas > 0 {
			httpSO.Spec.Replicas.Max = &maxReplicas
		}
		if targetPendingRequests > 0 {
			httpSO.Spec.TargetPendingRequests = &targetPendingRequests
		}

		err := updateOrCreate(ctx, httpSoName, "HttpSo", namespace, httpSO, client, update)
		if err != nil {
			return err
		}
		activeRessource = append(activeRessource, httpSoName)

	}
	if update == true {
		err := cleanNamespace(ctx, namespace, activeRessource, client)
		if err != nil {
			return err
		}
	}

	return nil
}

func convertResourceList(resources []*pb.Resource) corev1.ResourceList {
	result := make(corev1.ResourceList, len(resources))
	for _, res := range resources {
		var quantity resource.Quantity
		switch q := res.Quantity.(type) {
		case *pb.Resource_Amount:
			quantity = *resource.NewQuantity(int64(q.Amount), resource.BinarySI)
		case *pb.Resource_AmountMillis:
			quantity = *resource.NewMilliQuantity(int64(q.AmountMillis), resource.BinarySI)
		}
		result[corev1.ResourceName(res.Resource)] = quantity
	}
	return result
}

package main

import (
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"time"

	tpk8s "github.com/comrade-coop/trusted-pods/pkg/kubernetes"
	"github.com/comrade-coop/trusted-pods/pkg/prometheus"
	pb "github.com/comrade-coop/trusted-pods/pkg/proto"
	"github.com/comrade-coop/trusted-pods/pkg/resource"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Operations related to monitoring and pricing pod execution",
}

var prometheusUrl string
var pricingFile string
var pricingFileFormat string

var getMetricsCmd = &cobra.Command{
	Use:   "get",
	Short: "Get the metrics stored in prometheus",
	RunE: func(cmd *cobra.Command, args []string) error {
		resourceMeasurements := resource.ResourceMeasurementsMap{}

		err := prometheus.NewPrometheusAPI(prometheusUrl).FetchResourceMetrics(resourceMeasurements)
		if err != nil {
			return err
		}

		if pricingFile != "" {
			file, err := os.Open(pricingFile)
			if err != nil {
				return err
			}

			pricingTableContents, err := io.ReadAll(file)
			if err != nil {
				return err
			}

			Unmarshal := formats[pricingFileFormat]
			if Unmarshal == nil {
				return errors.New("Unknown format: " + pricingFileFormat)
			}

			pricingTable := &pb.PricingTable{}

			err = Unmarshal(pricingTableContents, pricingTable)
			if err != nil {
				return err
			}

			resourceMeasurements.Display(cmd.OutOrStdout(), pricingTable)
			fmt.Fprintf(cmd.OutOrStdout(), "totals: %v\n", resourceMeasurements.Price(pricingTable))
		} else {
			resourceMeasurements.Display(cmd.OutOrStdout(), nil)
		}

		return err
	},
}

type MonitoredState struct {
	Namespace         string
	CurrentQuota      map[*resource.Resource]*big.Float
	CurrentQuotaSince time.Time
}
type MonitoredStates map[types.UID]MonitoredState

func (ms MonitoredState) AddToMap(resourcesMap resource.ResourceMeasurementsMap, time time.Time) {
	seconds := big.NewFloat(time.Sub(ms.CurrentQuotaSince).Seconds())
	for r, q := range ms.CurrentQuota {
		resourcesMap.Add(ms.Namespace, r, (&big.Float{}).Mul(q, seconds))
	}
}

var monitorMetricsCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Monitor pods from in kubernetes to generate metrics directly",
	RunE: func(cmd *cobra.Command, args []string) error {
		var config *rest.Config
		var err error
		if kubeConfig == "-" {
			config, err = rest.InClusterConfig()
		} else {
			config, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
		}
		if err != nil {
			return err
		}
		sch, err := tpk8s.GetScheme()
		if err != nil {
			return err
		}

		cl, err := client.NewWithWatch(config, client.Options{
			Scheme: sch,
		})
		if err != nil {
			return err
		}

		pods := &corev1.PodList{}
		wi, err := cl.Watch(cmd.Context(), pods)
		if err != nil {
			return err
		}

		measuredResources := resource.ResourceMeasurementsMap{}
		monitoredStates := make(MonitoredStates)

		for e := range wi.ResultChan() {
			pod, _ := e.Object.(*corev1.Pod)
			resources := make(map[*resource.Resource]*big.Float)
			for _, c := range pod.Spec.Containers {
				for rn, v := range c.Resources.Requests {
					res := resource.GetResource(rn.String(), resource.ResourceKindReservation)
					valueDec := v.AsDec()
					value := big.NewFloat(0).SetInt(valueDec.UnscaledBig())
					value = value.Mul(value, big.NewFloat(math.Pow10(int(-valueDec.Scale()))))
					if resources[res] == nil {
						resources[res] = value
					} else {
						resources[res] = resources[res].Add(resources[res], value)
					}
				}
			}
			if pod.Status.Phase == corev1.PodRunning {
				runningSince := time.Now()
				for _, pc := range pod.Status.Conditions {
					if pc.Type == corev1.ContainersReady {
						runningSince = pc.LastTransitionTime.Time
					}
				}
				if ms, ok := monitoredStates[pod.UID]; ok {
					if ms.CurrentQuotaSince == runningSince {
						continue
					}
					ms.AddToMap(measuredResources, runningSince)
				}
				monitoredStates[pod.UID] = MonitoredState{
					Namespace:         pod.Namespace,
					CurrentQuota:      resources,
					CurrentQuotaSince: runningSince,
				}
			} else {
				if ms, ok := monitoredStates[pod.UID]; ok {
					stoppedAt := time.Now()
					for _, pc := range pod.Status.Conditions {
						if pc.Type == corev1.DisruptionTarget {
							stoppedAt = pc.LastTransitionTime.Time
						}
					}
					ms.AddToMap(measuredResources, stoppedAt)
					delete(monitoredStates, pod.UID)
				}
			}

			currentTime := time.Now()
			measuredResourcesClone := measuredResources.Clone()
			for _, v := range monitoredStates {
				v.AddToMap(measuredResourcesClone, currentTime)
			}
			measuredResourcesClone.Display(cmd.OutOrStdout(), nil)
		}
		return nil
	},
}

func init() {
	metricsCmd.AddCommand(getMetricsCmd)
	metricsCmd.AddCommand(monitorMetricsCmd)

	getMetricsCmd.Flags().StringVar(&prometheusUrl, "prometheus", "", "address at which the prometheus API can be accessed")
	getMetricsCmd.Flags().StringVar(&pricingFile, "pricing", "", "file containing pricing information")

	formatNames := make([]string, 0, len(formats))
	for name := range formats {
		formatNames = append(formatNames, name)
	}
	getMetricsCmd.Flags().StringVar(&pricingFileFormat, "pricing-format", "json", fmt.Sprintf("pricing file format. one of %v", formatNames))

	defaultKubeConfig := "-"
	if home := homedir.HomeDir(); home != "" {
		defaultKubeConfig = filepath.Join(home, ".kube", "config")
	}
	monitorMetricsCmd.Flags().StringVar(&kubeConfig, "kubeconfig", defaultKubeConfig, "absolute path to the kubeconfig file (- to use in-cluster config)")
}

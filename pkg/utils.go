package config

import (
	"context"
	"fmt"

	applicationv1alpha1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	// ManagedByLabel is the label used to identify resources managed by the operator
	ManagedByLabel       = "giantswarm.io/managed-by"
	OperatorName         = "bundle-operator"
	OperatorAppendSuffix = OperatorName + "-config"
)

type Bundle struct {
	Apps BundleAppConfig `yaml:"bundles"`
}

type BundleAppConfig struct {
	Apps map[string]BundleApp `yaml:"apps"`
}

type BundleApp struct {
	Namespace    string        `yaml:"namespace,omitempty"`
	Enabled      bool          `yaml:"enabled,omitempty"`
	Version      string        `yaml:"version,omitempty"`
	ExtraConfigs []ExtraConfig `yaml:"extraConfigs,omitempty"`
}

func (r *AppReconciler) createOrUpdateConfigmap(app applicationv1alpha1.App, ctx context.Context) (ctrl.Result, error) {
	// Check if the configMap already exists

	var configMap corev1.ConfigMap
	var appName string

	if _, exists := app.Labels[ManagedByLabel]; !exists {
		appName = app.Name
	} else {
		appName = app.Labels[ManagedByLabel]
	}

	configMapNamespacedName := client.ObjectKey{
		Name:      appName + "-" + OperatorAppendSuffix,
		Namespace: app.Namespace,
	}

	if err := r.Get(ctx, configMapNamespacedName, &configMap); err != nil {
		// If it's not found, create it, otherwise exit
		if !apierrors.IsNotFound(err) {
			logf.Log.Error(err, "unable to fetch ConfigMap")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		} else {
			logf.Log.Info(fmt.Sprintf("ConfigMap %s/%s not found, proceeding to create it", configMapNamespacedName.Namespace, configMapNamespacedName.Name))

			// Call creation logic
			newAppConfig := BundleApp{}
			newAppConfig.ExtraConfigs = r.Apps[app.Spec.Name].ExtraConfigs

			newBundleAppConfig := BundleAppConfig{
				Apps: map[string]BundleApp{
					app.Spec.Name: newAppConfig,
				},
			}

			newBundleAppByte, err := yaml.Marshal(newBundleAppConfig)
			if err != nil {
				logf.Log.Error(err, "unable to marshal ConfigMap data")
				return ctrl.Result{}, err
			}

			configMap.Data = map[string]string{
				"values": string(newBundleAppByte),
			}

			configMap.Name = configMapNamespacedName.Name
			configMap.Namespace = configMapNamespacedName.Namespace

			if err := r.Create(ctx, &configMap); err != nil {
				logf.Log.Error(err, "unable to create ConfigMap")
				return ctrl.Result{}, err
			} else {
				logf.Log.Info(fmt.Sprintf("ConfigMap %s/%s created", configMapNamespacedName.Namespace, configMapNamespacedName.Name))

				return ctrl.Result{}, nil
			}
		}
	} else {
		// Comparison logic
		// Unmarshall apps into AppConfig
		var bundleAppConfig BundleAppConfig
		configYaml := configMap.Data["values"]
		err := yaml.Unmarshal([]byte(configYaml), &bundleAppConfig)
		if err != nil {
			logf.Log.Error(err, "unable to unmarshal ConfigMap")
			return ctrl.Result{}, err
		}

		logf.Log.Info(fmt.Sprintf("ConfigMap %s/%s found", configMapNamespacedName.Namespace, configMapNamespacedName.Name))
		originalConfig := configMap.DeepCopy()
		for appName := range bundleAppConfig.Apps {
			// Check if the app exists in the operator config
			if _, exists := r.Apps[appName]; !exists {
				// Remove the app from the ConfigMap
				logf.Log.Info(fmt.Sprintf("App %s not found in operator config, removing it from bundle ConfigMap", appName))
				delete(bundleAppConfig.Apps, appName)
			} else {
				// Update ConfigMap with the operator config
				appConfig := bundleAppConfig.Apps[appName]
				appConfig.ExtraConfigs = r.Apps[appName].ExtraConfigs
				bundleAppConfig.Apps[appName] = appConfig
			}
		}

		// Marshall yaml back to string
		newBundleConfigYamlBytes, err := yaml.Marshal(bundleAppConfig)
		if err != nil {
			logf.Log.Error(err, "unable to marshal ConfigMap data")
			return ctrl.Result{}, err
		}
		configMap.Data["values"] = string(newBundleConfigYamlBytes)

		// Update the ConfigMap
		if err := r.Patch(ctx, &configMap, client.MergeFrom(originalConfig)); err != nil {
			logf.Log.Error(err, "unable to update ConfigMap")
			return ctrl.Result{}, err
		} else {
			logf.Log.Info(fmt.Sprintf("ConfigMap %s/%s updated", configMapNamespacedName.Namespace, configMapNamespacedName.Name))
		}
	}

	return ctrl.Result{}, nil
}

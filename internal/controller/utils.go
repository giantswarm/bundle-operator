package controller

import (
	"context"
	"fmt"

	applicationv1alpha1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	config "github.com/giantswarm/bundle-operator/pkg"
	"github.com/iancoleman/strcase"
	"gopkg.in/yaml.v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
)

var (
	// ManagedByLabel is the label used to identify resources managed by the operator
	ManagedByLabel       = "giantswarm.io/managed-by"
	OperatorName         = "bundle-operator"
	OperatorAppendSuffix = OperatorName + "-config"
)

func (r *AppReconciler) createOrUpdateConfigmap(app applicationv1alpha1.App, ctx context.Context) (ctrl.Result, error) {
	// Check if the configMap already exists

	var configMap corev1.ConfigMap

	appName := app.Name

	configMapNamespacedName := generateConfigMapNamespacedName(appName, app.Namespace)

	// Load expected data
	expectedData, err := initConfigMapData(r.Bundles[strcase.ToLowerCamel(app.Spec.Name)])
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Get(ctx, configMapNamespacedName, &configMap); err != nil {
		// If it's not found, create it, otherwise exit
		if !apierrors.IsNotFound(err) {
			logf.Log.Error(err, "unable to fetch ConfigMap")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		} else {
			logf.Log.Info(fmt.Sprintf("ConfigMap %s/%s not found, proceeding to create it", configMapNamespacedName.Namespace, configMapNamespacedName.Name))

			// Call creation logic
			configMap.Data = expectedData
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

		originalConfig := configMap.DeepCopy()

		if originalConfig.Data["values"] != expectedData["values"] {
			logf.Log.Info(fmt.Sprintf("ConfigMap %s/%s is outdated, proceeding to update", configMapNamespacedName.Namespace, configMapNamespacedName.Name))
			configMap.Data = expectedData

			// Update the ConfigMap
			if err := r.Patch(ctx, &configMap, client.MergeFrom(originalConfig)); err != nil {
				logf.Log.Error(err, "unable to update ConfigMap")
				return ctrl.Result{}, err
			} else {
				logf.Log.Info(fmt.Sprintf("ConfigMap %s/%s updated", configMapNamespacedName.Namespace, configMapNamespacedName.Name))
			}
		}
	}

	return ctrl.Result{}, nil
}

func generateConfigMapNamespacedName(appName, appNamespace string) client.ObjectKey {
	return client.ObjectKey{
		Name:      appName + "-" + OperatorAppendSuffix,
		Namespace: appNamespace,
	}
}

func initConfigMapData(config config.BundleConfig) (map[string]string, error) {
	// Init configmap data
	bundleConfig := map[string]interface{}{
		"apps": config,
	}

	newBundleConfigByte, err := yaml.Marshal(bundleConfig)
	if err != nil {
		logf.Log.Error(err, "unable to marshal ConfigMap data")
		return nil, err
	}

	data := map[string]string{
		"values": string(newBundleConfigByte),
	}

	return data, nil
}

func (r *AppReconciler) updateAppExtraConfigs(app *applicationv1alpha1.App, ctx context.Context) (ctrl.Result, error) {
	// Check if our ConfigMap is already in the App extraConfigs
	configMapName := generateConfigMapNamespacedName(app.Name, app.Namespace).Name
	configPresent := false

	for _, extraConfig := range app.Spec.ExtraConfigs {
		if extraConfig.Name == configMapName {
			// ConfigMap already present, nothing to do
			return ctrl.Result{}, nil
		}
	}
	if !configPresent {
		// Add our ConfigMap to the App extraConfigs

		originalApp := app.DeepCopy()

		app.Spec.ExtraConfigs = append(app.Spec.ExtraConfigs, applicationv1alpha1.AppExtraConfig{
			Name:      configMapName,
			Namespace: app.Namespace,
			Kind:      "configMap",
			Priority:  25,
		})
		// Update the App
		if err := r.Patch(ctx, app, client.MergeFrom(originalApp)); err != nil {
			logf.Log.Error(err, "unable to update App extraConfigs")
			return ctrl.Result{}, err
		} else {
			logf.Log.Info(fmt.Sprintf("App %s/%s extraConfigs updated", app.Namespace, app.Name))
		}
	}

	return ctrl.Result{}, nil
}

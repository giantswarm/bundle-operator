/*
Copyright 2025.

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

package controller

import (
	"context"
	"fmt"

	applicationv1alpha1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	config "github.com/giantswarm/bundle-operator/pkg"
	"github.com/iancoleman/strcase"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// AppReconciler reconciles a App object
type AppReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Bundles map[string]config.BundleConfig
}

func (r *AppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	// Skip if outside org-giantswarm
	if req.Namespace != "org-giantswarm" {
		return ctrl.Result{}, nil
	}

	var app applicationv1alpha1.App

	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		// Check if the Application was deleted
		if apierrors.IsNotFound(err) {
			// Ignore
			return ctrl.Result{}, nil
		}

		logf.Log.Error(err, "unable to fetch Application")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	appName := strcase.ToLowerCamel(app.Spec.Name)

	// Check if the App name is part of the config
	if _, exists := r.Bundles[appName]; exists {
		logf.Log.Info(fmt.Sprintf("App %s/%s found in config, processing", app.Namespace, app.Spec.Name))

		_, err := r.createOrUpdateConfigmap(app, ctx)
		if err != nil {
			logf.Log.Error(err, "unable to create or update ConfigMap")
			return ctrl.Result{}, err
		}

		// Update App extraConfigs if needed
		_, err = r.updateAppExtraConfigs(&app, ctx)
		if err != nil {
			logf.Log.Error(err, "unable to update App extraConfigs")
			return ctrl.Result{}, err
		}

		// Check if it's a legacy security-bundle
		if isLegacySecurityBundle(app) {
			logf.Log.Info(fmt.Sprintf("App %s/%s is a legacy security-bundle, processing migration", app.Namespace, app.Spec.Name))

			app.Spec.Version = "1.15.0"

			if err := r.Update(ctx, &app); err != nil {
				logf.Log.Error(err, "unable to update security-bundle App version")
				return ctrl.Result{}, err
			} else {
				logf.Log.Info(fmt.Sprintf("App %s/%s version updated to 1.15.0", app.Namespace, app.Spec.Name))
			}
		}
	}

	return ctrl.Result{}, nil
}

// isBundledApp checks if the app is part of a bundle, excluding bundles like cluster and cluster-aws
func isBundledApp(app applicationv1alpha1.App) bool {
	if _, exists := app.Annotations["meta.helm.sh/release-name"]; exists {
		// Check if app is part of a bundle
		if _, exists := app.Labels["giantswarm.io/managed-by"]; exists {
			// Check if the release name and the managed by label are the same to exclude cluster matches
			if app.Labels["giantswarm.io/managed-by"] == app.Annotations["meta.helm.sh/release-name"] {
				return true
			}
		}
	}

	return false
}

// handle security-bundle < v1.15.0
func isLegacySecurityBundle(app applicationv1alpha1.App) bool {
	if app.Spec.Name == "security-bundle" {
		if app.Spec.Version < "1.15.0" {
			return true
		}
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Uncomment the following line adding a pointer to an instance of the controlled resource as an argument
		For(&applicationv1alpha1.App{}).
		Complete(r)
}

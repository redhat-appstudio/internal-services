/*
Copyright 2022.

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

package internalrequest

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/redhat-appstudio/internal-services/api/v1alpha1"
	"github.com/redhat-appstudio/internal-services/loader"
	"github.com/redhat-appstudio/internal-services/tekton"
	"github.com/redhat-appstudio/operator-goodies/reconciler"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Adapter holds the objects needed to reconcile a Release.
type Adapter struct {
	client          client.Client
	ctx             context.Context
	internalClient  client.Client
	internalRequest *v1alpha1.InternalRequest
	loader          loader.ObjectLoader
	logger          logr.Logger
}

// NewAdapter creates and returns an Adapter instance.
func NewAdapter(ctx context.Context, client, internalClient client.Client, internalRequest *v1alpha1.InternalRequest, loader loader.ObjectLoader, logger logr.Logger) *Adapter {
	return &Adapter{
		client:          client,
		ctx:             ctx,
		internalRequest: internalRequest,
		internalClient:  internalClient,
		loader:          loader,
		logger:          logger,
	}
}

func (a *Adapter) EnsureRequestIsHandled() (reconciler.OperationResult, error) {
	pipelineRun, err := a.loader.GetInternalRequestPipelineRun(a.ctx, a.internalClient, a.internalRequest)
	if err != nil && !errors.IsNotFound(err) {
		return reconciler.RequeueWithError(err)
	}

	if pipelineRun == nil || !a.internalRequest.HasStarted() {
		if pipelineRun == nil {
			pipelineRun := tekton.NewPipelineRun(a.internalRequest)
			err := a.internalClient.Create(a.ctx, pipelineRun)
			if err != nil {
				return reconciler.RequeueWithError(err)
			}

			a.logger.Info("Created internal request PipelineRun",
				"PipelineRun.Name", pipelineRun.Name, "PipelineRun.Namespace", pipelineRun.Namespace)
		}

		return reconciler.RequeueOnErrorOrContinue(a.registerInternalRequestStatus(pipelineRun))
	}

	return reconciler.ContinueProcessing()
}

func (a *Adapter) EnsureStatusIsTracked() (reconciler.OperationResult, error) {
	pipelineRun, err := a.loader.GetInternalRequestPipelineRun(a.ctx, a.internalClient, a.internalRequest)
	if err != nil && !errors.IsNotFound(err) {
		return reconciler.RequeueWithError(err)
	}

	if pipelineRun != nil {
		return reconciler.RequeueOnErrorOrContinue(a.registerInternalRequestPipelineRunStatus(pipelineRun))
	}

	return reconciler.ContinueProcessing()
}

func (a *Adapter) registerInternalRequestStatus(pipelineRun *v1beta1.PipelineRun) error {
	if pipelineRun == nil {
		return nil
	}

	patch := client.MergeFrom(a.internalRequest.DeepCopy())

	a.internalRequest.Status.PipelineRun = pipelineRun.Name
	a.internalRequest.MarkRunning()

	return a.client.Status().Patch(a.ctx, a.internalRequest, patch)
}

func (a *Adapter) registerInternalRequestPipelineRunStatus(pipelineRun *v1beta1.PipelineRun) error {
	if pipelineRun != nil && pipelineRun.IsDone() {
		patch := client.MergeFrom(a.internalRequest.DeepCopy())

		a.internalRequest.Status.Results = pipelineRun.Status.PipelineResults

		condition := pipelineRun.Status.GetCondition(apis.ConditionSucceeded)
		if condition.IsTrue() {
			a.internalRequest.MarkSucceeded()
		} else {
			a.internalRequest.MarkFailed(condition.Message)
		}

		return a.client.Status().Patch(a.ctx, a.internalRequest, patch)
	}

	return nil
}

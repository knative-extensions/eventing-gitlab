/*
Copyright 2021 The Knative Authors

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

package gitlab

import (
	"context"

	"github.com/kelseyhightower/envconfig"

	"k8s.io/client-go/tools/cache"

	"knative.dev/eventing/pkg/reconciler/source"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/resolver"
	servingclient "knative.dev/serving/pkg/client/injection/client"
	serviceinformerv1 "knative.dev/serving/pkg/client/injection/informers/serving/v1/service"

	"knative.dev/eventing-gitlab/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-gitlab/pkg/client/gitlab"
	informerv1alpha1 "knative.dev/eventing-gitlab/pkg/client/injection/informers/sources/v1alpha1/gitlabsource"
	reconcilerv1alpha1 "knative.dev/eventing-gitlab/pkg/client/injection/reconciler/sources/v1alpha1/gitlabsource"
)

type envConfig struct {
	Image string `envconfig:"GL_RA_IMAGE" required:"true"`
}

// NewController returns the controller implementation with reconciler structure and logger
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	env := &envConfig{}
	envconfig.MustProcess("", env)

	serviceInformer := serviceinformerv1.Get(ctx)

	r := &Reconciler{
		gitlabCg:            gitlab.NewWebhookClientGetter(kubeclient.Get(ctx).CoreV1().Secrets),
		ksvcCli:             servingclient.Get(ctx).ServingV1().Services,
		ksvcLister:          serviceInformer.Lister(),
		receiveAdapterImage: env.Image,
		loggingContext:      ctx,
		configs:             source.WatchConfigurations(ctx, "gitlab-controller", cmw),
	}

	impl := reconcilerv1alpha1.NewImpl(ctx, r)
	r.sinkResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	logging.FromContext(ctx).Info("Setting up event handlers")

	informerv1alpha1.Get(ctx).Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	serviceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGVK(v1alpha1.SchemeGroupVersion.WithKind("GitLabSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl

}

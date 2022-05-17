module knative.dev/eventing-gitlab

go 1.16

require (
	github.com/cloudevents/sdk-go/v2 v2.8.0
	github.com/emicklei/go-restful v2.15.0+incompatible // indirect
	github.com/google/go-cmp v0.5.7
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/stretchr/testify v1.7.0
	github.com/xanzy/go-gitlab v0.39.0
	go.uber.org/zap v1.19.1
	gopkg.in/go-playground/webhooks.v5 v5.15.0
	k8s.io/api v0.23.5
	k8s.io/apimachinery v0.23.5
	k8s.io/client-go v0.23.5
	knative.dev/eventing v0.31.1-0.20220516052256-d7a8a95792bd
	knative.dev/hack v0.0.0-20220512014059-f4972b4daff9
	knative.dev/pkg v0.0.0-20220512013937-2d8305b2e59a
	knative.dev/serving v0.31.1-0.20220516213257-08e09bc9547a
)

replace github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2

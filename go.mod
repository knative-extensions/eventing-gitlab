module knative.dev/eventing-gitlab

go 1.15

require (
	github.com/cloudevents/sdk-go/v2 v2.4.1
	github.com/google/go-cmp v0.5.6
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/stretchr/testify v1.7.0
	github.com/xanzy/go-gitlab v0.39.0
	go.uber.org/zap v1.17.0
	gopkg.in/go-playground/webhooks.v5 v5.15.0
	k8s.io/api v0.20.7
	k8s.io/apimachinery v0.20.7
	k8s.io/client-go v0.20.7
	knative.dev/eventing v0.24.1-0.20210708130023-221dfdfced62
	knative.dev/hack v0.0.0-20210622141627-e28525d8d260
	knative.dev/pkg v0.0.0-20210708145023-4a3e56dc13b2
	knative.dev/serving v0.24.1-0.20210708194119-7a3e9d03b0e4
)

replace github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2

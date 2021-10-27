module knative.dev/eventing-gitlab

go 1.16

require (
	github.com/cloudevents/sdk-go/v2 v2.4.1
	github.com/google/go-cmp v0.5.6
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/stretchr/testify v1.7.0
	github.com/xanzy/go-gitlab v0.39.0
	go.uber.org/zap v1.19.1
	gopkg.in/go-playground/webhooks.v5 v5.15.0
	k8s.io/api v0.21.4
	k8s.io/apimachinery v0.21.4
	k8s.io/client-go v0.21.4
	knative.dev/eventing v0.26.1-0.20211027064300-a81d7ba31082
	knative.dev/hack v0.0.0-20211026141922-a71c865b5f66
	knative.dev/pkg v0.0.0-20211027105800-3b33e02e5b9c
	knative.dev/serving v0.26.1-0.20211026205200-8f02c277d0b3
)

replace github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2

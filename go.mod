module knative.dev/eventing-gitlab

go 1.15

require (
	github.com/cloudevents/sdk-go/v2 v2.2.0
	github.com/google/go-cmp v0.5.4
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/stretchr/testify v1.6.1
	github.com/xanzy/go-gitlab v0.39.0
	go.uber.org/zap v1.16.0
	gopkg.in/go-playground/webhooks.v5 v5.15.0
	k8s.io/api v0.19.7
	k8s.io/apimachinery v0.19.7
	k8s.io/client-go v0.19.7
	knative.dev/eventing v0.20.1-0.20210128132430-1725902f7e39
	knative.dev/hack v0.0.0-20210120165453-8d623a0af457
	knative.dev/pkg v0.0.0-20210127163530-0d31134d5f4e
	knative.dev/serving v0.20.1-0.20210128035538-fbbbb98dc9a6
)

replace github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2

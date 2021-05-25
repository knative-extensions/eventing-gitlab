module knative.dev/eventing-gitlab

go 1.15

require (
	github.com/cloudevents/sdk-go/v2 v2.4.1
	github.com/google/go-cmp v0.5.5
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/stretchr/testify v1.7.0
	github.com/xanzy/go-gitlab v0.39.0
	go.uber.org/zap v1.16.0
	gopkg.in/go-playground/webhooks.v5 v5.15.0
	k8s.io/api v0.19.7
	k8s.io/apimachinery v0.19.7
	k8s.io/client-go v0.19.7
	knative.dev/eventing v0.23.1-0.20210524085620-e48c80d7f969
	knative.dev/hack v0.0.0-20210428122153-93ad9129c268
	knative.dev/pkg v0.0.0-20210525071438-e2947dba201a
	knative.dev/serving v0.23.1-0.20210525132337-efbc518aa54e
)

replace github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2

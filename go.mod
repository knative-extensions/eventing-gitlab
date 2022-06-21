module knative.dev/eventing-gitlab

go 1.16

require (
	github.com/cloudevents/sdk-go/v2 v2.10.1
	github.com/emicklei/go-restful v2.15.0+incompatible // indirect
	github.com/google/go-cmp v0.5.7
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/stretchr/testify v1.7.0
	github.com/xanzy/go-gitlab v0.39.0
	go.uber.org/zap v1.21.0
	gopkg.in/go-playground/webhooks.v5 v5.15.0
	k8s.io/api v0.23.5
	k8s.io/apimachinery v0.23.5
	k8s.io/client-go v0.23.5
	knative.dev/eventing v0.32.1-0.20220614135415-c1b0ea692715
	knative.dev/hack v0.0.0-20220610014127-dc6c287516dc
	knative.dev/pkg v0.0.0-20220610014025-7d607d643ee2
	knative.dev/serving v0.32.1-0.20220620180920-5f8892f5a720
)

replace github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2

module knative.dev/eventing-gitlab

go 1.16

require (
	github.com/cloudevents/sdk-go/v2 v2.8.0
	github.com/emicklei/go-restful v2.15.0+incompatible // indirect
	github.com/google/go-cmp v0.5.6
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/stretchr/testify v1.7.0
	github.com/xanzy/go-gitlab v0.39.0
	go.uber.org/zap v1.19.1
	gopkg.in/go-playground/webhooks.v5 v5.15.0
	k8s.io/api v0.22.5
	k8s.io/apimachinery v0.22.5
	k8s.io/client-go v0.22.5
	knative.dev/eventing v0.29.1-0.20220203104220-17487954d41a
	knative.dev/hack v0.0.0-20220201013531-82bfca153560
	knative.dev/pkg v0.0.0-20220203020920-51be315ed160
	knative.dev/serving v0.29.1-0.20220203055220-1d9529495e17
)

replace github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2

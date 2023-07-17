module github.com/hashicorp/nomad-autoscaler

go 1.20

require (
	github.com/Azure/azure-sdk-for-go v64.1.0+incompatible
	github.com/Azure/go-autorest/autorest v0.11.27
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.11
	github.com/Azure/go-autorest/autorest/date v0.3.0
	github.com/DataDog/datadog-api-client-go v1.14.0
	github.com/armon/go-metrics v0.3.11
	github.com/aws/aws-sdk-go-v2 v1.19.0
	github.com/aws/aws-sdk-go-v2/config v1.18.28
	github.com/aws/aws-sdk-go-v2/service/autoscaling v1.23.2
	github.com/golang/protobuf v1.5.3
	github.com/google/go-cmp v0.5.9
	github.com/hashicorp/go-hclog v0.16.0
	github.com/hashicorp/go-msgpack v1.1.5
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/go-plugin v1.0.1
	github.com/hashicorp/hcl/v2 v2.10.0
	github.com/hashicorp/nomad/api v0.0.0-20230505125014-3d63bc62b35c
	github.com/mitchellh/cli v1.1.2
	github.com/mitchellh/copystructure v1.2.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/prometheus/client_golang v1.16.0
	github.com/prometheus/common v0.44.0
	github.com/shoenig/test v0.6.6
	github.com/stretchr/testify v1.8.1
	golang.org/x/text v0.9.0
	google.golang.org/api v0.103.0
	google.golang.org/grpc v1.53.0
	google.golang.org/protobuf v1.30.0
)

require (
	cloud.google.com/go/compute v1.15.1 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.19 // indirect
	github.com/Azure/go-autorest/autorest/azure/cli v0.4.5 // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.0 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/DataDog/datadog-go v3.6.0+incompatible // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/apparentlymart/go-textseg/v13 v13.0.0 // indirect
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.13.27
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.13.5 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.35 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.29 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.36 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.29 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.12.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.14.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.19.3 // indirect
	github.com/aws/smithy-go v1.13.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bgentry/speakeasy v0.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/circonus-labs/circonus-gometrics v2.3.1+incompatible // indirect
	github.com/circonus-labs/circonusllhist v0.1.3 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/fatih/color v1.10.0 // indirect
	github.com/golang-jwt/jwt/v4 v4.4.1 // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.2.0 // indirect
	github.com/googleapis/gax-go/v2 v2.7.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/hashicorp/cronexpr v1.1.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.0 // indirect
	github.com/hashicorp/go-retryablehttp v0.5.3 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-uuid v1.0.1 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/yamux v0.0.0-20180604194846-3520598351bb // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-colorable v0.1.8 // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mitchellh/go-testing-interface v1.14.1 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/oklog/run v1.0.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/posener/complete v1.2.3 // indirect
	github.com/prometheus/client_model v0.4.0 // indirect
	github.com/prometheus/procfs v0.10.1 // indirect
	github.com/tv42/httpunix v0.0.0-20150427012821-b75d8614f926 // indirect
	github.com/zclconf/go-cty v1.8.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	golang.org/x/crypto v0.1.0 // indirect
	golang.org/x/exp v0.0.0-20230108222341-4b8118a2686a // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/oauth2 v0.8.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230110181048-76db0878b65f // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

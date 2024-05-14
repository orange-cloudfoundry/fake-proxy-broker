package main

import (
	"code.cloudfoundry.org/lager"
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"github.com/cloudfoundry-community/gautocloud"
	"github.com/cloudfoundry-community/gautocloud/connectors/generic"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/brokerapi/domain"
	"github.com/satori/go.uuid"
	"net/http"
	"net/url"
	"os"
)

const (
	ROOT_UUID = "aaa4b55e-5768-41ea-a383-5d633725b88b"
)

func init() {
	gautocloud.RegisterConnector(generic.NewConfigGenericConnector(FakeBrokerConfig{}))
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Port     int    `json:"port"`
	Host     string `json:"host"`
	Protocol string `json:"protocol"`
	Uri      string `json:"uri"`
}

type ProxyConfig struct {
	Name             string `cloud:"name"`
	Description      string `cloud:"description"`
	Host             string `cloud:"host"`
	Port             int    `cloud:"port" cloud-default:"3128"`
	Protocol         string `cloud:"protocol" cloud-default:"http"`
	RandomSubdomain  bool   `cloud:"random_subdomain"`
	RandomUser       bool   `cloud:"random_user"`
	SupportURL       string `cloud:"support_url"`
	DocumentationURL string `cloud:"documentation_url"`
	ImagePath        string `cloud:"image_path"`
}

func (c ProxyConfig) toCredentials() Credentials {
	username := ""
	password := ""
	var user *url.Userinfo
	if c.RandomUser {
		username = randomName()
		password, _ = randomString()
		user = url.UserPassword(username, password)
	}
	host := c.Host
	if c.RandomSubdomain {
		host = fmt.Sprintf("%s.%s", randomDomain(), host)
	}

	u := &url.URL{
		Scheme: c.Protocol,
		Host:   fmt.Sprintf("%s:%d", host, c.Port),
		User:   user,
	}
	return Credentials{
		Host:     host,
		Port:     c.Port,
		Protocol: c.Protocol,
		Username: username,
		Password: password,
		Uri:      u.String(),
	}
}

type FakeBrokerConfig struct {
	Proxy          ProxyConfig `cloud:"proxy"`
	BrokerUsername string      `cloud:"broker_username" cloud-default:"brokeruser"`
	BrokerPassword string      `cloud:"broker_password" cloud-default:"password"`
}

type FakeProxyBroker struct {
	proxyConfig ProxyConfig
}

func (b *FakeProxyBroker) GetInstance(ctx context.Context, instanceID string) (domain.GetInstanceDetailsSpec, error) {
	return domain.GetInstanceDetailsSpec{}, nil
}

func (b *FakeProxyBroker) GetBinding(ctx context.Context, instanceID, bindingID string) (domain.GetBindingSpec, error) {
	return domain.GetBindingSpec{}, nil
}

func (b *FakeProxyBroker) LastBindingOperation(ctx context.Context, instanceID, bindingID string, details domain.PollDetails) (domain.LastOperation, error) {
	return domain.LastOperation{}, nil
}

func NewFakeProxyBroker(proxyConfig ProxyConfig) *FakeProxyBroker {
	return &FakeProxyBroker{proxyConfig}
}

func (b *FakeProxyBroker) Services(context.Context) ([]domain.Service, error) {
	rootUUid, _ := uuid.FromString(ROOT_UUID)
	serviceUuid := uuid.NewV5(rootUUid, b.proxyConfig.Name+"-service")
	planUuid := uuid.NewV5(rootUUid, b.proxyConfig.Name+"-plan")

	metadata := &domain.ServiceMetadata{
		DocumentationUrl: b.proxyConfig.DocumentationURL,
		SupportUrl:       b.proxyConfig.SupportURL,
	}

	if b.proxyConfig.ImagePath != "" {
		data, err := os.ReadFile(b.proxyConfig.ImagePath)
		if err == nil {
			encoded := base64.RawStdEncoding.EncodeToString(data)
			metadata.ImageUrl = fmt.Sprintf("data:image/png;base64,%s", encoded)
		}
	}

	return []domain.Service{
		{
			ID:          serviceUuid.String(),
			Name:        b.proxyConfig.Name,
			Description: b.proxyConfig.Description,
			Bindable:    true,
			Tags:        []string{"proxy", "http-proxy", "https-proxy"},
			Metadata:    metadata,
			Plans: []domain.ServicePlan{
				{
					ID:          planUuid.String(),
					Name:        b.proxyConfig.Name,
					Description: b.proxyConfig.Description,
				},
			},
		},
	}, nil
}

func (b *FakeProxyBroker) Provision(context context.Context, instanceID string, details domain.ProvisionDetails, asyncAllowed bool) (domain.ProvisionedServiceSpec, error) {
	return domain.ProvisionedServiceSpec{}, nil
}

func (b *FakeProxyBroker) Deprovision(context context.Context, instanceID string, details domain.DeprovisionDetails, asyncAllowed bool) (domain.DeprovisionServiceSpec, error) {
	return domain.DeprovisionServiceSpec{}, nil
}

func (b *FakeProxyBroker) Bind(context context.Context, instanceID string, bindingID string, details domain.BindDetails, asyncAllowed bool) (domain.Binding, error) {

	return domain.Binding{
		Credentials: b.proxyConfig.toCredentials(),
	}, nil
}

func (b *FakeProxyBroker) Unbind(context context.Context, instanceID string, bindingID string, details domain.UnbindDetails, asyncAllowed bool) (domain.UnbindSpec, error) {
	return domain.UnbindSpec{}, nil
}

func (b *FakeProxyBroker) LastOperation(context context.Context, instanceID string, details domain.PollDetails) (domain.LastOperation, error) {
	return domain.LastOperation{}, nil
}

func (b *FakeProxyBroker) Update(context context.Context, instanceID string, details domain.UpdateDetails, asyncAllowed bool) (domain.UpdateServiceSpec, error) {
	return domain.UpdateServiceSpec{}, nil
}

func main() {
	flag.Parse()

	conf := &FakeBrokerConfig{}

	err := gautocloud.Inject(conf)
	if err != nil {
		panic(err)
	}
	if conf.Proxy.Name == "" || conf.Proxy.Host == "" {
		panic(fmt.Errorf("you must have configured proxy name and proxy host at least"))
	}

	serviceBroker := NewFakeProxyBroker(conf.Proxy)
	logger := lager.NewLogger("guard-broker")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.ERROR))
	credentials := brokerapi.BrokerCredentials{
		Username: conf.BrokerUsername,
		Password: conf.BrokerPassword,
	}
	brokerAPI := brokerapi.New(serviceBroker, logger, credentials)
	http.Handle("/", brokerAPI)
	port := "8080"
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}

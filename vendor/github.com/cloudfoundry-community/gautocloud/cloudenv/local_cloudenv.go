package cloudenv

import (
	"fmt"
	"os"

	"encoding/json"
	"io"
	"path/filepath"
	"reflect"

	"github.com/go-viper/encoding/hcl"
	"github.com/google/uuid"
	"github.com/spf13/viper"
)

const (
	LOCAL_ENV_KEY        = "CLOUD_FILE"
	LOCAL_CONFIG_ENV_KEY = "CONFIG_FILE"
	DEFAULT_CONFIG_PATH  = "config.yml"
	SERVICES_CONFIG_KEY  = "services"
)

type LocalCloudEnv struct {
	servicesLocal []ServiceLocal
	id            string
	appName       string
}
type ServiceLocal struct {
	Name        string
	Tags        []string
	Credentials map[string]interface{}
}

func NewLocalCloudEnv() CloudEnv {
	cloudEnv := &LocalCloudEnv{}
	cloudEnv.servicesLocal = make([]ServiceLocal, 0)
	return cloudEnv
}
func NewLocalCloudEnvFromReader(r io.Reader, configType string) CloudEnv {
	cloudEnv := &LocalCloudEnv{}
	codecRegistry := viper.NewCodecRegistry()
	err := codecRegistry.RegisterCodec("hcl", hcl.Codec{})
	if err != nil {
		panic(fmt.Errorf("fatal error on registering codec: %s", err))
	}
	v := viper.NewWithOptions(viper.WithCodecRegistry(codecRegistry))
	v.SetConfigType(configType)
	err = v.ReadConfig(r)
	if err != nil {
		panic(fmt.Errorf("fatal error on reading cloud file: %s", err))
	}
	cloudEnv.loadServices(v.Get(SERVICES_CONFIG_KEY))
	cloudEnv.loadAppName()
	return cloudEnv
}
func (c *LocalCloudEnv) Load() error {
	if c.hasCloudFile() {
		err := c.loadCloudFile()
		if err != nil {
			return err
		}
	}
	err := c.loadConfigFile()
	if err != nil {
		return err
	}

	c.loadAppName()
	return nil
}
func (c *LocalCloudEnv) loadConfigFile() error {
	confPath := c.configPath()
	_, err := os.Stat(confPath)
	if err != nil {
		c.servicesLocal = append(c.servicesLocal, ServiceLocal{
			"config",
			[]string{"config"},
			make(map[string]interface{}),
		})
		return nil
	}
	codecRegistry := viper.NewCodecRegistry()
	err = codecRegistry.RegisterCodec("hcl", hcl.Codec{})
	if err != nil {
		panic(fmt.Errorf("fatal error on registering codec: %s", err))
	}
	v := viper.NewWithOptions(viper.WithCodecRegistry(codecRegistry))
	v.SetConfigType(filepath.Ext(confPath)[1:])
	v.SetConfigFile(confPath)
	err = v.ReadInConfig()
	if err != nil {
		return fmt.Errorf("fatal error on reading config file: %s", err.Error())
	}
	var creds map[interface{}]interface{}
	err = v.Unmarshal(&creds)
	if err != nil {
		return fmt.Errorf("fatal error when unmarshaling config file: %s", err.Error())
	}
	finalCreds := c.convertMapInterface(creds).(map[string]interface{})
	c.servicesLocal = append(c.servicesLocal, ServiceLocal{
		"config",
		[]string{"config"},
		finalCreds,
	})
	return nil
}
func (c *LocalCloudEnv) loadCloudFile() error {
	codecRegistry := viper.NewCodecRegistry()
	err := codecRegistry.RegisterCodec("hcl", hcl.Codec{})
	if err != nil {
		panic(fmt.Errorf("fatal error on registering codec: %s", err))
	}
	v := viper.NewWithOptions(viper.WithCodecRegistry(codecRegistry))
	v.SetConfigType(filepath.Ext(os.Getenv(LOCAL_ENV_KEY))[1:])
	v.SetConfigFile(os.Getenv(LOCAL_ENV_KEY))
	err = v.ReadInConfig()
	if err != nil {
		return fmt.Errorf("fatal error on reading cloud file: %s", err.Error())
	}
	services := v.Get(SERVICES_CONFIG_KEY)
	if services != nil {
		c.loadServices(v.Get(SERVICES_CONFIG_KEY))
	} else {
		c.servicesLocal = make([]ServiceLocal, 0)
	}
	return nil
}

func (c *LocalCloudEnv) loadAppName() {
	c.appName = "<unknown>"
	codecRegistry := viper.NewCodecRegistry()
	err := codecRegistry.RegisterCodec("hcl", hcl.Codec{})
	if err != nil {
		panic(fmt.Errorf("fatal error on registering codec: %s", err))
	}
	v := viper.NewWithOptions(viper.WithCodecRegistry(codecRegistry))
	appName := v.Get("app_name")
	if appName != nil {
		c.appName = appName.(string)
	}
}
func (c LocalCloudEnv) Name() string {
	return "localcloud"
}
func (c LocalCloudEnv) GetServicesFromName(name string) []Service {
	services := make([]Service, 0)
	for _, serviceLocal := range c.servicesLocal {
		if match(name, serviceLocal.Name) {
			services = append(services, Service{
				Credentials: serviceLocal.Credentials,
			})
		}
	}
	return services
}
func (c LocalCloudEnv) GetServicesFromTags(tags []string) []Service {
	services := make([]Service, 0)
	for _, tag := range tags {
		services = append(services, c.getServicesWithTag(tag)...)
	}
	return services
}
func (c LocalCloudEnv) getServicesWithTag(tag string) []Service {
	services := make([]Service, 0)
	for _, serviceLocal := range c.servicesLocal {
		if c.serviceLocalHasTag(serviceLocal, tag) {
			services = append(services, Service{
				Credentials: serviceLocal.Credentials,
			})
		}
	}
	return services
}
func (c LocalCloudEnv) serviceLocalHasTag(serviceLocal ServiceLocal, tag string) bool {
	for _, tagLocal := range serviceLocal.Tags {
		if match(tag, tagLocal) {
			return true
		}
	}
	return false
}
func (c LocalCloudEnv) convertSliceOfMap(toConvert map[string]interface{}) map[string]interface{} {
	for key, data := range toConvert {
		typeData := reflect.TypeOf(data)
		if typeData != reflect.TypeOf(make([]map[string]interface{}, 0)) {
			continue
		}
		dataSlice := make(map[string]interface{})
		for _, dataExtract := range data.([]map[string]interface{}) {
			for key, value := range dataExtract {
				dataSlice[key] = value
			}
		}
		toConvert[key] = dataSlice
	}
	return toConvert
}
func (c LocalCloudEnv) convertMapInterface(toConvert interface{}) interface{} {
	typeData := reflect.TypeOf(toConvert)
	if typeData != reflect.TypeOf(make(map[interface{}]interface{})) && typeData != reflect.TypeOf(make([]interface{}, 0)) {
		return reflect.ValueOf(toConvert).Interface()
	}
	if typeData == reflect.TypeOf(make([]interface{}, 0)) {
		dataSlice := toConvert.([]interface{})
		for i, data := range dataSlice {
			dataSlice[i] = c.convertMapInterface(data)
		}
		return dataSlice
	}
	converted := make(map[string]interface{})
	for key, value := range toConvert.(map[interface{}]interface{}) {
		converted[key.(string)] = c.convertMapInterface(value)
	}

	return converted
}
func (c *LocalCloudEnv) loadServices(v interface{}) {
	var dataFinal []interface{}
	if reflect.TypeOf(v) == reflect.TypeOf(make([]map[string]interface{}, 0)) {
		dataFinal = make([]interface{}, 0)
		dataSlice := v.([]map[string]interface{})
		for _, data := range dataSlice {
			dataFinal = append(dataFinal, c.convertSliceOfMap(data))
		}
	} else {
		dataSlice := v.([]interface{})
		for i, data := range dataSlice {
			dataSlice[i] = c.convertMapInterface(data)
		}
		dataFinal = dataSlice
	}
	b, err := json.Marshal(dataFinal)
	if err != nil {
		panic(fmt.Errorf("fatal error during loading cloud file: %s", err))
	}
	var services []ServiceLocal
	err = json.Unmarshal(b, &services)
	if err != nil {
		panic(fmt.Errorf("fatal error during loading cloud file: %s", err))
	}
	c.servicesLocal = services
}

func (c LocalCloudEnv) configPath() string {
	confPath := DEFAULT_CONFIG_PATH
	if os.Getenv(LOCAL_CONFIG_ENV_KEY) != "" {
		confPath = os.Getenv(LOCAL_CONFIG_ENV_KEY)
	}
	return confPath
}

// nolint:unused
func (c LocalCloudEnv) hasConfigFile() bool {
	return os.Getenv(LOCAL_CONFIG_ENV_KEY) != ""
}
func (c LocalCloudEnv) hasCloudFile() bool {
	return os.Getenv(LOCAL_ENV_KEY) != ""
}
func (c LocalCloudEnv) IsInCloudEnv() bool {
	return true
}
func (c *LocalCloudEnv) GetAppInfo() AppInfo {
	id := c.id
	if id == "" {
		id = uuid.NewString()
		c.id = id
	}
	return AppInfo{
		Id:         c.id,
		Name:       c.appName,
		Port:       0,
		Properties: make(map[string]interface{}),
	}

}

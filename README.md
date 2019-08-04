# fake-proxy-broker [![Build Status](https://travis-ci.com/orange-cloudfoundry/fake-proxy-broker.svg?branch=master)](https://travis-ci.com/orange-cloudfoundry/fake-proxy-broker)

This is a fake service broker for creating proxy credentials which target a valid host (set in host) 
and optionally simulate fake user/password and subdomain.

## Installation on a cloud foundry

1. Get latest releases in [releases page](https://github.com/orange-cloudfoundry/fake-proxy-broker/releases) (only build is linux amd64)
2. Unzip if you choose zipped version
3. Create an user provided service to manage your configuration, 
you can use the [file included in the repo](service.json) and run 
`cf cups <my-proxy-broker>-config -p service.json`
3. `cf push mybroker -c ./fake-proxy-broker`
4. register your broker: `cf create-service-broker myproxybroker <broker username> <broker password> <broker url>`
5. enable service: `cf enable-service-access <proxy name>`

## Config file

Explanation of the config file given in [/service.json](/service.json):

```json
{
  "broker_username": "user",
  "broker_password": "password",
  "proxy": {
    "name": "myproxy",
    "description": "description of my proxy",
    "host": "my.proxy.host",
    "port": 3128,
    "protocol": "http",
    "random_subdomain": true,
    "random_user": true
  }
}
```

- `name` (**Required**): Name of your proxy, will be use in catalog and plans
- `description` (*Optional*): Description of your service and plan
- `host` (**Required**): Real host pointing to an existing proxy
- `port` (*Optional*, Default: `3128`): Port of the real host
- `protocol` (*Optional*, Default: `http`): Protocol used for proxying
- `random_subdomain` (*Optional*): If set to true it will add a generated subdomain in binding credentials on host. 
e.g.: configured host is `my.proxy.host` credentials will be `<generated>.my.proxy.host`
- `random_user` (*Optional*): If set to true it will add a generated user and password in binding credentials.

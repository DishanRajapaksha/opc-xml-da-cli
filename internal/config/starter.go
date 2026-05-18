package config

func StarterConfigYAML() []byte {
	return []byte(`# OPC XML-DA CLI starter config.
endpoint: http://localhost/OPC/DA
http_timeout: 30s
request_timeout: 90s

# Optional SOAP locale and client request handle.
# locale: en-US
# client_handle: opc-xml-da-cli

# Optional Basic authentication.
# username: user
# password: secret

# Optional named profiles.
# default_profile: site-a
# profiles:
#   site-a:
#     endpoint: http://192.168.1.50/OPC/DA
#     username: user
#     password: secret
`)
}

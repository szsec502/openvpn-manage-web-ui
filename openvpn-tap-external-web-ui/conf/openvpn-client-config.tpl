{{ .ExtraClientOptions }}
client
proto {{ .Proto }}
remote {{ .ServerAddress }} {{ .Port }}
resolv-retry infinite
nobind

remote-cert-tls server
tls-version-min 1.2
verify-x509-name {{ .PiVPNServer }} name
persist-tun
persist-key

cipher {{ .Cipher }}
auth {{ .Auth }}
auth-nocache
# tls-client

ca {{ .Ca }}
cert {{ .Cert }}
key {{ .Key }}
ta {{ .Ta }}

# comp-lzo

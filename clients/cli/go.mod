module github.com/veritasvpn/clients/cli

go 1.22

require (
	github.com/veritasvpn/lib/config v0.0.0
	github.com/veritasvpn/lib/crypto v0.0.0
)

require (
	github.com/golang-jwt/jwt/v5 v5.2.1 // indirect
)

replace (
	github.com/veritasvpn/lib/config => ../../lib/config
	github.com/veritasvpn/lib/crypto => ../../lib/crypto
)

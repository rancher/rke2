package auth

import (
	"strings"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/group"
	"k8s.io/apiserver/pkg/authentication/request/union"
	"k8s.io/apiserver/pkg/authentication/request/x509"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	"k8s.io/apiserver/plugin/pkg/authenticator/password/passwordfile"
	"k8s.io/apiserver/plugin/pkg/authenticator/request/basicauth"
)

func FromArgs(args []string) (authenticator.Request, error) {
	var authenticators []authenticator.Request
	basicFile := getArg("--basic-auth-file=", args)
	if basicFile != "" {
		basicAuthenticator, err := passwordfile.NewCSV(basicFile)
		if err != nil {
			return nil, err
		}
		authenticators = append(authenticators, basicauth.New(basicAuthenticator))
	}

	clientCA := getArg("--client-ca-file", args)
	if clientCA != "" {
		ca, err := dynamiccertificates.NewDynamicCAContentFromFile("client-ca", clientCA)
		if err != nil {
			return nil, err
		}
		authenticators = append(authenticators, x509.NewDynamic(ca.VerifyOptions, x509.CommonNameUserConversion))
	}

	auth := union.New(authenticators...)
	return group.NewAuthenticatedGroupAdder(auth), nil
}

func getArg(key string, args []string) string {
	for _, arg := range args {
		if !strings.HasPrefix(arg, key) {
			continue
		}
		return strings.SplitN(arg, "=", 2)[1]
	}
	return ""
}

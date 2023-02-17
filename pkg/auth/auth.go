package auth

import (
	"context"

	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/group"
	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/client-go/informers"
	"k8s.io/kubernetes/plugin/pkg/auth/authenticator/token/bootstrap"
)

// BootstrapTokenAuthenticator returns an authenticator to handle bootstrap tokens.
// This requires a secret lister, which will be created from the provided kubeconfig.
func BootstrapTokenAuthenticator(ctx context.Context, file string) (authenticator.Request, error) {
	k8s, err := util.GetClientSet(file)
	if err != nil {
		return nil, err
	}

	factory := informers.NewSharedInformerFactory(k8s, 0)
	lister := factory.Core().V1().Secrets().Lister().Secrets(metav1.NamespaceSystem)
	audiences := authenticator.Audiences{version.Program}
	tokenAuth := authenticator.WrapAudienceAgnosticToken(audiences, bootstrap.NewTokenAuthenticator(lister))
	auth := bearertoken.New(tokenAuth)

	go factory.Core().V1().Secrets().Informer().Run(ctx.Done())
	return group.NewAuthenticatedGroupAdder(auth), nil
}

package graph

import (
	"context"
	"net/http"
	"net/url"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/alcionai/clues"
	abstractions "github.com/microsoft/kiota-abstractions-go"
	kauth "github.com/microsoft/kiota-authentication-azure-go"

	"github.com/alcionai/corso/src/pkg/account"
)

func GetAuth(tenant, client, secret, assertion string) (*kauth.AzureIdentityAuthenticationProvider, error) {
	var auth *kauth.AzureIdentityAuthenticationProvider
	var errAuth error

	if assertion == "" {
		// Client Provider: Uses Secret for access to tenant-level data
		cred, err := azidentity.NewClientSecretCredential(tenant, client, secret, nil)
		if err != nil {
			return nil, clues.Wrap(err, "creating m365 client identity")
		}
		auth, errAuth = kauth.NewAzureIdentityAuthenticationProviderWithScopes(
			cred,
			[]string{"https://graph.microsoft.com/.default"})
		if errAuth != nil {
			return nil, clues.Wrap(errAuth, "creating azure authentication")
		}
	} else {
		// Client Provider: Uses previously obtained user's Access Token as an Assertion for On-Behalf-Of flow
		cred, err := azidentity.NewOnBehalfOfCredentialWithSecret(tenant, client, assertion, secret, nil)
		if err != nil {
			return nil, clues.Wrap(err, "creating m365 client identity for on-behalf-of flow")
		}
		auth, errAuth = kauth.NewAzureIdentityAuthenticationProviderWithScopes(
			cred,
			[]string{"https://graph.microsoft.com/.default"})
		if errAuth != nil {
			return nil, clues.Wrap(errAuth, "creating azure authentication for on-behalf-of flow")
		}
	}

	return auth, nil
}

// ---------------------------------------------------------------------------
// requester authorization
// ---------------------------------------------------------------------------

type authorizer interface {
	addAuthToHeaders(
		ctx context.Context,
		urlStr string,
		headers http.Header,
	) error
}

// consumed by kiota
type authenticateRequester interface {
	AuthenticateRequest(
		ctx context.Context,
		request *abstractions.RequestInformation,
		additionalAuthenticationContext map[string]any,
	) error
}

// ---------------------------------------------------------------------------
// Azure Authorizer
// ---------------------------------------------------------------------------

type azureAuth struct {
	auth authenticateRequester
}

func NewAzureAuth(creds account.M365Config) (*azureAuth, error) {
	auth, err := GetAuth(
		creds.AzureTenantID,
		creds.AzureClientID,
		creds.AzureClientSecret,
		creds.AzureOnBehalfOfAssertion)

	return &azureAuth{auth}, clues.Stack(err).OrNil()
}

func (aa azureAuth) addAuthToHeaders(
	ctx context.Context,
	urlStr string,
	headers http.Header,
) error {
	requestInfo := abstractions.NewRequestInformation()

	uri, err := url.Parse(urlStr)
	if err != nil {
		return clues.WrapWC(ctx, err, "parsing url").OrNil()
	}

	requestInfo.SetUri(*uri)

	err = aa.auth.AuthenticateRequest(ctx, requestInfo, nil)

	for _, k := range requestInfo.Headers.ListKeys() {
		for _, v := range requestInfo.Headers.Get(k) {
			headers.Add(k, v)
		}
	}

	return clues.WrapWC(ctx, err, "authorizing request").OrNil()
}

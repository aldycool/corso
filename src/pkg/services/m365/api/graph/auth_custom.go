package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/alcionai/clues"
)

type CustomOnBehalfOfCredential struct {
	tenantID           string
	clientID           string
	clientSecret       string
	refreshToken       string
	serviceID          string
	serviceSecret      string
	clientAccessToken  string
	clientTokenExpiry  time.Time
	serviceAccessToken string
	serviceTokenExpiry time.Time
}

func NewCustomOnBehalfOfCredential(tenantID, clientID, clientSecret, refreshToken, serviceID, serviceSecret string) *CustomOnBehalfOfCredential {
	return &CustomOnBehalfOfCredential{
		tenantID:      tenantID,
		clientID:      clientID,
		clientSecret:  clientSecret,
		serviceID:     serviceID,
		serviceSecret: serviceSecret,
		refreshToken:  refreshToken,
	}
}

func (c *CustomOnBehalfOfCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	isAccessTokenValid := true
	// Invalidate all tokens 5 minutes before they expires just to be safe
	if c.serviceAccessToken == "" || time.Now().After(c.serviceTokenExpiry.Add(-5*time.Minute)) {
		isAccessTokenValid = false
	}
	if c.clientAccessToken == "" || time.Now().After(c.clientTokenExpiry.Add(-5*time.Minute)) {
		isAccessTokenValid = false
	}

	if isAccessTokenValid {
		return azcore.AccessToken{
			Token:     c.serviceAccessToken,
			ExpiresOn: c.serviceTokenExpiry,
		}, nil
	}

	tokenURL := "https://login.microsoftonline.com/" + c.tenantID + "/oauth2/v2.0/token"

	clientData := url.Values{}
	clientData.Set("grant_type", "refresh_token")
	clientData.Set("client_id", c.clientID)
	clientData.Set("client_secret", c.clientSecret)
	clientData.Set("refresh_token", c.refreshToken)

	clientToken, clientErr := GetTokenImpl(tokenURL, clientData)
	if clientErr != nil {
		return azcore.AccessToken{}, clues.Wrap(clientErr, "failed to get client token")
	}
	c.clientAccessToken = clientToken.Token
	c.clientTokenExpiry = clientToken.ExpiresOn

	serviceData := url.Values{}
	serviceData.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	serviceData.Set("client_id", c.serviceID)
	serviceData.Set("client_secret", c.serviceSecret)
	serviceData.Set("scope", "https://graph.microsoft.com/.default")
	serviceData.Set("requested_token_use", "on_behalf_of")
	serviceData.Set("assertion", c.clientAccessToken)

	serviceToken, serviceErr := GetTokenImpl(tokenURL, serviceData)
	if serviceErr != nil {
		return azcore.AccessToken{}, clues.Wrap(serviceErr, "failed to token")
	}
	c.serviceAccessToken = serviceToken.Token
	c.serviceTokenExpiry = serviceToken.ExpiresOn

	return azcore.AccessToken{
		Token:     c.serviceAccessToken,
		ExpiresOn: c.serviceTokenExpiry,
	}, nil
}

func GetTokenImpl(tokenURL string, data url.Values) (azcore.AccessToken, error) {
	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return azcore.AccessToken{}, clues.Wrap(err, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return azcore.AccessToken{}, clues.Wrap(err, "failed to send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return azcore.AccessToken{}, fmt.Errorf("failed to refresh token, status code: %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return azcore.AccessToken{}, clues.Wrap(err, "failed to parse response")
	}

	accessToken, ok := response["access_token"].(string)
	if !ok {
		return azcore.AccessToken{}, fmt.Errorf("no access token found in response")
	}
	expiresIn, ok := response["expires_in"].(float64)
	if !ok {
		return azcore.AccessToken{}, fmt.Errorf("no expires_in field found in response")
	}
	expiresOn := time.Now().Add(time.Duration(expiresIn) * time.Second)

	return azcore.AccessToken{
		Token:     accessToken,
		ExpiresOn: expiresOn,
	}, nil
}

package config

import (
	"os"

	"github.com/alcionai/clues"
	"github.com/spf13/viper"

	"github.com/alcionai/corso/src/cli/flags"
	"github.com/alcionai/corso/src/internal/common/str"
	"github.com/alcionai/corso/src/pkg/account"
	"github.com/alcionai/corso/src/pkg/credentials"
)

// prerequisite: readRepoConfig must have been run prior to this to populate the global viper values.
func m365ConfigsFromViper(vpr *viper.Viper) (account.M365Config, error) {
	var m365 account.M365Config

	m365.AzureClientID = vpr.GetString(account.AzureClientID)
	m365.AzureClientSecret = vpr.GetString(account.AzureSecret)
	m365.AzureOnBehalfOfRefreshToken = vpr.GetString(account.AzureOnBehalfOfRefreshToken)
	m365.AzureOnBehalfOfServiceID = vpr.GetString(account.AzureOnBehalfOfServiceID)
	m365.AzureOnBehalfOfServiceSecret = vpr.GetString(account.AzureOnBehalfOfServiceSecret)
	m365.AzureTenantID = vpr.GetString(account.AzureTenantIDKey)

	return m365, nil
}

func m365Overrides(in map[string]string) map[string]string {
	return map[string]string{
		account.AzureTenantID:          in[account.AzureTenantID],
		account.AccountProviderTypeKey: in[account.AccountProviderTypeKey],
	}
}

// add m365 config key names that require path related validations
var m365PathKeys = []string{}

// configureAccount builds a complete account configuration from a mix of
// viper properties and manual overrides.
func configureAccount(
	vpr *viper.Viper,
	readConfigFromViper bool,
	matchFromConfig bool,
	overrides map[string]string,
) (account.Account, error) {
	var (
		m365Cfg account.M365Config
		m365    credentials.M365
		acct    account.Account
		err     error
	)

	if readConfigFromViper {
		m365Cfg, err = m365ConfigsFromViper(vpr)
		if err != nil {
			return acct, clues.Wrap(err, "reading m365 configs from corso config file")
		}

		if matchFromConfig {
			providerType := vpr.GetString(account.AccountProviderTypeKey)
			if providerType != account.ProviderM365.String() {
				return acct, clues.New("unsupported account provider: [" + providerType + "]")
			}

			if err := mustMatchConfig(vpr, m365Overrides(overrides), m365PathKeys); err != nil {
				return acct, clues.Wrap(err, "verifying m365 configs in corso config file")
			}
		}
	}

	// compose the m365 config and credentials
	m365 = GetM365(m365Cfg)
	if err := m365.Validate(); err != nil {
		return acct, clues.Wrap(err, "validating m365 credentials")
	}

	m365Cfg = account.M365Config{
		M365: m365,
		AzureTenantID: str.First(
			overrides[account.AzureTenantID],
			flags.AzureClientTenantFV,
			os.Getenv(account.AzureTenantID),
			m365Cfg.AzureTenantID),
	}

	// ensure required properties are present
	if err := requireProps(map[string]string{
		credentials.AzureClientID:     m365Cfg.M365.AzureClientID,
		credentials.AzureClientSecret: m365Cfg.M365.AzureClientSecret,
		account.AzureTenantID:         m365Cfg.AzureTenantID,
	}); err != nil {
		return acct, err
	}

	// build the account
	acct, err = account.NewAccount(account.ProviderM365, m365Cfg)
	if err != nil {
		return acct, clues.Wrap(err, "retrieving m365 account configuration")
	}

	return acct, nil
}

// M365 is a helper for aggregating m365 secrets and credentials.
func GetM365(m365Cfg account.M365Config) credentials.M365 {
	AzureClientID := str.First(
		flags.AzureClientIDFV,
		os.Getenv(credentials.AzureClientID),
		m365Cfg.AzureClientID)
	AzureClientSecret := str.First(
		flags.AzureClientSecretFV,
		os.Getenv(credentials.AzureClientSecret),
		m365Cfg.AzureClientSecret)
	AzureOnBehalfOfRefreshToken := str.First(
		flags.AzureOnBehalfOfRefreshTokenFV,
		os.Getenv(credentials.AzureOnBehalfOfRefreshToken),
		m365Cfg.AzureOnBehalfOfRefreshToken)
	AzureOnBehalfOfServiceID := str.First(
		flags.AzureOnBehalfOfServiceIDFV,
		os.Getenv(credentials.AzureOnBehalfOfServiceID),
		m365Cfg.AzureOnBehalfOfServiceID)
	AzureOnBehalfOfServiceSecret := str.First(
		flags.AzureOnBehalfOfServiceSecretFV,
		os.Getenv(credentials.AzureOnBehalfOfServiceSecret),
		m365Cfg.AzureOnBehalfOfServiceSecret)

	return credentials.M365{
		AzureClientID:                AzureClientID,
		AzureClientSecret:            AzureClientSecret,
		AzureOnBehalfOfRefreshToken:  AzureOnBehalfOfRefreshToken,
		AzureOnBehalfOfServiceID:     AzureOnBehalfOfServiceID,
		AzureOnBehalfOfServiceSecret: AzureOnBehalfOfServiceSecret,
	}
}

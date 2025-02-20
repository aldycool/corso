package flags

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	UserFN                         = "user"
	MailBoxFN                      = "mailbox"
	AzureClientTenantFN            = "azure-tenant-id"
	AzureClientIDFN                = "azure-client-id"
	AzureClientSecretFN            = "azure-client-secret"
	AzureOnBehalfOfRefreshTokenFN  = "azure-on-behalf-of-refresh-token"
	AzureOnBehalfOfServiceIDFN     = "azure-on-behalf-of-service-id"
	AzureOnBehalfOfServiceSecretFN = "azure-on-behalf-of-service-secret"
)

var (
	UserFV                         []string
	AzureClientTenantFV            string
	AzureClientIDFV                string
	AzureClientSecretFV            string
	AzureOnBehalfOfRefreshTokenFV  string
	AzureOnBehalfOfServiceIDFV     string
	AzureOnBehalfOfServiceSecretFV string
)

// AddUserFlag adds the --user flag.
func AddUserFlag(cmd *cobra.Command) {
	cmd.Flags().StringSliceVar(
		&UserFV,
		UserFN, nil,
		"Backup a specific user's data; accepts '"+Wildcard+"' to select all users.")
	cobra.CheckErr(cmd.MarkFlagRequired(UserFN))
}

// AddMailBoxFlag adds the --user and --mailbox flag.
func AddMailBoxFlag(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.StringSliceVar(
		&UserFV,
		UserFN, nil,
		"Backup a specific user's data; accepts '"+Wildcard+"' to select all users.")

	cobra.CheckErr(flags.MarkDeprecated(UserFN, fmt.Sprintf("use --%s instead", MailBoxFN)))

	flags.StringSliceVar(
		&UserFV,
		MailBoxFN, nil,
		"Backup a specific mailbox's data; accepts '"+Wildcard+"' to select all mailbox.")
}

// AddAzureCredsFlags adds M365 cred flags
func AddAzureCredsFlags(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.StringVar(&AzureClientTenantFV, AzureClientTenantFN, "", "Azure tenant ID")
	fs.StringVar(&AzureClientIDFV, AzureClientIDFN, "", "Azure app client ID")
	fs.StringVar(&AzureClientSecretFV, AzureClientSecretFN, "", "Azure app client secret")
	fs.StringVar(&AzureOnBehalfOfRefreshTokenFV, AzureOnBehalfOfRefreshTokenFN, "", "Azure On-Behalf-Of Refresh Token")
	fs.StringVar(&AzureOnBehalfOfServiceIDFV, AzureOnBehalfOfServiceIDFN, "", "Azure On-Behalf-Of Service ID")
	fs.StringVar(&AzureOnBehalfOfServiceSecretFV, AzureOnBehalfOfServiceSecretFN, "", "Azure On-Behalf-Of Service Secret")
}

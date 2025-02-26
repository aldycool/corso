package betasdk

import (
	"testing"

	"github.com/alcionai/clues"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/alcionai/corso/src/internal/tester"
	"github.com/alcionai/corso/src/internal/tester/tconfig"
	"github.com/alcionai/corso/src/pkg/account"
	"github.com/alcionai/corso/src/pkg/count"
	"github.com/alcionai/corso/src/pkg/services/m365/api/graph"
)

type BetaClientSuite struct {
	tester.Suite
	credentials account.M365Config
}

func TestBetaClientSuite(t *testing.T) {
	suite.Run(t, &BetaClientSuite{
		Suite: tester.NewIntegrationSuite(t, [][]string{tconfig.M365AcctCredEnvs}),
	})
}

func (suite *BetaClientSuite) SetupSuite() {
	t := suite.T()

	ctx, flush := tester.NewContext(t)
	defer flush()

	graph.InitializeConcurrencyLimiter(ctx, false, 4)

	a := tconfig.NewM365Account(t)
	m365, err := a.M365Config()
	require.NoError(t, err, clues.ToCore(err))

	suite.credentials = m365
}

func (suite *BetaClientSuite) TestCreateBetaClient() {
	t := suite.T()
	adpt, err := graph.CreateAdapter(
		suite.credentials.AzureTenantID,
		suite.credentials.AzureClientID,
		suite.credentials.AzureClientSecret,
		suite.credentials.AzureOnBehalfOfRefreshToken,
		suite.credentials.AzureOnBehalfOfServiceID,
		suite.credentials.AzureOnBehalfOfServiceSecret,
		count.New())

	require.NoError(t, err, clues.ToCore(err))

	client := NewBetaClient(adpt)
	assert.NotNil(t, client)
}

// TestBasicClientGetFunctionality. Tests that adapter is able
// to parse retrieved Site Page. Additional tests should
// be handled within the /internal/connector/sharepoint when
// additional features are added.
func (suite *BetaClientSuite) TestBasicClientGetFunctionality() {
	t := suite.T()

	ctx, flush := tester.NewContext(t)
	defer flush()

	adpt, err := graph.CreateAdapter(
		suite.credentials.AzureTenantID,
		suite.credentials.AzureClientID,
		suite.credentials.AzureClientSecret,
		suite.credentials.AzureOnBehalfOfRefreshToken,
		suite.credentials.AzureOnBehalfOfServiceID,
		suite.credentials.AzureOnBehalfOfServiceSecret,
		count.New())
	require.NoError(t, err, clues.ToCore(err))

	client := NewBetaClient(adpt)
	require.NotNil(t, client)

	siteID := tconfig.M365SiteID(t)

	// TODO(dadams39) document allowable calls in main
	collection, err := client.SitesById(siteID).Pages().Get(ctx, nil)
	// Ensures that the client is able to receive data from beta
	// Not Registered Error: content type application/json does not have a factory registered to be parsed
	require.NoError(t, err, clues.ToCore(err))

	for _, page := range collection.GetValue() {
		assert.NotNil(t, page, "betasdk call for page does not return value.")

		if page != nil {
			t.Logf("Page :%s ", *page.GetName())
			assert.NotNil(t, page.GetId())
		}
	}
}

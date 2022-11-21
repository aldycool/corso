package connector

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/alcionai/corso/src/internal/connector/exchange"
	"github.com/alcionai/corso/src/internal/connector/support"
	"github.com/alcionai/corso/src/internal/tester"
	"github.com/alcionai/corso/src/pkg/selectors"
)

// ---------------------------------------------------------------------------
// DataCollection tests
// ---------------------------------------------------------------------------

type ConnectorDataCollectionIntegrationSuite struct {
	suite.Suite
	connector *GraphConnector
	user      string
	site      string
}

func TestConnectorDataCollectionIntegrationSuite(t *testing.T) {
	if err := tester.RunOnAny(
		tester.CorsoCITests,
		tester.CorsoConnectorDataCollectionTests,
	); err != nil {
		t.Skip(err)
	}

	suite.Run(t, new(ConnectorDataCollectionIntegrationSuite))
}

func (suite *ConnectorDataCollectionIntegrationSuite) SetupSuite() {
	ctx, flush := tester.NewContext()
	defer flush()

	_, err := tester.GetRequiredEnvVars(tester.M365AcctCredEnvs...)
	require.NoError(suite.T(), err)
	suite.connector = loadConnector(ctx, suite.T(), AllResources)
	suite.user = tester.M365UserID(suite.T())
	suite.site = tester.M365SiteID(suite.T())
	tester.LogTimeOfTest(suite.T())
}

// TestExchangeDataCollection verifies interface between operation and
// GraphConnector remains stable to receive a non-zero amount of Collections
// for the Exchange Package. Enabled exchange applications:
// - mail
// - contacts
// - events
func (suite *ConnectorDataCollectionIntegrationSuite) TestExchangeDataCollection() {
	ctx, flush := tester.NewContext()
	defer flush()

	connector := loadConnector(ctx, suite.T(), Users)
	tests := []struct {
		name        string
		getSelector func(t *testing.T) selectors.Selector
	}{
		{
			name: suite.user + " Email",
			getSelector: func(t *testing.T) selectors.Selector {
				sel := selectors.NewExchangeBackup()
				sel.Include(sel.MailFolders([]string{suite.user}, []string{exchange.DefaultMailFolder}, selectors.PrefixMatch()))

				return sel.Selector
			},
		},
		{
			name: suite.user + " Contacts",
			getSelector: func(t *testing.T) selectors.Selector {
				sel := selectors.NewExchangeBackup()
				sel.Include(sel.ContactFolders(
					[]string{suite.user},
					[]string{exchange.DefaultContactFolder},
					selectors.PrefixMatch()))

				return sel.Selector
			},
		},
		// {
		// 	name: suite.user + " Events",
		// 	getSelector: func(t *testing.T) selectors.Selector {
		// 		sel := selectors.NewExchangeBackup()
		// 		sel.Include(sel.EventCalendars(
		// 			[]string{suite.user},
		// 			[]string{exchange.DefaultCalendar},
		// 			selectors.PrefixMatch(),
		// 		))

		// 		return sel.Selector
		// 	},
		// },
	}

	for _, test := range tests {
		suite.T().Run(test.name, func(t *testing.T) {
			collection, err := connector.ExchangeDataCollection(ctx, test.getSelector(t))
			require.NoError(t, err)
			assert.Equal(t, len(collection), 1)
			channel := collection[0].Items()
			for object := range channel {
				buf := &bytes.Buffer{}
				_, err := buf.ReadFrom(object.ToReader())
				assert.NoError(t, err, "received a buf.Read error")
			}
			status := connector.AwaitStatus()
			assert.NotZero(t, status.Successful)
			t.Log(status.String())
		})
	}
}

// TestInvalidUserForDataCollections ensures verification process for users
func (suite *ConnectorDataCollectionIntegrationSuite) TestInvalidUserForDataCollections() {
	ctx, flush := tester.NewContext()
	defer flush()

	invalidUser := "foo@example.com"
	connector := loadConnector(ctx, suite.T(), Users)
	tests := []struct {
		name        string
		getSelector func(t *testing.T) selectors.Selector
	}{
		{
			name: "invalid exchange backup user",
			getSelector: func(t *testing.T) selectors.Selector {
				sel := selectors.NewExchangeBackup()
				sel.Include(sel.MailFolders([]string{invalidUser}, selectors.Any()))
				return sel.Selector
			},
		},
		{
			name: "Invalid onedrive backup user",
			getSelector: func(t *testing.T) selectors.Selector {
				sel := selectors.NewOneDriveBackup()
				sel.Include(sel.Folders([]string{invalidUser}, selectors.Any()))
				return sel.Selector
			},
		},
	}

	for _, test := range tests {
		suite.T().Run(test.name, func(t *testing.T) {
			collections, err := connector.DataCollections(ctx, test.getSelector(t))
			assert.Error(t, err)
			assert.Empty(t, collections)
		})
	}
}

// TestSharePointDataCollection verifies interface between operation and
// GraphConnector remains stable to receive a non-zero amount of Collections
// for the SharePoint Package.
func (suite *ConnectorDataCollectionIntegrationSuite) TestSharePointDataCollection() {
	ctx, flush := tester.NewContext()
	defer flush()

	connector := loadConnector(ctx, suite.T(), Users)
	tests := []struct {
		name        string
		getSelector func(t *testing.T) selectors.Selector
	}{
		{
			name: "Items - TODO: actual sharepoint categories",
			getSelector: func(t *testing.T) selectors.Selector {
				sel := selectors.NewSharePointBackup()
				sel.Include(sel.Folders([]string{suite.site}, selectors.Any()))

				return sel.Selector
			},
		},
	}

	for _, test := range tests {
		suite.T().Run(test.name, func(t *testing.T) {
			_, err := connector.SharePointDataCollections(ctx, test.getSelector(t))
			require.NoError(t, err)

			// TODO: Implementation
			// assert.Equal(t, len(collection), 1)

			// channel := collection[0].Items()

			// for object := range channel {
			// 	buf := &bytes.Buffer{}
			// 	_, err := buf.ReadFrom(object.ToReader())
			// 	assert.NoError(t, err, "received a buf.Read error")
			// }

			// status := connector.AwaitStatus()
			// assert.NotZero(t, status.Successful)

			// t.Log(status.String())
		})
	}
}

// ---------------------------------------------------------------------------
// CreateExchangeCollection tests
// ---------------------------------------------------------------------------

type ConnectorCreateExchangeCollectionIntegrationSuite struct {
	suite.Suite
	connector *GraphConnector
	user      string
	site      string
}

func TestConnectorCreateExchangeCollectionIntegrationSuite(t *testing.T) {
	if err := tester.RunOnAny(
		tester.CorsoCITests,
		tester.CorsoConnectorCreateExchangeCollectionTests,
	); err != nil {
		t.Skip(err)
	}

	suite.Run(t, new(ConnectorCreateExchangeCollectionIntegrationSuite))
}

func (suite *ConnectorCreateExchangeCollectionIntegrationSuite) SetupSuite() {
	ctx, flush := tester.NewContext()
	defer flush()

	_, err := tester.GetRequiredEnvVars(tester.M365AcctCredEnvs...)
	require.NoError(suite.T(), err)
	suite.connector = loadConnector(ctx, suite.T(), Users)
	suite.user = tester.M365UserID(suite.T())
	suite.site = tester.M365SiteID(suite.T())
	tester.LogTimeOfTest(suite.T())
}

func (suite *ConnectorCreateExchangeCollectionIntegrationSuite) TestMailFetch() {
	ctx, flush := tester.NewContext()
	defer flush()

	var (
		t      = suite.T()
		userID = tester.M365UserID(t)
	)

	tests := []struct {
		name        string
		scope       selectors.ExchangeScope
		folderNames map[string]struct{}
	}{
		{
			name: "Folder Iterative Check Mail",
			scope: selectors.NewExchangeBackup().MailFolders(
				[]string{userID},
				[]string{exchange.DefaultMailFolder},
				selectors.PrefixMatch(),
			)[0],
			folderNames: map[string]struct{}{
				exchange.DefaultMailFolder: {},
			},
		},
	}

	gc := loadConnector(ctx, t, Users)

	for _, test := range tests {
		suite.T().Run(test.name, func(t *testing.T) {
			collections, err := gc.createExchangeCollections(ctx, test.scope)
			require.NoError(t, err)

			for _, c := range collections {
				require.NotEmpty(t, c.FullPath().Folder())
				folder := c.FullPath().Folder()

				if _, ok := test.folderNames[folder]; ok {
					delete(test.folderNames, folder)
				}
			}

			assert.Empty(t, test.folderNames)
		})
	}
}

// TestMailSerializationRegression verifies that all mail data stored in the
// test account can be successfully downloaded into bytes and restored into
// M365 mail objects
func (suite *ConnectorCreateExchangeCollectionIntegrationSuite) TestMailSerializationRegression() {
	ctx, flush := tester.NewContext()
	defer flush()

	t := suite.T()
	connector := loadConnector(ctx, t, Users)
	sel := selectors.NewExchangeBackup()
	sel.Include(sel.MailFolders([]string{suite.user}, []string{exchange.DefaultMailFolder}, selectors.PrefixMatch()))
	collection, err := connector.createExchangeCollections(ctx, sel.Scopes()[0])
	require.NoError(t, err)

	for _, edc := range collection {
		suite.T().Run(edc.FullPath().String(), func(t *testing.T) {
			streamChannel := edc.Items()
			// Verify that each message can be restored
			for stream := range streamChannel {
				buf := &bytes.Buffer{}
				read, err := buf.ReadFrom(stream.ToReader())
				assert.NoError(t, err)
				assert.NotZero(t, read)
				message, err := support.CreateMessageFromBytes(buf.Bytes())
				assert.NotNil(t, message)
				assert.NoError(t, err)
			}
		})
	}

	status := connector.AwaitStatus()
	suite.NotNil(status)
	suite.Equal(status.ObjectCount, status.Successful)
}

// TestContactSerializationRegression verifies ability to query contact items
// and to store contact within Collection. Downloaded contacts are run through
// a regression test to ensure that downloaded items can be uploaded.
func (suite *ConnectorCreateExchangeCollectionIntegrationSuite) TestContactSerializationRegression() {
	ctx, flush := tester.NewContext()
	defer flush()

	connector := loadConnector(ctx, suite.T(), Users)

	tests := []struct {
		name          string
		getCollection func(t *testing.T) []*exchange.Collection
	}{
		{
			name: "Default Contact Folder",
			getCollection: func(t *testing.T) []*exchange.Collection {
				scope := selectors.
					NewExchangeBackup().
					ContactFolders([]string{suite.user}, []string{exchange.DefaultContactFolder}, selectors.PrefixMatch())[0]
				collections, err := connector.createExchangeCollections(ctx, scope)
				require.NoError(t, err)

				return collections
			},
		},
	}

	for _, test := range tests {
		suite.T().Run(test.name, func(t *testing.T) {
			edcs := test.getCollection(t)
			require.Equal(t, len(edcs), 1)
			edc := edcs[0]
			assert.Equal(t, edc.FullPath().Folder(), exchange.DefaultContactFolder)
			streamChannel := edc.Items()
			count := 0
			for stream := range streamChannel {
				buf := &bytes.Buffer{}
				read, err := buf.ReadFrom(stream.ToReader())
				assert.NoError(t, err)
				assert.NotZero(t, read)
				contact, err := support.CreateContactFromBytes(buf.Bytes())
				assert.NotNil(t, contact)
				assert.NoError(t, err, "error on converting contact bytes: "+string(buf.Bytes()))
				count++
			}
			assert.NotZero(t, count)

			status := connector.AwaitStatus()
			suite.NotNil(status)
			suite.Equal(status.ObjectCount, status.Successful)
		})
	}
}

// TestEventsSerializationRegression ensures functionality of createCollections
// to be able to successfully query, download and restore event objects
func (suite *ConnectorCreateExchangeCollectionIntegrationSuite) TestEventsSerializationRegression() {
	ctx, flush := tester.NewContext()
	defer flush()

	connector := loadConnector(ctx, suite.T(), Users)

	tests := []struct {
		name, expected string
		getCollection  func(t *testing.T) []*exchange.Collection
	}{
		{
			name:     "Default Event Calendar",
			expected: exchange.DefaultCalendar,
			getCollection: func(t *testing.T) []*exchange.Collection {
				sel := selectors.NewExchangeBackup()
				sel.Include(sel.EventCalendars([]string{suite.user}, []string{exchange.DefaultCalendar}, selectors.PrefixMatch()))
				collections, err := connector.createExchangeCollections(ctx, sel.Scopes()[0])
				require.NoError(t, err)

				return collections
			},
		},
		{
			name:     "Birthday Calendar",
			expected: "Birthdays",
			getCollection: func(t *testing.T) []*exchange.Collection {
				sel := selectors.NewExchangeBackup()
				sel.Include(sel.EventCalendars([]string{suite.user}, []string{"Birthdays"}, selectors.PrefixMatch()))
				collections, err := connector.createExchangeCollections(ctx, sel.Scopes()[0])
				require.NoError(t, err)

				return collections
			},
		},
	}

	for _, test := range tests {
		suite.T().Run(test.name, func(t *testing.T) {
			collections := test.getCollection(t)
			require.Equal(t, len(collections), 1)
			edc := collections[0]
			assert.Equal(t, edc.FullPath().Folder(), test.expected)
			streamChannel := edc.Items()

			for stream := range streamChannel {
				buf := &bytes.Buffer{}
				read, err := buf.ReadFrom(stream.ToReader())
				assert.NoError(t, err)
				assert.NotZero(t, read)
				event, err := support.CreateEventFromBytes(buf.Bytes())
				assert.NotNil(t, event)
				assert.NoError(t, err, "experienced error parsing event bytes: "+string(buf.Bytes()))
			}

			status := connector.AwaitStatus()
			suite.NotNil(status)
			suite.Equal(status.ObjectCount, status.Successful)
		})
	}
}

// TestAccessOfInboxAllUsers verifies that GraphConnector can
// support `--users *` for backup operations. Selector.DiscreteScopes
// returns all of the users within one scope. Only users who have
// messages in their inbox will have a collection returned.
// The final test insures that more than a 75% of the user collections are
// returned. If an error was experienced, the test will fail overall
func (suite *ConnectorCreateExchangeCollectionIntegrationSuite) TestAccessOfInboxAllUsers() {
	ctx, flush := tester.NewContext()
	defer flush()

	t := suite.T()
	connector := loadConnector(ctx, t, Users)
	sel := selectors.NewExchangeBackup()
	sel.Include(sel.MailFolders(selectors.Any(), []string{exchange.DefaultMailFolder}, selectors.PrefixMatch()))
	scopes := sel.DiscreteScopes(connector.GetUsers())

	for _, scope := range scopes {
		users := scope.Get(selectors.ExchangeUser)
		standard := (len(users) / 4) * 3
		collections, err := connector.createExchangeCollections(ctx, scope)
		require.NoError(t, err)
		suite.Greater(len(collections), standard)
	}
}

// ---------------------------------------------------------------------------
// CreateSharePointCollection tests
// ---------------------------------------------------------------------------

type ConnectorCreateSharePointCollectionIntegrationSuite struct {
	suite.Suite
	connector *GraphConnector
	user      string
}

func TestConnectorCreateSharePointCollectionIntegrationSuite(t *testing.T) {
	if err := tester.RunOnAny(
		tester.CorsoCITests,
		tester.CorsoConnectorCreateSharePointCollectionTests,
	); err != nil {
		t.Skip(err)
	}

	suite.Run(t, new(ConnectorCreateSharePointCollectionIntegrationSuite))
}

func (suite *ConnectorCreateSharePointCollectionIntegrationSuite) SetupSuite() {
	ctx, flush := tester.NewContext()
	defer flush()

	_, err := tester.GetRequiredEnvVars(tester.M365AcctCredEnvs...)
	require.NoError(suite.T(), err)
	suite.connector = loadConnector(ctx, suite.T(), Sites)
	suite.user = tester.M365UserID(suite.T())
	tester.LogTimeOfTest(suite.T())
}

func (suite *ConnectorCreateSharePointCollectionIntegrationSuite) TestCreateSharePointCollection() {
	ctx, flush := tester.NewContext()
	defer flush()

	var (
		t      = suite.T()
		userID = tester.M365UserID(t)
	)

	gc := loadConnector(ctx, t, Sites)
	scope := selectors.NewSharePointBackup().Folders(
		[]string{userID},
		[]string{"foo"},
		selectors.PrefixMatch(),
	)[0]

	_, err := gc.createSharePointCollections(ctx, scope)
	require.NoError(t, err)
}
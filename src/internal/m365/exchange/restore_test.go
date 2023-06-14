package exchange

import (
	"context"
	"testing"

	"github.com/alcionai/clues"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/alcionai/corso/src/internal/common/ptr"
	exchMock "github.com/alcionai/corso/src/internal/m365/exchange/mock"
	"github.com/alcionai/corso/src/internal/tester"
	"github.com/alcionai/corso/src/pkg/account"
	"github.com/alcionai/corso/src/pkg/fault"
	"github.com/alcionai/corso/src/pkg/path"
	"github.com/alcionai/corso/src/pkg/services/m365/api"
)

type RestoreIntgSuite struct {
	tester.Suite
	credentials account.M365Config
	ac          api.Client
}

func TestRestoreIntgSuite(t *testing.T) {
	suite.Run(t, &RestoreIntgSuite{
		Suite: tester.NewIntegrationSuite(
			t,
			[][]string{tester.M365AcctCredEnvs}),
	})
}

func (suite *RestoreIntgSuite) SetupSuite() {
	t := suite.T()

	a := tester.NewM365Account(t)
	m365, err := a.M365Config()
	require.NoError(t, err, clues.ToCore(err))

	suite.credentials = m365
	suite.ac, err = api.NewClient(m365)
	require.NoError(t, err, clues.ToCore(err))
}

// TestRestoreContact ensures contact object can be created, placed into
// the Corso Folder. The function handles test clean-up.
func (suite *RestoreIntgSuite) TestRestoreContact() {
	t := suite.T()

	ctx, flush := tester.NewContext(t)
	defer flush()

	var (
		userID     = tester.M365UserID(t)
		folderName = tester.DefaultTestRestoreConfig("contact").Location
		handler    = newContactRestoreHandler(suite.ac)
	)

	aFolder, err := handler.ac.CreateContainer(ctx, userID, folderName, "")
	require.NoError(t, err, clues.ToCore(err))

	folderID := ptr.Val(aFolder.GetId())

	defer func() {
		// Remove the folder containing contact prior to exiting test
		err = suite.ac.Contacts().DeleteContainer(ctx, userID, folderID)
		assert.NoError(t, err, clues.ToCore(err))
	}()

	info, err := handler.restore(
		ctx,
		exchMock.ContactBytes("Corso TestContact"),
		userID, folderID,
		fault.New(true))
	assert.NoError(t, err, clues.ToCore(err))
	assert.NotNil(t, info, "contact item info")
}

// TestRestoreEvent verifies that event object is able to created
// and sent into the test account of the Corso user in the newly created Corso Calendar
func (suite *RestoreIntgSuite) TestRestoreEvent() {
	t := suite.T()

	ctx, flush := tester.NewContext(t)
	defer flush()

	var (
		userID  = tester.M365UserID(t)
		subject = tester.DefaultTestRestoreConfig("event").Location
		handler = newEventRestoreHandler(suite.ac)
	)

	calendar, err := handler.ac.CreateContainer(ctx, userID, subject, "")
	require.NoError(t, err, clues.ToCore(err))

	calendarID := ptr.Val(calendar.GetId())

	defer func() {
		// Removes calendar containing events created during the test
		err = suite.ac.Events().DeleteContainer(ctx, userID, calendarID)
		assert.NoError(t, err, clues.ToCore(err))
	}()

	tests := []struct {
		name  string
		bytes []byte
	}{
		{
			name:  "Test Event With Attendees",
			bytes: exchMock.EventWithAttendeesBytes(subject),
		},
		{
			name:  "Test recurrenceTimeZone: Empty",
			bytes: exchMock.EventWithRecurrenceBytes(subject, `""`),
		},
	}

	for _, test := range tests {
		suite.Run(test.name, func() {
			t := suite.T()

			ctx, flush := tester.NewContext(t)
			defer flush()

			info, err := handler.restore(
				ctx,
				test.bytes,
				userID, calendarID,
				fault.New(true))
			assert.NoError(t, err, clues.ToCore(err))
			assert.NotNil(t, info, "event item info")
		})
	}
}

// TestRestoreExchangeObject verifies path.Category usage for restored objects
func (suite *RestoreIntgSuite) TestRestoreExchangeObject() {
	t := suite.T()

	handlers := restoreHandlers(suite.ac)

	userID := tester.M365UserID(suite.T())

	tests := []struct {
		name        string
		bytes       []byte
		category    path.CategoryType
		destination func(*testing.T, context.Context) string
	}{
		{
			name:     "Test Mail",
			bytes:    exchMock.MessageBytes("Restore Exchange Object"),
			category: path.EmailCategory,
			destination: func(t *testing.T, ctx context.Context) string {
				folderName := tester.DefaultTestRestoreConfig("mailobj").Location
				folder, err := handlers[path.EmailCategory].
					CreateContainer(ctx, userID, folderName, "")
				require.NoError(t, err, clues.ToCore(err))

				return ptr.Val(folder.GetId())
			},
		},
		{
			name:     "Test Mail: One Direct Attachment",
			bytes:    exchMock.MessageWithDirectAttachment("Restore 1 Attachment"),
			category: path.EmailCategory,
			destination: func(t *testing.T, ctx context.Context) string {
				folderName := tester.DefaultTestRestoreConfig("mailwattch").Location
				folder, err := handlers[path.EmailCategory].
					CreateContainer(ctx, userID, folderName, "")
				require.NoError(t, err, clues.ToCore(err))

				return ptr.Val(folder.GetId())
			},
		},
		{
			name:     "Test Mail: Item Attachment_Event",
			bytes:    exchMock.MessageWithItemAttachmentEvent("Event Item Attachment"),
			category: path.EmailCategory,
			destination: func(t *testing.T, ctx context.Context) string {
				folderName := tester.DefaultTestRestoreConfig("eventwattch").Location
				folder, err := handlers[path.EmailCategory].
					CreateContainer(ctx, userID, folderName, "")
				require.NoError(t, err, clues.ToCore(err))

				return ptr.Val(folder.GetId())
			},
		},
		{
			name:     "Test Mail: Item Attachment_Mail",
			bytes:    exchMock.MessageWithItemAttachmentMail("Mail Item Attachment"),
			category: path.EmailCategory,
			destination: func(t *testing.T, ctx context.Context) string {
				folderName := tester.DefaultTestRestoreConfig("mailitemattch").Location
				folder, err := handlers[path.EmailCategory].
					CreateContainer(ctx, userID, folderName, "")
				require.NoError(t, err, clues.ToCore(err))

				return ptr.Val(folder.GetId())
			},
		},
		{
			name: "Test Mail: Hydrated Item Attachment Mail",
			bytes: exchMock.MessageWithNestedItemAttachmentMail(t,
				exchMock.MessageBytes("Basic Item Attachment"),
				"Mail Item Attachment",
			),
			category: path.EmailCategory,
			destination: func(t *testing.T, ctx context.Context) string {
				folderName := tester.DefaultTestRestoreConfig("mailbasicattch").Location
				folder, err := handlers[path.EmailCategory].
					CreateContainer(ctx, userID, folderName, "")
				require.NoError(t, err, clues.ToCore(err))

				return ptr.Val(folder.GetId())
			},
		},
		{
			name: "Test Mail: Hydrated Item Attachment Mail One Attach",
			bytes: exchMock.MessageWithNestedItemAttachmentMail(t,
				exchMock.MessageWithDirectAttachment("Item Attachment Included"),
				"Mail Item Attachment",
			),
			category: path.EmailCategory,
			destination: func(t *testing.T, ctx context.Context) string {
				folderName := tester.DefaultTestRestoreConfig("mailnestattch").Location
				folder, err := handlers[path.EmailCategory].
					CreateContainer(ctx, userID, folderName, "")
				require.NoError(t, err, clues.ToCore(err))

				return ptr.Val(folder.GetId())
			},
		},
		{
			name: "Test Mail: Item Attachment_Contact",
			bytes: exchMock.MessageWithNestedItemAttachmentContact(t,
				exchMock.ContactBytes("Victor"),
				"Contact Item Attachment",
			),
			category: path.EmailCategory,
			destination: func(t *testing.T, ctx context.Context) string {
				folderName := tester.DefaultTestRestoreConfig("mailcontactattch").Location
				folder, err := handlers[path.EmailCategory].
					CreateContainer(ctx, userID, folderName, "")
				require.NoError(t, err, clues.ToCore(err))

				return ptr.Val(folder.GetId())
			},
		},
		{ // Restore will upload the Message without uploading the attachment
			name:     "Test Mail: Item Attachment_NestedEvent",
			bytes:    exchMock.MessageWithNestedItemAttachmentEvent("Nested Item Attachment"),
			category: path.EmailCategory,
			destination: func(t *testing.T, ctx context.Context) string {
				folderName := tester.DefaultTestRestoreConfig("nestedattch").Location
				folder, err := handlers[path.EmailCategory].
					CreateContainer(ctx, userID, folderName, "")
				require.NoError(t, err, clues.ToCore(err))

				return ptr.Val(folder.GetId())
			},
		},
		{
			name:     "Test Mail: One Large Attachment",
			bytes:    exchMock.MessageWithLargeAttachment("Restore Large Attachment"),
			category: path.EmailCategory,
			destination: func(t *testing.T, ctx context.Context) string {
				folderName := tester.DefaultTestRestoreConfig("maillargeattch").Location
				folder, err := handlers[path.EmailCategory].
					CreateContainer(ctx, userID, folderName, "")
				require.NoError(t, err, clues.ToCore(err))

				return ptr.Val(folder.GetId())
			},
		},
		{
			name:     "Test Mail: Two Attachments",
			bytes:    exchMock.MessageWithTwoAttachments("Restore 2 Attachments"),
			category: path.EmailCategory,
			destination: func(t *testing.T, ctx context.Context) string {
				folderName := tester.DefaultTestRestoreConfig("mailtwoattch").Location
				folder, err := handlers[path.EmailCategory].
					CreateContainer(ctx, userID, folderName, "")
				require.NoError(t, err, clues.ToCore(err))

				return ptr.Val(folder.GetId())
			},
		},
		{
			name:     "Test Mail: Reference(OneDrive) Attachment",
			bytes:    exchMock.MessageWithOneDriveAttachment("Restore Reference(OneDrive) Attachment"),
			category: path.EmailCategory,
			destination: func(t *testing.T, ctx context.Context) string {
				folderName := tester.DefaultTestRestoreConfig("mailrefattch").Location
				folder, err := handlers[path.EmailCategory].
					CreateContainer(ctx, userID, folderName, "")
				require.NoError(t, err, clues.ToCore(err))

				return ptr.Val(folder.GetId())
			},
		},
		{
			name:     "Test Contact",
			bytes:    exchMock.ContactBytes("Test_Omega"),
			category: path.ContactsCategory,
			destination: func(t *testing.T, ctx context.Context) string {
				folderName := tester.DefaultTestRestoreConfig("contact").Location
				folder, err := handlers[path.ContactsCategory].
					CreateContainer(ctx, userID, folderName, "")
				require.NoError(t, err, clues.ToCore(err))

				return ptr.Val(folder.GetId())
			},
		},
		{
			name:     "Test Events",
			bytes:    exchMock.EventBytes("Restored Event Object"),
			category: path.EventsCategory,
			destination: func(t *testing.T, ctx context.Context) string {
				folderName := tester.DefaultTestRestoreConfig("event").Location
				calendar, err := handlers[path.EventsCategory].
					CreateContainer(ctx, userID, folderName, "")
				require.NoError(t, err, clues.ToCore(err))

				return ptr.Val(calendar.GetId())
			},
		},
		{
			name:     "Test Event with Attachment",
			bytes:    exchMock.EventWithAttachment("Restored Event Attachment"),
			category: path.EventsCategory,
			destination: func(t *testing.T, ctx context.Context) string {
				folderName := tester.DefaultTestRestoreConfig("eventobj").Location
				calendar, err := handlers[path.EventsCategory].
					CreateContainer(ctx, userID, folderName, "")
				require.NoError(t, err, clues.ToCore(err))

				return ptr.Val(calendar.GetId())
			},
		},
	}

	for _, test := range tests {
		suite.Run(test.name, func() {
			t := suite.T()

			ctx, flush := tester.NewContext(t)
			defer flush()

			destination := test.destination(t, ctx)
			info, err := handlers[test.category].restore(
				ctx,
				test.bytes,
				userID, destination,
				fault.New(true))
			assert.NoError(t, err, clues.ToCore(err))
			assert.NotNil(t, info, "item info was not populated")
		})
	}
}
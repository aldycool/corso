package exchange

import (
	"context"

	"github.com/microsoft/kiota-abstractions-go/serialization"

	"github.com/alcionai/corso/src/pkg/backup/details"
	"github.com/alcionai/corso/src/pkg/control"
	"github.com/alcionai/corso/src/pkg/count"
	"github.com/alcionai/corso/src/pkg/fault"
	"github.com/alcionai/corso/src/pkg/path"
	"github.com/alcionai/corso/src/pkg/services/m365/api"
	"github.com/alcionai/corso/src/pkg/services/m365/api/graph"
	"github.com/alcionai/corso/src/pkg/services/m365/api/pagers"
)

// ---------------------------------------------------------------------------
// backup
// ---------------------------------------------------------------------------

type backupHandler interface {
	itemEnumerator() addedAndRemovedItemGetter
	itemHandler() itemGetterSerializer
	folderGetter() containerGetter
	previewIncludeContainers() []string
	previewExcludeContainers() []string
	NewContainerCache(userID string) (string, graph.ContainerResolver)

	canSkipItemFailurer
}

type addedAndRemovedItemGetter interface {
	GetAddedAndRemovedItemIDs(
		ctx context.Context,
		user, containerID, oldDeltaToken string,
		config api.CallConfig,
	) (pagers.AddedAndRemoved, error)
}

type itemGetterSerializer interface {
	GetItem(
		ctx context.Context,
		user, itemID string,
		errs *fault.Bus,
	) (serialization.Parsable, *details.ExchangeInfo, error)
	Serialize(
		ctx context.Context,
		item serialization.Parsable,
		user, itemID string,
	) ([]byte, error)
}

func BackupHandlers(ac api.Client) map[path.CategoryType]backupHandler {
	return map[path.CategoryType]backupHandler{
		path.ContactsCategory: newContactBackupHandler(ac),
		path.EmailCategory:    newMailBackupHandler(ac),
		path.EventsCategory:   newEventBackupHandler(ac),
	}
}

type canSkipItemFailurer interface {
	CanSkipItemFailure(
		err error,
		resourceID string,
		opts control.Options,
	) (fault.SkipCause, bool)
}

// ---------------------------------------------------------------------------
// restore
// ---------------------------------------------------------------------------

type restoreHandler interface {
	itemRestorer
	containerAPI
	getItemsByCollisionKeyser
	NewContainerCache(userID string) graph.ContainerResolver
	ShouldSetContainerToDefaultRoot(
		restoreFolderPath string,
		collectionPath path.Path,
	) bool
	FormatRestoreDestination(
		destinationContainerName string,
		collectionFullPath path.Path,
	) *path.Builder
}

// runs the item restoration (ie: item creation) process
// for a single item, whose summary contents are held in
// the body property.
type itemRestorer interface {
	restore(
		ctx context.Context,
		body []byte,
		userID, destinationID string,
		collisionKeyToItemID map[string]string,
		collisionPolicy control.CollisionPolicy,
		errs *fault.Bus,
		ctr *count.Bus,
	) (*details.ExchangeInfo, error)
}

// produces structs that interface with the graph/cache_container
// CachedContainer interface.
type containerAPI interface {
	containerByNamer

	// POSTs the creation of a new container
	CreateContainer(
		ctx context.Context,
		userID, parentContainerID, containerName string,
	) (graph.Container, error)
	DefaultRootContainer() string
}

type containerByNamer interface {
	// searches for a container by name.
	GetContainerByName(
		ctx context.Context,
		userID, parentContainerID, containerName string,
	) (graph.Container, error)
}

// primary interface controller for all per-cateogry restoration behavior.
func RestoreHandlers(
	ac api.Client,
) map[path.CategoryType]restoreHandler {
	return map[path.CategoryType]restoreHandler{
		path.ContactsCategory: newContactRestoreHandler(ac),
		path.EmailCategory:    newMailRestoreHandler(ac),
		path.EventsCategory:   newEventRestoreHandler(ac),
	}
}

type getItemsByCollisionKeyser interface {
	// GetItemsInContainerByCollisionKey looks up all items currently in
	// the container, and returns them in a map[collisionKey]itemID.
	// The collision key is uniquely defined by each category of data.
	// Collision key checks are used during restore to handle the on-
	// collision restore configurations that cause the item restore to get
	// skipped, replaced, or copied.
	GetItemsInContainerByCollisionKey(
		ctx context.Context,
		userID, containerID string,
	) (map[string]string, error)
}

// ---------------------------------------------------------------------------
// other interfaces
// ---------------------------------------------------------------------------

type postItemer[T any] interface {
	PostItem(
		ctx context.Context,
		userID, containerID string,
		body T,
	) (T, error)
}

type deleteItemer interface {
	DeleteItem(
		ctx context.Context,
		userID, itemID string,
	) error
}
